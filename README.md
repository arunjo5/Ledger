# LedgerCore

A single-node, double-entry payment ledger in Go and Postgres, with a React web console.

This isn't a payments product. It's built to stay correct under retries, concurrency, and crashes. The core rules are enforced in the database, not just the application code, so they hold even if the Go layer has a bug. Balances aren't stored; they're computed from the entries. Entries are immutable, and a transaction that doesn't balance can't commit.

## Key features

- Entries are immutable and stored as integer cents. No floats, no edits or deletes; to reverse a transaction you post an opposite one.
- Balances are not stored. They're computed as `SUM(entries.amount)` per currency. Entries are timestamped, so you can also query a balance as of any past time.
- Every committed transaction nets to zero per currency. A `DEFERRABLE INITIALLY DEFERRED` trigger enforces it, so an unbalanced transaction can't commit even through raw SQL. The Go layer checks it too.
- Writes are idempotent. A request carries an idempotency key. The same key and body returns the original response; a different body returns 409.
- Overdraft-protected accounts can't go negative. The check runs at commit under a row lock, so concurrent withdrawals can't overdraw.
- Transactions use a two-phase write: intent first, then committed atomically with its entries. A crash in between leaves a pending row, which startup recovery marks as failed. A reconciliation endpoint re-checks every invariant on demand.

## Quick start guide

Requires Go 1.25+ and Docker.

```
make up        # start Postgres
make migrate   # apply the schema
make seed      # load demo accounts and transactions
make run       # start the API on :8080
```

Then start the web console. It is the React frontend; it runs on port 5173 and forwards its API calls to the server on port 8080.

```
cd web
npm install
npm run dev
```

## API

| method and path | description |
| --- | --- |
| `POST /transactions` | create a transaction (requires an `Idempotency-Key` header) |
| `GET /transactions`, `GET /transactions/{id}` | list (newest first) or fetch one |
| `POST /transactions/{id}/reverse` | create the reversing transaction |
| `GET /accounts`, `POST /accounts` | list or create accounts |
| `GET /accounts/{id}/balance?as_of=` | derived balances, optionally as of a past time |
| `GET /accounts/{id}/entries` | recent entries for an account |
| `POST /admin/reconcile` | re-check the global invariants |
| `GET /overview`, `GET /health` | dashboard counts, database health |

Amounts are always signed integers in cents. Errors are returned as `{ "error", "code" }`.

## Tests

`make test` runs the suite against a throwaway Postgres database using Testcontainers, so the real triggers are active, not mocked. Property-based tests generate random transaction histories and assert that conservation and per-account balances always hold. A crash-injection test covers recovery, and a concurrency test covers overdraft.
