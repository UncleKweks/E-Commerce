# Stage 3: Products and categories

**Goal:** customers can browse the shop (search, filter, sort, page through results) without logging in. Only admins can add, change or remove products and categories.

**Files in this stage:**

| File | What it does |
| --- | --- |
| [internal/server/server.go](../../internal/server/server.go) | Adds the public catalog routes and the `/admin` routes |
| [internal/handlers/catalog_handler.go](../../internal/handlers/catalog_handler.go) | Reads requests, calls the service, picks status codes |
| [internal/handlers/bind.go](../../internal/handlers/bind.go) | Reading JSON and query strings, validation messages, custom rules |
| [internal/services/catalog_service.go](../../internal/services/catalog_service.go) | The rules: filtering, pagination, partial updates, safe deletes |
| [internal/dto/catalog.go](../../internal/dto/catalog.go) | What the API accepts and returns for products and categories |
| [internal/response/response.go](../../internal/response/response.go) | New `Paged` reply with page information |
| [cmd/admin/main.go](../../cmd/admin/main.go) | A small command that turns a user into an admin |
| [db/seed.sql](../../db/seed.sql) | Sample categories and products for local testing |

---

## 1. REST: URLs are things, methods are actions

Most web APIs follow a style called **REST**. The idea is simple:

- The **URL names a thing** (a "resource"): `/products`, `/products/42`, `/categories`.
- The **method says what to do with it**: `GET` reads, `POST` creates, `PATCH` changes, `DELETE` removes.

So instead of URLs like `/getProduct?id=42` or `/deleteProduct/42`, you have:

| Method | URL | Meaning |
| --- | --- | --- |
| `GET` | `/products` | List products |
| `GET` | `/products/42` | Get product 42 |
| `POST` | `/admin/products` | Create a product |
| `PATCH` | `/admin/products/42` | Change some fields of product 42 |
| `DELETE` | `/admin/products/42` | Remove product 42 |

Anyone who has used one REST API can guess how yours works. That's the whole point.

**`PATCH` vs `PUT`:** `PUT` means "replace the whole thing with what I'm sending". `PATCH` means "change only the fields I'm sending". This API uses `PATCH`, so an admin can fix a typo in a name without resending the price, stock and everything else.

**Why `/admin/...` URLs?** The public and admin views are different. The public `/products` hides products that aren't for sale yet; `/admin/products` shows everything. Keeping admin routes under one prefix also makes them easy to protect in one place (next section).

### New status codes in this stage

| Code | Meaning | Example |
| --- | --- | --- |
| 400 Bad Request | I can't even read what you sent | `?page=abc`, broken JSON |
| 404 Not Found | That thing doesn't exist (for you) | `/products/99999` |
| 409 Conflict | Your request is fine, but it clashes with what's already there | SKU already used; deleting a category that still has products |
| 422 Unprocessable | I can read it, but some values are wrong | negative price, missing name |

---

## 2. Admin-only routes

### Protecting a whole group at once

In [server.go](../../internal/server/server.go):

```go
admin := api.Group("/admin")
admin.Use(middleware.AuthRequired(tokens), middleware.RequireRole("admin"))
```

Every route added to `admin` goes through two checks, in order:

1. **`AuthRequired`**: are you logged in? If not, **401**.
2. **`RequireRole("admin")`**: are you an admin? If not, **403**.

Because the checks sit on the *group*, not on each route, you can't add a new admin route next year and forget to protect it. Safe by default beats "remember to do it every time".

### Where does the first admin come from?

There's no API endpoint that makes someone an admin, on purpose. If there were, it would be the number one target for attackers. Instead, someone with access to the server runs a command:

```powershell
make admin email=ada@example.com
```

This runs [cmd/admin/main.go](../../cmd/admin/main.go), which updates the user's role directly in the database. Real companies do the same thing: the first admin is created by hand, and after that admins can be managed through internal tools.

### Why you must log in again after becoming an admin

Your role is written **inside your access token** when you log in (lesson 2). The server trusts the token without checking the database. So after `make admin`, your old token still says `"role": "customer"` until you log in again and get a new one.

The flip side matters more: if you *remove* someone's admin role, their current token keeps working as admin for up to 15 minutes. That's one more reason access tokens are short.

---

## 3. Validation: three kinds of "no"

When an admin creates a product, things can go wrong at three different levels. Each one gets a different status code, so the frontend knows what to show:

| Level | Example | Who checks | Status |
| --- | --- | --- | --- |
| **Can't read it** | `"price_cents": "ten"` or broken JSON | Go's JSON decoder | 400 |
| **Values are wrong** | price is `-5`, name is missing, SKU has spaces | `binding` rules in the DTO | 422 |
| **Clashes with the data** | SKU `TSHIRT-PLAIN` already exists | the database | 409 |

### Rules live on the DTO

From [dto/catalog.go](../../internal/dto/catalog.go):

```go
PriceCents *int64 `json:"price_cents" binding:"required,min=0,max=100000000"`
SKU        string `json:"sku" binding:"required,sku,max=64"`
```

Gin checks these automatically when it reads the request. All the problems come back at once, so the user can fix everything in one go:

```json
{"error": {"code": "validation_failed", "details": {
  "price_cents": "must be at least 0",
  "sku": "may only contain letters, numbers and dashes",
  "stock": "must be at least 0"
}}}
```

**Why a maximum price?** Without one, someone could send a price of 9 quintillion cents. It would fit in the database, but multiplying it by a quantity at checkout would overflow and wrap around to a negative number. Limits on every number and every text field protect everything further down.

### Two rules we wrote ourselves

[bind.go](../../internal/handlers/bind.go) adds:

- **`sku`**: only letters, numbers and dashes. SKUs end up in URLs, spreadsheets and warehouse labels, where spaces and symbols cause trouble.
- **`notblank`**: rejects `"   "`. The built-in `required` only checks that the text isn't empty, and three spaces aren't empty.

### Clean the data before saving it

The service also **normalizes** input:

- Names are trimmed: `"  Hoodie "` becomes `"Hoodie"`.
- SKUs are uppercased: `"tshirt-red-m"` becomes `"TSHIRT-RED-M"`.

Without the uppercasing, `abc-1` and `ABC-1` would count as two different SKUs, and the warehouse would have two labels for what people think is one product.

### Some rules can only be checked by the database

"Is this SKU already used?" can't be checked by looking at the request. And as you saw in lesson 2, checking first and inserting second is a race. So, like email at registration, the service just inserts and lets the **unique index** say no. The handler turns that into a 409.

---

## 4. PATCH and the "missing or zero?" problem

This is one of the most common bugs in real APIs, so it's worth understanding properly.

An admin sends this to say "we've sold out":

```json
PATCH /admin/products/42
{"stock": 0}
```

In Go, if a field is a plain `int`, then "the client sent 0" and "the client didn't send this field at all" look **exactly the same**: both are `0`. So the code can't tell "set stock to 0" from "leave stock alone".

**The fix: pointers.** In `UpdateProductRequest`, every field is a pointer:

```go
Stock *int `json:"stock"`
```

- Field not sent: `Stock` is `nil`, so leave it alone.
- `"stock": 0` sent: `Stock` points to `0`, so set it to 0.

### GORM has the same trap

GORM's `Updates` with a struct **skips zero values**:

```go
db.Model(&product).Updates(models.Product{Stock: 0}) // does NOT set stock to 0!
```

GORM thinks "0 is the default, the caller probably didn't mean it". So the service builds a **map** with only the fields that were sent, and a map has no such ambiguity:

```go
updates := map[string]any{}
if req.Stock != nil {
    updates["stock"] = *req.Stock
}
```

The walkthrough in section 11 tests exactly this.

---

## 5. Hiding vs deleting

There are two ways to make a product disappear from the shop, and they mean different things.

| | `is_active = false` (hide) | Soft delete |
| --- | --- | --- |
| Meaning | "Not for sale right now" | "This product is gone" |
| Can be undone through the API? | Yes, `PATCH {"is_active": true}` | No |
| Admin can still see it? | Yes, at `/admin/products` | No |
| Still in the database? | Yes | Yes (`deleted_at` is set) |
| Example | Seasonal item, product not launched yet | Discontinued forever |

### Hidden things are a 404, not a 403

If a customer requests a hidden product, they get **404 Not Found**, exactly as if it didn't exist. A 403 would say "this exists, but you can't see it". That leaks information: competitors could find your unreleased products just by trying IDs.

### Hiding a category hides its products

The public queries only show products where **the product is active and its category is active**:

```go
q.Where("is_active").
  Where("category_id IN (SELECT id FROM categories WHERE is_active AND deleted_at IS NULL)")
```

The seed data has a hidden "Coming Soon" category with a Smart Watch in it. The watch itself is active, but customers can't see it.

### Deleting a category that still has products

The `products.category_id` foreign key says `ON DELETE RESTRICT` (lesson 1), so the database refuses to *really* delete a category that has products. But we use soft deletes, which are actually `UPDATE`s, and the foreign key doesn't notice them. So the service checks itself: if the category still has products, it says **409** "move or delete them first". Otherwise you'd have products pointing at a category nobody can see.

### Why a deleted SKU can be reused

The unique index on SKU (lesson 1) is a **partial** index: `WHERE deleted_at IS NULL`. Deleted products don't count, so you can delete a product and create a new one with the same SKU.

---

## 6. Pagination: never send everything

A real shop might have 50,000 products. Sending all of them in one response would be slow for the database, slow over the network and slow for the phone showing them. So lists come in **pages**:

```text
GET /products?page=2&page_size=20
```

The reply includes `meta`, so the app can show "page 2 of 3" and know when to stop:

```json
{
  "data": [ ...20 products... ],
  "meta": {"page": 2, "page_size": 20, "total": 57, "total_pages": 3}
}
```

Under the hood, this becomes SQL:

```sql
SELECT ... ORDER BY created_at DESC, id DESC LIMIT 20 OFFSET 20
```

- `LIMIT 20`: give me at most 20 rows.
- `OFFSET 20`: skip the first 20 (that was page 1).

### Always cap the page size

`page_size` must be 100 or less. Without a cap, anyone can send `page_size=10000000` and make your database do a huge amount of work on every request. It's one of the easiest ways to slow an API down.

### Sort with a tiebreaker, or pages break

Say five products all cost 9.99 and you sort by price. The database is allowed to return those five in **any order**, and that order can change between two requests. So one of them could show up on page 1 *and* page 2, and another on neither.

The fix: always finish the sort with something unique. Every sort here ends with `id`:

```go
"price_asc": "price_cents ASC, id ASC",
```

Now there are never ties, so the order is always the same. A unit test checks that every sort option has this tiebreaker.

### Offset gets slow on very deep pages

To answer `OFFSET 100000`, the database still has to walk past 100,000 rows and throw them away. For a shop where people rarely go past page 10, that's fine. Huge feeds (like a social media timeline) use **cursor pagination** instead: "give me 20 items after product #8812". That's always fast, but you can't jump straight to page 37. Offset is the right choice here; just know the other one exists. The API caps `page` at 10,000 for the same reason.

---

## 7. Filtering, search and SQL injection

The product list takes filters in the **query string** (the part after `?`):

```text
GET /products?category_id=3&q=shirt&min_price=1000&max_price=5000&in_stock=true&sort=price_asc
```

They're read into `ProductListQuery` using `form:"..."` tags (instead of `json:"..."`) and validated just like a JSON body.

### SQL injection: never glue user input into SQL

This is one of the oldest and most damaging attacks on the web. Imagine building the search like this:

```go
// NEVER do this
db.Raw("SELECT * FROM products WHERE name ILIKE '%" + search + "%'")
```

Now someone searches for `'; DROP TABLE products; --` and your SQL becomes two commands, the second of which deletes your products table. The database can't tell your SQL from theirs.

**The fix is placeholders (`?`)**:

```go
q.Where("name ILIKE ?", "%"+escapeLike(search)+"%")
```

The SQL and the value travel to the database **separately**. The value is only ever treated as data, never as SQL, no matter what's in it. Every query in this project uses placeholders.

### Sort can't use placeholders, so it uses a whitelist

Placeholders work for *values*, but not for parts of the SQL itself like `ORDER BY price_cents`. So the client never sends SQL. It sends a word, and the code looks it up in a fixed list:

```go
var productSorts = map[string]string{
    "newest":     "created_at DESC, id DESC",
    "price_asc":  "price_cents ASC, id ASC",
    ...
}
```

`?sort=price_asc` becomes the fixed string we wrote. Anything not in the list is rejected with a 422 before it gets anywhere near the database. **Whitelist** (only allow known-good values) beats **blacklist** (try to block known-bad values), because you can never think of every bad value.

### Wildcards in search

In SQL `LIKE`, `%` means "anything" and `_` means "any one character". So a search for `100%` would match everything that starts with `100`. `escapeLike` puts a `\` in front of these, so they mean themselves. It's not a security hole, just wrong results, but it's the kind of detail that separates a careful API from a sloppy one.

### Search performance

`ILIKE '%shirt%'` can't use a normal index (an index can't help with "contains", only "starts with"), so Postgres reads every product. For a few thousand products that takes milliseconds. For millions, you'd add Postgres's `pg_trgm` extension or a search engine. Don't add that complexity until you need it.

---

## 8. Two queries per list, and the N+1 trap

Listing products runs **three** queries:

```sql
SELECT count(*) FROM products WHERE ...;                  -- 1. how many match? (for total)
SELECT * FROM products WHERE ... LIMIT 20 OFFSET 0;       -- 2. this page
SELECT * FROM categories WHERE id IN (2, 3, 4);           -- 3. their categories
```

Query 3 comes from `Preload("Category")`, and it's worth understanding why it's written this way.

Each product in the reply includes its category name. The naive way is: fetch 20 products, then for each one, fetch its category. That's 1 + 20 = **21 queries**. With 100 per page, 101 queries. This is called the **N+1 problem**, and it's the most common reason ORMs get blamed for being slow.

`Preload` collects all the category IDs from the page and fetches them in **one** query with `IN (...)`. Always 3 queries, however big the page.

**How to spot N+1 in the wild:** a page that gets slower the more items it shows, and logs full of nearly identical queries.

---

## 9. A first taste of locking

Picture two admins at the same moment:

| Time | Admin A: delete "Toys" | Admin B: add a Yo-yo to "Toys" |
| --- | --- | --- |
| 1 | Count products in Toys: 0 | |
| 2 | | Check Toys exists: yes |
| 3 | Delete Toys | |
| 4 | | Insert Yo-yo into Toys |

Now there's a Yo-yo in a deleted category. Both checks were true when they ran, and the result is still wrong. It's the same race from lesson 2 in a new place.

The fix is a **row lock**. Both operations run inside a transaction and lock the category's row first:

```go
cat, err := lockCategory(tx, id, "UPDATE") // delete: SELECT ... FOR UPDATE
cat, err := lockCategory(tx, id, "SHARE")  // add product: SELECT ... FOR SHARE
```

- **`FOR SHARE`** means "I'm using this row; don't change it until I'm done". Many transactions can share-lock the same row, so several admins can add products to one category at once.
- **`FOR UPDATE`** means "I'm about to change this row; everyone else wait". It waits for share locks to finish, and blocks new ones.

So whoever gets the lock first finishes completely, and the other one waits, then sees the true state: either the category is gone ("does not exist"), or it has a product ("still has products").

Locks are released automatically when the transaction commits or rolls back. Stage 5 (checkout) is built around this idea, because there two customers will race for the last item in stock.

---

## 10. Seed data and other details

### Seed data is not a migration

[db/seed.sql](../../db/seed.sql) fills the database with sample products so you have something to browse. It's **not** a migration:

- **Migrations** change the *structure* (tables, columns, indexes) and run everywhere, including production.
- **Seed data** is example *content* for your laptop. It never goes to production.

It uses `ON CONFLICT ... DO NOTHING`, so running `make seed` twice doesn't create duplicates.

### Gaps in IDs are normal

You might see product IDs jump from 15 to 29. Postgres hands out IDs from a counter, and an ID is used up even when the insert is then skipped (`ON CONFLICT DO NOTHING`) or rolled back. Never treat IDs as "how many products exist" or expect them to have no gaps.

### Lists return `[]`, never `null`

In Go, an empty slice can encode to JSON as `null`. The response helpers always make a real slice, so an empty search returns `"data": []`. Frontend code can then always loop over the result without checking for `null` first.

---

## 11. Go concepts used in this stage

**Pointers to mean "optional".** `*int` can be `nil` (not sent) or point to a number (including 0). This is how Go code tells "missing" from "zero", here and in many JSON APIs.

**Maps for lookups.** `order, ok := productSorts[f.Sort]`: the `ok` is `false` if the key isn't there, so you can fall back to a default.

**Methods as handlers.** `api.GET("/products", catalog.ListProducts)` passes the method itself, already tied to `catalog`, like a function. No wrapper needed.

**Small shared helpers instead of copy-paste.** The public and admin list routes do almost the same thing, so both call `listProducts(c, includeInactive)` with a different `true`/`false`.

**Compile regexes once.** `skuPattern` is created once when the program starts, at package level, not on every request.

**Builders that return a new value.** `q = q.Where(...)` adds a condition and returns the updated query. The filters are added one at a time, only when the client sent them.

---

## 12. Try it yourself

Do these in order. Start the server in Terminal A (`make docker-up`, then `make run`) and run the steps in Terminal B. Every step uses fresh names, so you can repeat the whole walkthrough.

### Step 1: Load the sample data and browse

In Terminal B:

```powershell
make seed
$api = "http://localhost:8080/api/v1"
function Show-Error($err) { "Status: $([int]$err.Exception.Response.StatusCode)"; $err.ErrorDetails.Message }

(Invoke-RestMethod "$api/categories").data | Format-Table id, name
```

You see Books, Clothing and Electronics, but not "Coming Soon", because it's hidden.

### Step 2: Page through the products

```powershell
$res = Invoke-RestMethod "$api/products?page_size=4"
$res.data | Format-Table id, name, price_cents, stock
$res.meta
```

`meta` says how many there are in total and how many pages. Now ask for page 2 and see the next 4:

```powershell
(Invoke-RestMethod "$api/products?page_size=4&page=2").data | Format-Table id, name
```

### Step 3: Filter, search and sort

```powershell
# Cheapest first
(Invoke-RestMethod "$api/products?sort=price_asc").data | Format-Table name, price_cents

# Search (case doesn't matter)
(Invoke-RestMethod "$api/products?q=SHIRT").data | Format-Table name

# Between 20.00 and 50.00, in stock only
(Invoke-RestMethod "$api/products?min_price=2000&max_price=5000&in_stock=true").data | Format-Table name, price_cents, stock
```

The Wool Socks and The Pragmatic Programmer have stock 0, so `in_stock=true` leaves them out. The Smart Watch and Old Phone Charger never appear: one is in a hidden category, the other is hidden itself.

### Step 4: Send bad filters

```powershell
try { Invoke-RestMethod "$api/products?sort=cheapest&page_size=1000" } catch { Show-Error $_ }
try { Invoke-RestMethod "$api/products?min_price=5000&max_price=100" } catch { Show-Error $_ }
try { Invoke-RestMethod "$api/products?page=two" } catch { Show-Error $_ }
```

The first two are 422 with a message per field. The last is 400: `two` isn't a number at all.

### Step 5: Try the admin routes as a customer

```powershell
$email = "admin$(Get-Random)@example.com"
$body = @{ email = $email; password = "supersecret1"; first_name = "Ada"; last_name = "Obi" } | ConvertTo-Json
$token = (Invoke-RestMethod -Method Post -Uri "$api/auth/register" -ContentType "application/json" -Body $body).data.access_token

try { Invoke-RestMethod "$api/admin/products" } catch { Show-Error $_ }
try { Invoke-RestMethod "$api/admin/products" -Headers @{ Authorization = "Bearer $token" } } catch { Show-Error $_ }
```

No token: **401**. A customer's token: **403**. Now you've seen both, and the difference from lesson 2.

### Step 6: Become an admin

```powershell
make admin email=$email

# Your old token still says "customer". Log in again for a new one:
$login = @{ email = $email; password = "supersecret1" } | ConvertTo-Json
$token = (Invoke-RestMethod -Method Post -Uri "$api/auth/login" -ContentType "application/json" -Body $login).data.access_token
$auth = @{ Authorization = "Bearer $token" }

(Invoke-RestMethod "$api/admin/products?page_size=50" -Headers $auth).meta.total
```

The admin list counts **every** product, including the hidden ones the public list left out.

### Step 7: Create a category and a product

```powershell
$cat = Invoke-RestMethod -Method Post -Uri "$api/admin/categories" -Headers $auth -ContentType "application/json" `
  -Body (@{ name = "Toys $(Get-Random)"; description = "Fun things" } | ConvertTo-Json)
$catId = $cat.data.id

$sku = "yoyo-$(Get-Random)"
$prod = Invoke-RestMethod -Method Post -Uri "$api/admin/products" -Headers $auth -ContentType "application/json" `
  -Body (@{ category_id = $catId; name = "  Yo-yo  "; price_cents = 499; stock = 7; sku = $sku } | ConvertTo-Json)
$prodId = $prod.data.id
$prod.data
```

Look at the reply: the name was trimmed to `Yo-yo`, and the SKU was uppercased.

### Step 8: Break the rules on purpose

```powershell
# Everything wrong at once: 422 listing each problem
try { Invoke-RestMethod -Method Post -Uri "$api/admin/products" -Headers $auth -ContentType "application/json" `
  -Body (@{ category_id = $catId; name = "   "; price_cents = -1; stock = -3; sku = "has spaces!" } | ConvertTo-Json) } catch { Show-Error $_ }

# Same SKU in lowercase: 409, because SKUs are uppercased before saving
try { Invoke-RestMethod -Method Post -Uri "$api/admin/products" -Headers $auth -ContentType "application/json" `
  -Body (@{ category_id = $catId; name = "Copy"; price_cents = 100; sku = $sku.ToLower() } | ConvertTo-Json) } catch { Show-Error $_ }

# A category that doesn't exist: 422 on category_id
try { Invoke-RestMethod -Method Post -Uri "$api/admin/products" -Headers $auth -ContentType "application/json" `
  -Body (@{ category_id = 999999; name = "Ghost"; price_cents = 100; sku = "GHOST-1" } | ConvertTo-Json) } catch { Show-Error $_ }
```

### Step 9: The "missing or zero?" test

```powershell
# Sell out: set stock to exactly 0
$p = Invoke-RestMethod -Method Patch -Uri "$api/admin/products/$prodId" -Headers $auth -ContentType "application/json" -Body '{"stock": 0}'
$p.data | Format-List name, price_cents, stock
```

Stock is now 0 and **nothing else changed**: the name and price are as before. If the service used a struct instead of a map, stock would still be 7 (section 4).

### Step 10: Hide it, then delete

```powershell
# Hide it
Invoke-RestMethod -Method Patch -Uri "$api/admin/products/$prodId" -Headers $auth -ContentType "application/json" -Body '{"is_active": false}' | Out-Null

# Customers get 404; the admin still sees it
try { Invoke-RestMethod "$api/products/$prodId" } catch { Show-Error $_ }
(Invoke-RestMethod "$api/admin/products/$prodId" -Headers $auth).data.is_active

# The category still has a product, so it can't be deleted: 409
try { Invoke-RestMethod -Method Delete -Uri "$api/admin/categories/$catId" -Headers $auth } catch { Show-Error $_ }

# Delete the product, then the category: both work now
Invoke-RestMethod -Method Delete -Uri "$api/admin/products/$prodId" -Headers $auth
Invoke-RestMethod -Method Delete -Uri "$api/admin/categories/$catId" -Headers $auth
"deleted"
```

### Step 11: Look in the database

```powershell
docker exec -it ecommerce_postgres psql -U postgres -d ecommerce_shop
```

```sql
-- The deleted product is still here, with deleted_at set
SELECT id, name, sku, is_active, deleted_at FROM products ORDER BY id DESC LIMIT 3;

-- Ask Postgres how it runs the public product list
EXPLAIN SELECT * FROM products WHERE is_active AND category_id = 3 ORDER BY created_at DESC LIMIT 20;
\q
```

`EXPLAIN` shows Postgres's plan. With so few rows it will say `Seq Scan` (read every row), which is the fastest choice for a small table. On a big table you'd hope to see `Index Scan`. You'll use `EXPLAIN` whenever a query is slow.

---

## 13. The same thing, in a browser

Open **http://localhost:8080/** while the server runs. It's a small shop page ([web/index.html](../../web/index.html)) that calls the same API, with a **Behind the scenes** panel on the right showing every request and response. Things to try:

- **Shop tab:** search, filter and sort, and watch the URL change under the filters. Choose "500 (too many)" per page to see the API's 422.
- **Account tab:** register, log in with a wrong password, register the same email twice. Press "Refresh tokens", then "Replay the old refresh token" to watch reuse detection end your sessions.
- **Admin tab as a customer:** you'll see the 403. Run the `make admin` command it shows, press "Refresh tokens", and the Admin tab opens up.
- **Edit a product:** the box at the bottom of the form shows the exact request. Change only the stock and see that the `PATCH` sends only `stock`.
- **Break things:** a price of `-5` (422), a price of `abc` (400), an existing SKU (409). Each error appears next to the field it belongs to.
- **Look at the terminal:** every request in the panel has a request ID, and the same ID is on the matching log line in Terminal A.

Read the page's JavaScript too. It's plain JavaScript with comments, and shows the frontend side of things the lessons talk about: turning cents into dollars only for display, never refreshing tokens twice at once, and inserting product names as text so a malicious name can't run code (XSS).

---

## Check yourself

1. Why does this API use `PATCH /admin/products/42` instead of `POST /updateProduct?id=42`?
2. What's the difference between a 400, a 422 and a 409? Give one example of each from this stage.
3. Why is there no API endpoint for making someone an admin? How does the first admin get created?
4. You make someone an admin. Why do they need to log in again before admin routes work?
5. An admin sends `{"stock": 0}`. Why would a plain `int` field fail to tell this apart from not sending stock at all? What two things in this code fix it?
6. A customer requests a hidden product. Why 404 and not 403?
7. Why must every sort end with `id`? What goes wrong without it?
8. Why is `page_size` capped at 100?
9. Why can the search value use a `?` placeholder, but the sort can't? How is the sort kept safe instead?
10. What is the N+1 problem, and how does `Preload` avoid it?
11. Describe the race between "delete category" and "add product to category". How do `FOR UPDATE` and `FOR SHARE` prevent it?
