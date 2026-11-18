# Stage 2: Authentication

**Goal:** people can create an account, log in, stay logged in safely, and log out. Some routes only work for logged-in users.

**Files in this stage:**

| File | What it does |
| --- | --- |
| [cmd/api/main.go](../../cmd/api/main.go) | Starts the web server and shuts it down cleanly |
| [internal/server/server.go](../../internal/server/server.go) | Connects each URL to its handler |
| [internal/middleware/](../../internal/middleware/) | Code that runs on every request: logging, login checks, rate limits |
| [internal/handlers/auth_handler.go](../../internal/handlers/auth_handler.go) | Reads the request, calls the service, sends the response |
| [internal/services/auth_service.go](../../internal/services/auth_service.go) | The actual logic: register, login, refresh, logout |
| [internal/auth/](../../internal/auth/) | Password hashing and tokens, plus tests |
| [internal/dto/auth.go](../../internal/dto/auth.go) | The JSON shapes the API accepts and returns |
| [internal/response/response.go](../../internal/response/response.go) | One standard format for every reply |

---

## 1. What happens when a request arrives

When someone logs in, their app sends an **HTTP request** to your server. It passes through several layers, and each layer has one job:

```text
Request: POST /api/v1/auth/login  {"email": "...", "password": "..."}
   │
   ▼
Router            "Which code handles POST /api/v1/auth/login?"
   │
   ▼
Middleware        Runs for every request: give it an ID, log it, check the
   │              size limit, check the rate limit, check the login token...
   ▼
Handler           Reads the JSON, checks the fields, calls the service,
   │              turns the result into a status code and JSON reply
   ▼
Service           The business logic: find the user, check the password,
   │              create the tokens
   ▼
Database
```

**Why split it up like this?** Each layer can change without touching the others. The service doesn't know anything about HTTP, so later you could call the same `Login` logic from a test, a command-line tool or a different kind of API without changing it.

### HTTP in 60 seconds

A request has:

- a **method**, which says what kind of action it is: `GET` to read, `POST` to create or perform an action, `PUT`/`PATCH` to update, `DELETE` to remove
- a **path**, such as `/api/v1/users/me`
- **headers**, which are extra information, such as `Authorization: Bearer <token>`
- an optional **body**, usually JSON

A response has a **status code** and usually a JSON body. The status code is the most important part, because it tells the app what happened without reading anything else:

| Code | Meaning | When this API uses it |
| --- | --- | --- |
| 200 OK | It worked | Login, refresh, getting your profile |
| 201 Created | It worked and made something new | Register |
| 204 No Content | It worked, nothing to send back | Logout |
| 400 Bad Request | I can't even read this | The body isn't valid JSON |
| 401 Unauthorized | I don't know who you are | Wrong password, missing or expired token |
| 403 Forbidden | I know who you are, and you're not allowed | A customer trying an admin action (Stage 3) |
| 404 Not Found | That doesn't exist | Unknown URL |
| 409 Conflict | That clashes with something that exists | Email already registered |
| 413 Too Large | The body is too big | More than 1 MB of JSON |
| 422 Unprocessable | I can read it, but the values are wrong | Email isn't an email, password too short |
| 429 Too Many Requests | Slow down | Too many login attempts |
| 500 Internal Error | Our fault | A bug, or the database is down |

**Rule of thumb:** 4xx means the client did something wrong; 5xx means the server did.

---

## 2. Passwords: never store them

If your database stores real passwords, then anyone who gets a copy of it (a hacker, a leaked backup, a careless employee) gets every customer's password. And because people reuse passwords, they also get into those customers' email and bank accounts.

So we store a **hash** instead.

### What a hash is

A hash function turns any text into a scrambled fingerprint. It only works one way:

```text
"supersecret1"  ──hash──▶  "$2a$12$0/85G/1uKxLkhSSwBpPV9u..."
"$2a$12$0/85..."  ──▶  ✗ impossible to turn back into "supersecret1"
```

**How do we check a login, then?** We hash the password they just typed and see whether it matches the stored hash. We never need the original.

**Hashing is not encryption.** Encryption can be reversed by anyone with the key, so if the key leaks, every password leaks. A hash can't be reversed at all.

### Why bcrypt, and why slow is good

Attackers with a stolen database don't reverse hashes. They **guess**: they hash millions of common passwords and look for matches. A normal fast hash like SHA-256 lets a single graphics card try **billions of guesses per second**.

bcrypt is designed to be slow. With `bcryptCost = 12` (in [password.go](../../internal/auth/password.go)), one check takes a quarter to half a second on a typical laptop:

- A user logging in won't notice a fraction of a second.
- An attacker trying a billion guesses at 0.25 seconds each needs about **8 years** instead of 1 second.

Each +1 to the cost doubles the time, so you can raise it as computers get faster.

### Salt

bcrypt also adds a random **salt** to every password before hashing. So two users who both pick `password123` get completely different hashes. That means an attacker has to crack each account separately, and can't use pre-built tables of common password hashes. You can see this in the test: hashing the same password twice gives two different results.

**bcrypt's one quirk:** it ignores anything past 72 bytes. That's why passwords are limited to 72 characters (`max=72`).

---

## 3. Authentication vs authorization

These sound alike but are different questions:

- **Authentication**: *who are you?* Proved by logging in. If it fails, the answer is **401**.
- **Authorization**: *are you allowed to do this?* Checked using your role. If it fails, the answer is **403**.

[middleware/auth.go](../../internal/middleware/auth.go) has one of each: `AuthRequired` (who are you?) and `RequireRole("admin")` (are you allowed?). Stage 3 uses `RequireRole` so only admins can create products.

---

## 4. Tokens: staying logged in

HTTP has no memory. Every request is a stranger. So after you log in, every following request has to prove who you are again. Asking for your password on every click would be terrible, so instead the server gives you a **token** after login, and your app sends it with every request:

```text
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### What a JWT is

Our access token is a **JWT** (JSON Web Token, said "jot"). It's three pieces of text joined by dots:

```text
eyJhbGciOiJIUzI1NiIs...  .  eyJyb2xlIjoiY3VzdG9tZXIi...  .  Xk3f9...
       header                        payload                 signature
```

- **Header:** which signing method was used (`HS256`).
- **Payload:** the facts, for example `{"sub": "8", "role": "customer", "exp": 1763470000}`. That's the user ID, their role, and when it expires.
- **Signature:** a seal made from the header, the payload and **your secret key**.

**The key point:** the payload is only encoded, **not encrypted**. Anyone can read it. Paste a token into [jwt.io](https://jwt.io) and you'll see the contents. So **never put secrets in a JWT** (no passwords, no card numbers).

What the signature gives you is **tamper-proofing**. If someone changes `"role": "customer"` to `"role": "admin"`, the signature no longer matches, and the server rejects the token. Only someone with `JWT_SECRET` can make a valid signature. That's why:

- the secret must be long and random (the config refuses anything under 32 characters)
- it must never be committed to GitHub (it lives in `.env`, which is gitignored)
- if it ever leaks, change it immediately: every existing token stops working

### The "alg: none" attack

Some JWT libraries used to accept tokens whose header said `"alg": "none"`, meaning "no signature needed". An attacker could write any payload they liked, including `"role": "admin"`, and get in. [tokens.go](../../internal/auth/tokens.go) prevents this with `jwt.WithValidMethods`: only HS256 is accepted. The test `TestAccessTokenRejected` checks this attack actually fails.

### Why the server doesn't look up the token

When a request arrives with a JWT, the server only checks the signature and the expiry date. It doesn't need the database, so it's very fast. That's the main advantage of JWTs.

It's also their weakness: **you can't cancel a JWT before it expires.** If someone steals one, it keeps working until its `exp` time. That's why access tokens are **short-lived: 15 minutes.**

---

## 5. Why there are two tokens

A 15-minute login would be annoying if you had to type your password every 15 minutes. So login returns **two** tokens:

| | Access token | Refresh token |
| --- | --- | --- |
| What it is | A JWT | A random 43-character string |
| Lasts | 15 minutes | 7 days |
| Sent | With every request | Only to `/auth/refresh` |
| Checked by | Its signature (no database) | Looking it up in the database |
| Can be cancelled? | No, it just expires | Yes, by setting `revoked_at` |

When the access token expires, the app quietly sends the refresh token to `/auth/refresh` and gets a fresh pair. The user stays logged in for a week, but a stolen access token is only useful for 15 minutes. And because refresh tokens live in the database, **logout actually works**: it marks the refresh token revoked.

### Refresh tokens are hashed too

The database stores a SHA-256 hash of each refresh token, never the token itself, for the same reason as passwords: a leaked database shouldn't give anyone working logins.

Why SHA-256 here and bcrypt for passwords? Because people choose weak passwords that can be guessed, but a refresh token is 32 random bytes. There are more possible tokens than atoms in the universe, so guessing is pointless and a fast hash is safe.

### Rotation: every refresh token works once

Each time you use a refresh token, it's revoked and you get a brand-new one. Here's why that helps:

1. A thief steals Ada's refresh token and uses it. They get a new pair, and the stolen token is now revoked.
2. Later, Ada's app tries to refresh with the same (now revoked) token.
3. The server sees **a revoked token being used again**. It doesn't know who is the thief, but it knows the token was copied.
4. So it **revokes every session Ada has**. The thief's new tokens die too. Ada logs in again with her password, which the thief doesn't have.

This is called **reuse detection**, and it's in `Refresh` in [auth_service.go](../../internal/services/auth_service.go). You can see it work in the test results below: after an old token is reused, even the newest token stops working.

---

## 6. Login security

### The same message for every failure

Login replies `Email or password is incorrect.` whether the email doesn't exist, the password is wrong, or the account is disabled. If it said "no account with that email", attackers could use your login form to check which emails are registered, then target those people.

### Timing

There's a sneakier leak. Checking a password takes a noticeable fraction of a second (bcrypt is slow on purpose). If the email doesn't exist and the server replies instantly, an attacker can measure the response time and tell the difference. So when the email isn't found, `SpendPasswordCheckTime` runs a fake bcrypt check, and both cases take the same time.

### Rate limiting

Without a limit, an attacker can try thousands of passwords per minute against one account. [security.go](../../internal/middleware/security.go) allows each IP address about one login or register attempt every 6 seconds, with bursts of up to 10. After that the reply is **429 Too Many Requests**.

It uses a **token bucket**, a common rate-limiting method: each IP has a bucket holding up to 10 tokens. Each attempt uses one, and one token is refilled every 6 seconds. Normal users never notice; scripts get stopped.

**Limit:** the counts live in this server's memory. With several servers, each counts separately. Big sites move this to Redis or their load balancer.

### Don't trust the X-Forwarded-For header

To rate-limit by IP, you need the real IP. When a site runs behind a load balancer, the real IP arrives in an `X-Forwarded-For` header. But anyone can send that header with a fake IP and get a fresh rate limit every time. `router.SetTrustedProxies(nil)` in [server.go](../../internal/server/server.go) tells Gin to ignore that header. If you deploy behind a load balancer later, you list only its address there.

---

## 7. Never trust the client

Anything the client sends can be faked. A real attacker doesn't use your app. They send requests directly with whatever they want in them.

### Mass assignment

Imagine the register handler read the JSON straight into the `models.User` struct. An attacker sends:

```json
{"email": "x@x.com", "password": "...", "role": "admin"}
```

And they're an admin. This is called **mass assignment**, and it has caused real breaches.

The fix here: the request is read into `dto.RegisterRequest`, which **has no `Role` field**, so a `role` value in the JSON is simply ignored. The service always sets `Role: models.UserRoleCustomer`.

### DTOs vs models

That's why there are separate **DTOs** (Data Transfer Objects) in [internal/dto/](../../internal/dto/):

- **Models** match the database. `User` has `PasswordHash`.
- **DTOs** match the API. `UserResponse` has no password field at all.

You choose exactly what goes in and out. You can never accidentally send a password hash to a client, because the type sent to clients doesn't have one.

### Validation

`binding:"required,email,max=254"` tags on DTO fields are checked automatically. Failures come back as a 422 with a message per field, so the frontend can show the error next to the right input:

```json
{"error": {"code": "validation_failed", "message": "Some fields are invalid.",
  "details": {"email": "must be a valid email address", "password": "must be at least 8 characters"}}}
```

---

## 8. Race conditions (again)

Stage 1 showed two checkouts racing for the last item. The same thing can happen here.

**Two sign-ups with the same email at the same moment.** If register did "check whether the email exists, then insert", both requests could pass the check before either inserts, leaving two accounts. Instead, register just inserts and lets the database's unique index decide. The loser gets `gorm.ErrDuplicatedKey`, which becomes a 409.

**The same refresh token used twice at the same moment.** The revoke runs as:

```sql
UPDATE refresh_tokens SET revoked_at = now() WHERE id = ? AND revoked_at IS NULL
```

The `AND revoked_at IS NULL` matters. If two requests race, only one of them can change the row. The other sees "0 rows changed" and is rejected.

**General rule:** "check, then act" is broken when two requests run at once. Make the database do the check and the action in one step.

---

## 9. Errors: helpful to users, useless to attackers

The tutorial's `response.go` did this:

```go
response.Error = err.Error()
```

That sends the raw error text to the client. A database error might say something like `ERROR: relation "users" does not exist (SQLSTATE 42P01)`, which tells an attacker your table names, your database type and that something is broken. Errors can also contain file paths and SQL.

How it works now:

1. **Services return named errors** like `ErrEmailTaken` and `ErrInvalidCredentials`.
2. **Handlers check them with `errors.Is` and pick the status code** and a safe message.
3. **Anything unexpected becomes a generic 500.** The real error is written to the **log**, tagged with a **request ID**.
4. **Every response carries an `X-Request-ID` header.** If a customer reports "checkout failed", they (or your frontend) give you that ID, and you search the logs for the exact error.

Each error also has a stable `code` like `email_taken`. Frontend code should check the code, not the message, so you can reword messages without breaking the app.

---

## 10. Go concepts used in this stage

**Wrapping errors with `%w`.** `fmt.Errorf("create user: %w", err)` adds context ("this failed while creating a user") but keeps the original error inside, so `errors.Is(err, gorm.ErrDuplicatedKey)` still finds it. Use `%w`, not `%v`, when you want callers to be able to check the original.

**Sentinel errors.** `var ErrEmailTaken = errors.New(...)` creates one specific error value that other code can compare against with `errors.Is`.

**`context.Context`.** Every service method takes `ctx` first and uses `db.WithContext(ctx)`. If the user closes their browser mid-request, the context is cancelled and the database query stops instead of finishing for nobody. It also carries the request-scoped logger, which is how `internalError` logs with the right request ID.

**Dependency injection.** `NewAuthService(db, tokens, refreshTTL)` receives what it needs instead of reaching for global variables. You can see every dependency at a glance, and tests can pass in different ones.

**Closures holding state.** `RateLimit` returns a function that keeps using the `clients` map created when `RateLimit` was called. Each call to `RateLimit` gets its own separate map. The `sync.Mutex` stops two requests editing that map at the same time, which would crash Go.

**Goroutines and graceful shutdown.** In `main.go`, the server runs in a goroutine (a lightweight background thread) so `main` can wait for Ctrl+C. When the stop signal comes, `srv.Shutdown` stops accepting new requests but lets current ones finish (up to 10 seconds). Without it, every deploy would cut off customers mid-checkout.

**Struct tags.** The text in backticks, like `` `json:"first_name" binding:"required"` ``, is read by libraries: `json` names the field in JSON and `binding` holds validation rules.

---

## 11. Protecting the server itself

| Protection | Where | What it stops |
| --- | --- | --- |
| Read/write timeouts | `main.go` | A client that sends one byte per minute holding a connection forever (a "slowloris" attack). Enough of them use up the server |
| 1 MB body limit | `BodyLimit` | Someone sending a 5 GB request that the server tries to read into memory |
| Panic recovery | `Recovery` | One bug in one handler crashing the whole server for everybody |
| Health check | `GET /health` | Load balancers sending traffic to a server whose database is down |

### CORS

Browsers have a safety rule: JavaScript running on `evil.com` can't read responses from `your-api.com` unless your API explicitly allows `evil.com`. That's what CORS headers do.

The tutorial allowed `*` (every website). Now only the origins in `CORS_ALLOWED_ORIGINS` get the headers. In testing, `http://localhost:3000` got `Access-Control-Allow-Origin`, and `https://evil.example` didn't.

CORS only controls **browsers**. It doesn't stop a script or curl from calling your API directly, so it's not a replacement for authentication.

---

## 12. Try it yourself

Start everything:

```powershell
make docker-up
make run
```

In a **second** terminal (PowerShell):

```powershell
$api = "http://localhost:8080/api/v1"

# Register
$body = @{ email = "ada@example.com"; password = "supersecret1"; first_name = "Ada"; last_name = "Obi" } | ConvertTo-Json
$res = Invoke-RestMethod -Method Post -Uri "$api/auth/register" -ContentType "application/json" -Body $body
$res.data

# Who am I?
$token = $res.data.access_token
Invoke-RestMethod -Uri "$api/users/me" -Headers @{ Authorization = "Bearer $token" }

# Get a new pair of tokens
$res2 = Invoke-RestMethod -Method Post -Uri "$api/auth/refresh" -ContentType "application/json" -Body (@{ refresh_token = $res.data.refresh_token } | ConvertTo-Json)

# Try the OLD refresh token again: reuse detection kicks in (401),
# and now even $res2's refresh token is revoked.
Invoke-RestMethod -Method Post -Uri "$api/auth/refresh" -ContentType "application/json" -Body (@{ refresh_token = $res.data.refresh_token } | ConvertTo-Json)
```

Things to try:

- Paste `$token` into [jwt.io](https://jwt.io) and read the payload. Notice it isn't secret.
- Register the same email again and look for the 409.
- Look in the database: `docker exec -it ecommerce_postgres psql -U postgres -d ecommerce_shop`, then `SELECT email, password_hash FROM users;` and `SELECT token_hash, revoked_at FROM refresh_tokens;`. You'll find no real passwords or tokens anywhere.
- Watch the terminal running `make run`: each request logs one line with its `request_id`, status and duration.
- Press Ctrl+C in that terminal and read the shutdown messages.

---

## Check yourself

1. Why is a password hashed instead of encrypted?
2. bcrypt takes a fraction of a second per check. Why is that a feature, not a problem?
3. Anyone can read a JWT's payload. So what stops someone changing `"role": "customer"` to `"role": "admin"`?
4. Why do access tokens last only 15 minutes?
5. A thief steals a refresh token and uses it. Then the real user's app tries to use the same token. What happens, step by step?
6. Why does login give the same message for "no such email" and "wrong password"?
7. What's the difference between a 401 and a 403?
8. The register request body includes `"role": "admin"`. Why doesn't the user become an admin?
9. Why does the API send `Something went wrong on our side` instead of the real error text?
10. Why can register skip checking "does this email already exist?" before inserting?
