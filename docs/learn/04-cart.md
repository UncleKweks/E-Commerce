# Stage 4: The cart

**Goal:** a logged-in customer can add products to a cart, change quantities, remove things and see a total. The cart stays honest when products change: if something sells out or is taken off sale while it sits in a cart, the cart says so.

**Files in this stage:**

| File | What it does |
| --- | --- |
| [internal/services/cart_service.go](../../internal/services/cart_service.go) | The rules: upserts, stock limits, checking each line against the catalog |
| [internal/services/cart_service_test.go](../../internal/services/cart_service_test.go) | Tests for line statuses and totals |
| [internal/handlers/cart_handler.go](../../internal/handlers/cart_handler.go) | Turns requests into service calls, and errors into status codes |
| [internal/dto/cart.go](../../internal/dto/cart.go) | What the cart endpoints accept and return |
| [internal/server/server.go](../../internal/server/server.go) | The `/cart` routes, all behind login |
| [web/index.html](../../web/index.html) | The playground now has a working cart |

| Method | URL | What it does |
| --- | --- | --- |
| `GET` | `/cart` | Your cart, checked against today's prices and stock |
| `POST` | `/cart/items` | Add a product (adds to the quantity if it's already there) |
| `PUT` | `/cart/items/{product_id}` | Set a product's quantity to an exact number |
| `DELETE` | `/cart/items/{product_id}` | Remove a product |
| `DELETE` | `/cart` | Empty the cart |

Every one of these replies with the **whole updated cart**, so the frontend never has to work out totals itself.

---

## 1. What a cart is, and what it isn't

A cart feels simple, but it's worth being exact about what it promises:

- **It's a wish list with quantities.** It remembers what you intend to buy.
- **It does NOT reserve stock.** Putting the last pair of headphones in your cart doesn't stop someone else buying them. Stock is only taken at checkout (Stage 5).
- **It does NOT lock in the price.** The cart always shows **today's** price. The price is only frozen when an order is placed, which is why order items copy the price (lesson 1) and cart items don't.

**Why not reserve stock in the cart?** Most carts are abandoned. If adding to a cart reserved stock, people (and bots) could make your best products look sold out just by filling carts and walking away. Shops that *do* reserve (concert tickets, flash sales) hold items only for a few minutes, with a countdown. A normal shop doesn't.

**Why is the cart in the database and not just the browser?** Because it belongs to the account: add something on your phone, and it's there on your laptop. The cost is that you must log in to use it. Many shops also allow **guest carts**, kept in the browser and merged into the account cart when you log in. That's a nice extra, but it's more code, so this API requires login.

---

## 2. Ownership: whose cart is it?

Look at the URLs again. **None of them contain a cart ID.** There's no `/carts/17/items`. The server works out whose cart it is from the access token:

```go
cart, err := h.carts.GetCart(c.Request.Context(), middleware.UserID(c))
```

`middleware.UserID(c)` comes from the token's signed `sub` claim, which nobody can change (lesson 2). Every cart query in the service goes through one helper:

```go
func inUsersCart(db *gorm.DB, userID uint) *gorm.DB {
    return db.Model(&models.CartItem{}).Where("cart_id = (SELECT id FROM carts WHERE user_id = ?)", userID)
}
```

### The bug this avoids: IDOR

Imagine the API had `DELETE /cart-items/551`, and the code just did `DELETE FROM cart_items WHERE id = 551`. Any logged-in user could change the number to 552, 553, 554... and empty **other people's** carts. They're logged in, so the 401 check passes; there's no role involved, so the 403 check passes. The code simply never asked "is item 551 *yours*?"

This is called **IDOR** (Insecure Direct Object Reference), and it's one of the most common serious bugs in real APIs. It's found in banks, hospitals and big tech companies every year. The rule:

> **Every query for a user's data must include "and it belongs to this user".** Better still, design URLs so there's no ID to tamper with in the first place, like `/cart`.

Stage 5 needs the same care: `GET /orders/42` must only return order 42 if it's *your* order.

---

## 3. Upserts: insert, or update if it's already there

When you press "Add to cart" on a mouse that's already in your cart, you want the quantity to go from 1 to 2, not a second "Mouse" line. The obvious code is:

```text
1. Look for a line for this product.
2. If there isn't one, insert it with quantity 1.
3. If there is one, update it to quantity + 1.
```

You know from lessons 2 and 3 what's coming: **two requests at once break it.** Double-click the button, and both requests do step 1 at the same moment, both see "no line", and both insert. One of them then crashes on the unique rule `UNIQUE (cart_id, product_id)`, and the customer sees an error for clicking twice.

The fix is to do it in **one statement** that the database runs atomically (all at once, with nothing in between):

```sql
INSERT INTO cart_items (cart_id, product_id, quantity, ...)
VALUES (7, 11, 1, ...)
ON CONFLICT (cart_id, product_id)
DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
RETURNING quantity;
```

In plain words: "insert this line, but **if one already exists for this cart and product**, add the new quantity to it instead. Then tell me the final quantity." `EXCLUDED` means "the row I tried to insert".

This is called an **upsert** (update + insert). It only works because of the unique rule from Stage 1: `ON CONFLICT` needs a unique constraint to know what counts as "already there". Database design decisions keep paying off later.

The cart itself is created the same way, on the first "add to cart":

```sql
INSERT INTO carts (user_id, ...) VALUES (42, ...)
ON CONFLICT (user_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
RETURNING id;
```

### We tested it

The test fired **10 "add 1 mouse" requests at the same instant**, for a user who had no cart yet. Result: all 10 succeeded, **1 cart, 1 line, quantity 10**. Without upserts you'd typically see errors, two carts attempted, or a quantity lower than 10.

---

## 4. The "lost update" problem

Why `quantity = cart_items.quantity + EXCLUDED.quantity` *in SQL*, and not "read the quantity in Go, add one, write it back"?

| Time | Request A | Request B |
| --- | --- | --- |
| 1 | Reads quantity: 1 | |
| 2 | | Reads quantity: 1 |
| 3 | Writes 1 + 1 = 2 | |
| 4 | | Writes 1 + 1 = 2 |

Two adds, but the quantity went from 1 to **2**, not 3. Request B's write silently overwrote A's. This is called a **lost update**.

When the addition happens inside the database (`quantity = quantity + 1`), the database makes the second request wait for the first and then adds to the *new* value. You get 3.

**Rule of thumb:** to change a number based on its current value (stock, balance, quantity, view counts), do the maths in the `UPDATE` itself. Never read it into your program, change it and write it back. Stage 5 uses this for taking stock at checkout.

---

## 5. Stock limits in the cart

When you add to the cart, the API checks you're not asking for more than exists:

- **Stock limit:** you can't have 5 headphones in your cart if there are 3.
- **Per-item cap:** at most 99 of any one product, even if there are 10,000. One customer shouldn't be able to empty the warehouse by mistake (a typo of 1000 instead of 10), and it keeps `price × quantity` far from overflowing.

The check happens **after** the upsert, inside a transaction:

```go
// upsert, which returns newQty...
if limit := purchaseLimit(product); newQty > limit {
    return &NotEnoughStockError{Max: limit, InCart: newQty - quantity}
}
```

Returning an error from inside `Transaction(...)` **rolls everything back**, so the too-big quantity is never saved. It's as if the request never happened.

### This check is a courtesy, not a guarantee

The cart check stops obvious mistakes, but it can't stop overselling. Stock changes after you add to your cart: someone else buys the last one five minutes later. That's fine, because **the real check happens at checkout**, with row locks, in Stage 5. Think of the cart check as a helpful shop assistant, and the checkout check as the till.

### Errors that carry information

Most errors so far were simple values like `ErrSKUTaken`. "Not enough stock" needs more: the customer wants to know **how many they can have**. So it's a custom error type with fields:

```go
type NotEnoughStockError struct {
    Max    int // the most this cart line may hold
    InCart int // how many are already in the cart
}
```

The handler pulls it out with `errors.As` (which finds an error of a given *type*, where `errors.Is` finds a specific *value*), then sends both the message and the numbers:

```json
{"error": {
  "code": "not_enough_stock",
  "message": "You can have at most 3 of this product, and 2 are already in your cart.",
  "details": {"max": "3", "in_cart": "2"}
}}
```

The message is for people; `details` is for code. A frontend could use `max` to cap its quantity picker, without having to parse an English sentence.

---

## 6. Carts go stale

A cart can sit for days. Meanwhile an admin changes the price, stock sells out, or a product is taken off sale. The cart has to handle all of that honestly.

So **every time the cart is read, each line is checked against the product as it is right now**, and gets a status:

| Status | When | What the customer sees |
| --- | --- | --- |
| `ok` | Everything's fine | Normal line, counted in the subtotal |
| `not_enough_stock` | Fewer left than in the cart | "Only 1 left. Please lower the quantity." |
| `out_of_stock` | Stock is 0 | "Out of stock. Please remove it or check back later." |
| `unavailable` | Hidden, deleted, or its category is hidden | "This product is no longer available. Please remove it." |

Lines with a problem aren't counted in `subtotal_cents`, and `can_checkout` becomes `false`. Stage 5 will refuse to check out a cart like that.

### Don't make deleted products vanish

When a product is soft-deleted, GORM normally pretends it doesn't exist. If the cart loaded products the normal way, a deleted product's line would come back with no product at all, and the customer would see a mystery line with no name, or it would silently disappear. Neither is good: "where did my item go?" is a support email.

So the cart loads products with `Unscoped()`, which includes deleted ones:

```go
Preload("Product", func(q *gorm.DB) *gorm.DB { return q.Unscoped() })
```

Now the line still shows "Temp Book: This product is no longer available", and the customer understands what happened.

### GET never changes anything

You might think: "if a product is unavailable, why not just delete the line when the cart is loaded?" Because **GET requests must be safe**: reading something must never change it. Browsers, proxies and prefetchers send GET requests on their own (to preload links, retry, check caches). If GET changed data, those automatic requests would quietly edit carts. The cart *reports* the problem; the customer decides.

The same rule is why `GET /cart` for a brand-new user doesn't create a cart row. It just returns an empty cart. The row is created on the first "add".

---

## 7. POST, PUT and DELETE: which one, and why

| Request | Meaning | Send it twice and... |
| --- | --- | --- |
| `POST /cart/items {"product_id": 11, "quantity": 1}` | **Add** one more | ...you have **2** (not the same result) |
| `PUT /cart/items/11 {"quantity": 3}` | **Set** to exactly 3 | ...still 3 (same result) |
| `DELETE /cart/items/11` | **Remove** it | ...still removed (same result) |

A request that gives the same result however many times you send it is called **idempotent**. `PUT` and `DELETE` are idempotent; this `POST` isn't.

**Why it matters:** networks fail. A phone sends "set quantity to 3", the connection drops before the reply arrives, and the app doesn't know whether it worked. With `PUT`, it can safely just send it again. With `POST /cart/items`, retrying might add another one.

That's why the quantity box in a cart page should use `PUT` (set to the number shown), and only the "Add to cart" button uses `POST`.

Notice that removing a product that isn't in the cart returns **200 with the cart**, not a 404. The customer wanted it gone, and it's gone. Idempotent means the *effect* is the same each time; the API chose to make the reply the same too.

**Stage 5 preview:** checkout is a `POST` that charges money. Retrying it must not create two orders. That's what **idempotency keys** solve, and you'll build them there.

---

## 8. Smaller details

**A cart row is only created when it's needed.** A user who never added anything has no `carts` row, and that's fine: `GET /cart` returns an empty cart. Creating rows when first needed (lazily) avoids millions of empty carts for users who never shop.

**`updated_at` on the cart means "last activity".** Every add bumps it. Real shops use this for "you left something in your cart" emails, and to delete carts nobody has touched in months. You'll see a background job like that in Stage 7.

**Quantity is a pointer again.** `AddCartItemRequest.Quantity` is `*int`, like in lesson 3: not sent means "add 1"; an explicit `0` is rejected with a 422. Same idea, new place.

**`ItemCount` counts units, not lines.** 2 shirts and 1 mug is 3 items. That's what the little number on a cart icon shows.

---

## 9. Go concepts used in this stage

**Custom error types.** `NotEnoughStockError` is a struct with an `Error() string` method, so it *is* an `error`. Use one whenever the caller needs details, not just "what kind of failure".

**`errors.As` vs `errors.Is`.** `errors.Is(err, ErrCartItemNotFound)` asks "is it this exact error?" `errors.As(err, &stock)` asks "is it this *type* of error? If so, put it in `stock` so I can read its fields." Both look through wrapped errors (`fmt.Errorf("...: %w", err)`).

**Pure functions are easy to test.** `buildCart` takes cart rows and returns the response, with no database. So the tests can build fake products (deleted, hidden, low stock) in a few lines and check every status, without starting Postgres.

**`min` and `max` are built in** (since Go 1.21): `max(0, min(p.Stock, MaxQuantityPerItem))`.

**Raw SQL when it's clearer.** GORM can do upserts, but the raw `INSERT ... ON CONFLICT ... RETURNING` is easier to read and shows exactly what runs. Using an ORM doesn't mean you can never write SQL. Placeholders (`?`) still keep it safe.

**Immediately-invoked functions in JavaScript.** In the playground, `(() => { ... })()` builds the quantity box and button together in place. That's the JavaScript cousin of a Go closure.

---

## 10. Try it yourself

### In the browser

Restart the server (Ctrl+C, then `make run`) and open **<http://localhost:8080/>**.

1. **Log in**, or register on the Account tab. The Cart tab will say "log in first" until you do.
2. **Add to cart:** click a product, set a quantity, press "Add to cart". The Cart tab shows the count. Watch `POST /cart/items` in the log, and notice the reply is the whole cart.
3. **Ask for too many:** open the Noise-Cancelling Headphones (only 3 in stock) and add 5. Read the 409 message, then open the log entry and find `details.max`.
4. **Add the same product twice** and check the cart: one line, with the quantities added together. That's the upsert.
5. **Change a quantity** with the Update button: that's a `PUT`. Notice it *sets* the number rather than adding.
6. **Make the cart go stale.** Become an admin (lesson 3), then in the Admin tab lower a product's stock below what's in your cart, and hide another one. Go back to the Cart tab: the lines show the problems, the subtotal leaves them out, and a warning appears.
7. **Two users, two carts.** Open a private window, register a second account, and add something different. Each window sees only its own cart.

### In the terminal

```powershell
$api = "http://localhost:8080/api/v1"
function Show-Error($err) { "Status: $([int]$err.Exception.Response.StatusCode)"; $err.ErrorDetails.Message }

$email = "cart$(Get-Random)@example.com"
$body = @{ email = $email; password = "supersecret1"; first_name = "Cara"; last_name = "Obi" } | ConvertTo-Json
$token = (Invoke-RestMethod -Method Post -Uri "$api/auth/register" -ContentType "application/json" -Body $body).data.access_token
$auth = @{ Authorization = "Bearer $token" }

# Find the headphones' ID
$hp = (Invoke-RestMethod "$api/products?q=headphones").data[0].id

# Add 2, then 2 more: the second is refused (only 3 in stock)
Invoke-RestMethod -Method Post -Uri "$api/cart/items" -Headers $auth -ContentType "application/json" -Body "{`"product_id`": $hp, `"quantity`": 2}" | Out-Null
try { Invoke-RestMethod -Method Post -Uri "$api/cart/items" -Headers $auth -ContentType "application/json" -Body "{`"product_id`": $hp, `"quantity`": 2}" } catch { Show-Error $_ }

# The cart still says 2: the failed add was rolled back
(Invoke-RestMethod "$api/cart" -Headers $auth).data.items | Format-Table name, quantity, available, status
```

### The race test

This fires 10 "add 1" requests at exactly the same moment. Run it with a **new** user, so the cart doesn't exist yet either:

```powershell
$email = "race$(Get-Random)@example.com"
$body = @{ email = $email; password = "supersecret1"; first_name = "Race"; last_name = "Test" } | ConvertTo-Json
$token = (Invoke-RestMethod -Method Post -Uri "$api/auth/register" -ContentType "application/json" -Body $body).data.access_token
$mouse = (Invoke-RestMethod "$api/products?q=mouse").data[0].id

Add-Type -AssemblyName System.Net.Http
$client = New-Object System.Net.Http.HttpClient
$client.DefaultRequestHeaders.Authorization = New-Object System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", $token)
$tasks = 1..10 | ForEach-Object {
    $client.PostAsync("$api/cart/items", (New-Object System.Net.Http.StringContent("{`"product_id`": $mouse}", [Text.Encoding]::UTF8, "application/json")))
}
[Threading.Tasks.Task]::WaitAll($tasks)
"Statuses: " + (($tasks | ForEach-Object { [int]$_.Result.StatusCode }) -join ", ")

(Invoke-RestMethod "$api/cart" -Headers @{ Authorization = "Bearer $token" }).data.items | Format-Table name, quantity
```

You should see ten `200`s and **one** line with quantity **10**.

### In the database

```powershell
docker exec -it ecommerce_postgres psql -U postgres -d ecommerce_shop
```

```sql
-- One cart per user, one line per product
SELECT c.user_id, ci.product_id, ci.quantity
FROM carts c JOIN cart_items ci ON ci.cart_id = c.id
ORDER BY c.id DESC LIMIT 5;

-- Try to break the rules by hand: both fail
INSERT INTO cart_items (cart_id, product_id, quantity) VALUES ((SELECT max(id) FROM carts), (SELECT min(id) FROM products), 0);
INSERT INTO carts (user_id) VALUES ((SELECT user_id FROM carts LIMIT 1));
\q
```

The first fails the `quantity > 0` check; the second fails "one cart per user". Even a bug in the Go code couldn't create those rows.

---

## Check yourself

1. Why doesn't adding something to your cart reserve stock? When does stock actually get taken?
2. The cart shows today's price, but an order keeps the price from checkout time. Why the difference?
3. What is IDOR? How do the cart URLs make it impossible here?
4. What goes wrong with "look for the line, then insert or update it" when someone double-clicks? How does `INSERT ... ON CONFLICT` fix it?
5. What does `ON CONFLICT (cart_id, product_id)` depend on in the database schema?
6. What's a "lost update"? Why is `quantity = quantity + 1` in SQL safe when read-add-write in Go isn't?
7. The cart says you can have 3 headphones. Why can checkout still fail to sell you 3?
8. Why is `NotEnoughStockError` a struct instead of a simple `errors.New(...)` value? How does the handler get its fields out?
9. Why does the cart load products with `Unscoped()`?
10. Why must `GET /cart` never delete unavailable lines, or create a cart row?
11. Which of `POST /cart/items`, `PUT /cart/items/{id}` and `DELETE /cart/items/{id}` are idempotent? Why does that matter on a bad mobile connection?
