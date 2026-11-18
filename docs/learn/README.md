# Learning path

This project is built in stages. Each stage adds one part of a real e-commerce backend and comes with a lesson explaining what was built, why it was built that way, and what goes wrong if you skip it.

Read each lesson with the code open next to it. Every lesson ends with questions; if you can't answer one, that's the gap to ask about.

| Stage | Lesson | What you'll understand |
| --- | --- | --- |
| 1 | [Database foundations](01-database-foundations.md) | Tables, keys, constraints, migrations, storing money, connection pools |
| 2 | [Authentication](02-authentication.md) | Password hashing, JWTs, refresh tokens, roles, the HTTP layer |
| 3 | Products and categories | REST design, validation, pagination, filtering, admin-only routes |
| 4 | Cart | Upserts, ownership checks, keeping the cart in sync with stock |
| 5 | Checkout and orders | Transactions, row locking, overselling, idempotency, order states |
| 6 | Payments with Stripe | Payment intents, webhooks, signature checks, never trusting the client |
| 7 | Events and notifications | Queues, background workers, the outbox pattern, retries |
| 8 | Testing and deployment | Unit and integration tests, CI, Docker images, graceful shutdown |
