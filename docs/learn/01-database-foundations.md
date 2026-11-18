# Stage 1: Database foundations

**Goal:** a database that refuses bad data on its own, a repeatable way to create and change it, and money stored correctly.

**Files in this stage:**

- [db/migrations/](../../db/migrations/): the SQL that creates every table
- [internal/models/](../../internal/models/): Go structs that mirror those tables
- [internal/database/database.go](../../internal/database/database.go): the connection and its pool
- [Makefile](../../Makefile): `migrate-*` commands

---

## 1. What a relational database actually is

A database like PostgreSQL stores data in **tables**. A table is like a spreadsheet with strict rules:

- Each **column** has a name and a fixed type (`TEXT`, `BIGINT`, `BOOLEAN`, `TIMESTAMPTZ`...). Postgres refuses a value of the wrong type.
- Each **row** is one record: one user, one product, one order.
- Each table has a **primary key** (`id`), a value that uniquely identifies a row and never changes. `BIGSERIAL` means Postgres hands out 1, 2, 3... automatically.

Tables point at each other with **foreign keys**. `products.category_id` holds the `id` of a row in `categories`. `REFERENCES categories (id)` makes Postgres check that the category really exists. You can't create a product in category 999 if there is no category 999.

Here's how this project's tables connect:

```text
users ──< refresh_tokens
  │
  ├──── carts ──< cart_items >── products >── categories
  │                                 │
  └───< orders ──< order_items >────┘
                                    │
                         product_images
```

`──<` means "one to many": one user has many orders, and one order has many items. The user-to-cart line has no `<` because it's one to one: `carts.user_id` is `UNIQUE`.

### What happens on delete

Every foreign key has to answer one question: what happens to the child rows when the parent is deleted?

| Rule | Meaning | Used for |
| --- | --- | --- |
| `CASCADE` | Delete the children too | A cart's items, an order's items, a user's refresh tokens |
| `RESTRICT` | Refuse to delete the parent | A category that still has products; a user who has orders |

Picking the wrong rule is how real systems lose data. If `orders.user_id` were `CASCADE`, deleting one user would silently wipe their entire order history, including the financial records you're legally required to keep.

---

## 2. Constraints: the database as the last line of defence

Your Go code will validate input, so why also put rules in the database?

Because **the database is the only place every write passes through.** Code has bugs. Someone writes a script, an admin tool, or a second service, or two requests race each other. Constraints catch all of these.

The constraints in this project, and what each one prevents:

| Constraint | Prevents |
| --- | --- |
| `CHECK (stock >= 0)` | Selling stock you don't have, even if two checkouts race (Stage 5 relies on this) |
| `CHECK (price_cents >= 0)` | A negative price paying customers to take products |
| `CHECK (role IN ('customer', 'admin'))` | A typo like `'admn'` creating a user with undefined permissions |
| `CHECK (email = lower(email))` | `Bob@x.com` and `bob@x.com` becoming two accounts |
| `UNIQUE (cart_id, product_id)` | The same product appearing twice in one cart |
| `carts.user_id UNIQUE` | A user ending up with two carts |
| Unique index on `products (sku)` | Two products sharing a stock-keeping code |
| `NOT NULL` almost everywhere | Missing data that crashes code expecting a value |

**The rule:** validate in Go to give users friendly error messages, and constrain in the database to guarantee correctness. Do both, every time.

When a constraint fires, Postgres returns an error such as `SQLSTATE 23505` (a duplicate key). `TranslateError: true` in [database.go](../../internal/database/database.go) converts those into Go errors you can check:

```go
if errors.Is(err, gorm.ErrDuplicatedKey) {
    // tell the user "that email is already registered"
}
```

You'll use exactly this in Stage 2.

---

## 3. Money: never use floats

The tutorial's models stored prices as `float64` (a decimal number like `19.99`). **This is the most common e-commerce bug.**

### The problem in one sentence

Computers can't store most decimal amounts exactly, so they store something very slightly off, and money maths built on "very slightly off" gives wrong totals.

### Why can't computers store 0.10?

Think about writing 1/3 as a decimal: `0.3333333...`. The 3s go on forever, so at some point you have to stop and accept a number that's *almost* 1/3.

Computers have the same problem, just with different numbers. They store everything in **binary**, which means building every number out of halves, quarters, eighths, sixteenths and so on.

- `0.5` is one half, so it's stored exactly.
- `0.25` is one quarter, so it's stored exactly.
- `0.10` can't be made from halves, quarters and eighths, no matter how many you add. Like 1/3 in decimal, it goes on forever, so the computer stores the closest thing it can: about `0.1000000000000000055`.

That tiny error is invisible on its own. It shows up when you start adding.

### See it happen

A customer buys ten items that cost 10 cents each. The total should be exactly $1.00:

```go
package main

import "fmt"

func main() {
    var total float64
    for i := 0; i < 10; i++ {
        total += 0.10
    }
    fmt.Println(total)        // 0.9999999999999999, not 1
    fmt.Println(total == 1.0) // false
}
```

In a real store this means:

- A check like "has the customer paid the full total?" says **no** when they have.
- The total you show and the total you charge are a fraction of a cent apart. Across thousands of orders, your books stop balancing.
- Rounding in different places (cart, order, receipt) gives different answers, and customers notice.

### The fix: count cents, not dollars

Instead of storing `19.99` dollars, store `1999` **cents**. Whole numbers have no fractions to approximate, so the computer stores them exactly, every time.

```go
price := int64(1999)          // $19.99, stored as 1999 cents
total := price * 3            // 5997, exactly
fmt.Printf("$%d.%02d\n", total/100, total%100) // prints $59.97
```

Think of it like counting coins instead of guessing at an amount. Ten 10-cent coins is 100 cents. No approximation is possible.

The rule:

1. **Store and calculate in cents** (or the smallest unit of your currency: pence for GBP, kobo for NGN).
2. **Only turn it into "$19.99" when showing it to a person**, at the very last step.

That's why the model field is:

```go
PriceCents int64 // 1999 means $19.99
```

The name `PriceCents` (not `Price`) tells every future reader the unit, so nobody accidentally treats 1999 as $1,999. Stripe works the same way: in Stage 6 you'll send it `amount: 1999` to charge $19.99.

### A trap when you try this yourself

If you test with `fmt.Println(0.1 + 0.2)`, Go prints `0.3`, and it looks like there's no problem. That's because when you write the numbers directly in code, Go does the maths ahead of time with perfect precision. Real prices come from a database or a customer's request, so they're always stored in variables, and that's where the errors appear. Use variables when you experiment: `a, b := 0.1, 0.2` then `fmt.Println(a + b)` prints `0.30000000000000004`.

**Currency.** An amount means nothing without its currency: 1999 USD and 1999 JPY are very different amounts. Orders store `currency CHAR(3)` (an ISO code like `USD`) next to the total.

---

## 4. Snapshots: orders record what was true at checkout

`order_items` stores `product_name` and `unit_price_cents` even though the product table already has a name and price. That looks like duplicate data, so why?

Because prices change. If an order only linked to the product, raising a product's price next month would rewrite every past order, and your accounting, refunds and customer receipts would all show the wrong amount.

**Anything financial is copied at the moment it happens and never changes afterwards.**

---

## 5. Soft deletes and partial unique indexes

`users`, `categories` and `products` have a `deleted_at` column. GORM's `gorm.DeletedAt` type makes `db.Delete(&user)` set that timestamp instead of removing the row, and every normal query then quietly adds `WHERE deleted_at IS NULL`.

Why keep "deleted" rows?

- Past orders still point at the product. Hard-deleting it would break them, and `RESTRICT` blocks it anyway.
- You can undo a mistake.
- You keep an audit trail.

This creates a trap. A plain `UNIQUE (email)` would stop a deleted user's email from ever registering again. The fix is a **partial unique index**:

```sql
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;
```

Uniqueness is only enforced among rows that aren't deleted.

**Orders are deliberately NOT soft-deleted.** You never delete financial records. You change their status (`cancelled`, `refunded`) instead.

---

## 6. Indexes: making queries fast

Without an index, finding "orders for user 42" means Postgres reads **every row** in `orders`. That's fine at 100 rows and painfully slow at 10 million.

An index is a sorted lookup structure, like the index at the back of a book. This project indexes:

- **Every foreign key column** (`orders.user_id`, `cart_items.product_id`...). Postgres does **not** do this automatically, and joins and cascading deletes on unindexed foreign keys are a classic performance problem.
- `orders (user_id, created_at DESC)`: a **composite index** shaped exactly for the "my orders, newest first" page.

Indexes aren't free: each one slows down writes slightly and uses disk space. Add them for queries you actually run.

---

## 7. Migrations: version control for your database

Before this stage, the only description of the database was the Go structs. Tutorials often call `db.AutoMigrate(&User{})`, which creates tables from structs. That's fine for a demo but dangerous in production:

- It only **adds** things. It never removes a column or changes a type safely.
- You can't see or review what SQL it will run.
- It can't express partial indexes, `CHECK` constraints or `ON DELETE` rules properly.
- Every running server tries to alter the schema at startup.

**Migrations** are numbered SQL files that each make one change. Each one has a pair:

```text
000004_create_orders.up.sql     ← applies the change
000004_create_orders.down.sql   ← undoes it
```

`golang-migrate` records the current version in a `schema_migrations` table and applies whatever hasn't run yet, in order. Every developer, every test run and production all end up with exactly the same schema.

```bash
make migrate-up                      # apply everything pending
make migrate-version                 # which version is the database on?
make migrate-down                    # undo the most recent one
make migrate-create name=add_address # create the next pair of files
```

### The golden rules

1. **Never edit a migration that has been applied anywhere else.** Others' databases already ran the old version and won't run it again. Write a new migration instead.
2. **Every `up` needs a working `down`.** I tested this stage by going up, down all four, and up again.
3. **Commit migrations with the code that needs them**, in the same commit.
4. **The Go structs must match the SQL.** Nothing checks this automatically yet. In Stage 8 an integration test will.

### The SQL and Go types used

| SQL | Go | Why |
| --- | --- | --- |
| `BIGSERIAL` / `BIGINT` | `uint` / `int64` | 64-bit ids never run out; `INTEGER` caps at ~2.1 billion |
| `TEXT` | `string` | In Postgres, `TEXT` is as fast as `VARCHAR(n)` and has no arbitrary limit |
| `TIMESTAMPTZ` | `time.Time` | Stores an exact moment in time. Always use it over `TIMESTAMP`, which drops the timezone |
| nullable `TEXT` | `*string` | A Go `string` can't be "missing"; a pointer can be `nil` |
| `BOOLEAN` | `bool` | |

On times: everything is stored in UTC (`TimeZone=UTC`, `NowFunc` returns UTC). You only convert to a customer's local time when displaying it. Mixing timezones in storage causes bugs around daylight-saving changes that are miserable to track down.

---

## 8. Connection pooling

Opening a database connection is slow (a network handshake plus authentication) and each one costs Postgres memory. Go's `database/sql` (which GORM uses underneath) keeps a **pool**: a set of open connections that requests borrow and return.

```go
sqlDB.SetMaxOpenConns(25)                  // never more than 25 at once
sqlDB.SetMaxIdleConns(25)                  // keep them open between requests
sqlDB.SetConnMaxLifetime(30 * time.Minute) // replace old connections periodically
sqlDB.SetConnMaxIdleTime(5 * time.Minute)  // close ones unused for a while
```

With no limit, a traffic spike can open hundreds of connections. Postgres allows **100 by default**, shared by every app and admin tool, so it starts refusing connections and the whole site goes down. A pool limit makes extra requests wait briefly instead.

`PingContext` at startup forces a real connection. If the password is wrong or Docker isn't running, the app fails immediately with a clear error, instead of starting "fine" and failing on the first customer's request. This is called **failing fast**.

---

## 9. Seeing your database

The tables now exist. To browse them, install the **PostgreSQL** extension (by Microsoft) in VS Code, or use DBeaver or pgAdmin:

| Field | Value |
| --- | --- |
| Host | `localhost` |
| Port | `5432` |
| User | `postgres` |
| Password | `DB_PASSWORD` from your `.env` |
| Database | `ecommerce_shop` |

Or use the terminal:

```bash
docker exec -it ecommerce_postgres psql -U postgres -d ecommerce_shop
\dt              -- list tables
\d products      -- show a table's columns, indexes and constraints
\q               -- quit
```

Try `\d products` and match what you see against [000002_create_catalog.up.sql](../../db/migrations/000002_create_catalog.up.sql).

---

## Check yourself

1. Why is the `stock >= 0` check in the database and not only in Go?
2. A product's price goes from 1999 to 2499. What happens to an order placed yesterday, and which design choice guarantees that?
3. What would go wrong if `orders.user_id` used `ON DELETE CASCADE`?
4. Why does `users_email_key` have `WHERE deleted_at IS NULL`?
5. You need to add a `shipping_address` column to `orders`. Your teammate already ran migration 000004. What do you do?
6. Why is `Phone` a `*string` but `FirstName` a `string`?
7. With `a, b := 0.1, 0.2`, what does `a + b` print, and how does storing cents avoid the problem?
8. The site gets a traffic spike. What does `SetMaxOpenConns(25)` protect against?
