# LedgerCore — API Contract

The frontend talks to the ledger through one module (`src/api/client.js`). This document is the contract your backend must satisfy so you can drop the HTTP client in place of the mock. The executable reference is `src/api/mockLedger.js` — when this doc and the mock disagree, the mock is the source of truth for behavior.

## Conventions
- **Amounts**: signed integers in **minor units** (`10000` = `100.00`). Never floats.
- **Currency**: exactly 3 uppercase letters (`^[A-Z]{3}$`).
- **IDs**: UUID v4 strings.
- **Timestamps**: ISO 8601 UTC strings.
- **Response envelope** (what the UI consumes): `{ status, ok, body, meta }`.
  - `status` HTTP status code; `ok` = 2xx; `body` = JSON below; `meta.cached` = true on an idempotent replay.
  - On a real HTTP client, `status`/`ok` come from the response, `body` from JSON, and `meta.cached` from a signal of your choice (e.g. an `Idempotency-Replayed: true` header).
- **Errors** always return `{ error: string, code: string, ...context }`. Never raw stack traces.

---

## POST /transactions
Create (commit) a transaction. Carries an idempotency key (header `Idempotency-Key` or body field `idempotency_key`).

**Request**
```json
{
  "description": "Customer payment — invoice 8821",
  "idempotency_key": "0f1c…uuid",
  "entries": [
    { "account_id": "…a", "currency": "USD", "amount": -2000 },
    { "account_id": "…rev", "currency": "USD", "amount": 2000 }
  ]
}
```

**Processing order (must match the mock):**
1. **Idempotency check first.** If the key was seen before:
   - same key + **identical** body → return the **original response** verbatim, create nothing, signal cached. (Mock returns the cached status — `201` for a prior success, or the cached `400` for a prior rejection.)
   - same key + **different** body → `409`.
2. **Structural validation** → `400` if: no entries; unknown `account_id`; bad currency; non-integer amount.
3. **Zero-sum per currency** → `400` if any currency's entries don't net to 0.
4. **Overdraft** → `400` if any `overdraft_protected` account would go negative in any currency.
5. **Commit** → `201` with the full Transaction (status `committed`, `committed_at` set, server-generated entry ids). The response does **not** echo a top-level `idempotency_key`.

**Responses**
- `201` → `Transaction`
- `400 unbalanced` → `{ "error": "transaction does not net to zero per currency", "code": "unbalanced", "net": { "USD": 500 } }`
- `400 overdraft` → `{ "error": "would overdraw account liabilities:customer_a", "code": "overdraft", "account": "liabilities:customer_a", "currency": "USD" }`
- `400` structural → codes: `empty_transaction`, `unknown_account`, `bad_currency`, `bad_amount`
- `409` → `{ "error": "Same idempotency key, different body", "code": "idempotency_conflict", "idempotency_key": "…" }`

> **Only successful responses are cached.** A 400/409 does not occupy the key — a corrected retry under the same key can still commit. (The mock enforces this.)

## GET /transactions/:id
- `200` → `Transaction`
- `404` → `{ "error": "no transaction with that id", "code": "not_found" }`

## POST /transactions/:id/reverse
Create the mirror-image (negated entries) transaction. The reversal still passes overdraft checks. Reverse just creates the mirror — there are no "already reversed" / "cannot reverse a reversal" guards.
- `201` → the new reversing `Transaction` with `reverses_transaction_id` = the original id. (No back-pointer is written onto the original.)
- `400 overdraft` → same shape as above (if reversal would overdraw a protected account).
- `404` → not found.

---

## GET /accounts
- `200` → `{ "accounts": Account[] }`

## POST /accounts
Create an account (used by the inline "+ New account" dropdown option). New accounts are **not** overdraft-protected by default.
- `201` → `Account`
- `400` → `{ code: "bad_label" }` ; `409` → `{ code: "duplicate_label" }`

## GET /accounts/:id/balance?as_of=ISO
Derived balances. `as_of` optional → time travel (sum entries with `committed_at <= as_of`).
- `200` → `{ "balances": [ { "currency": "USD", "amount": 5000 } ] }` (sorted by currency)

## GET /accounts/:id/entries?as_of=ISO
Recent entries touching the account (newest first), each with tx context.
- `200` → `{ "entries": [ { "id", "transaction_id", "currency", "amount", "created_at" } ] }`

---

## GET /overview
Dashboard counts + recent activity (drives the Overview screen).
- `200` →
```json
{
  "total": 8, "committed": 6, "pending": 1, "failed": 1,
  "recent": [
    { "id": "…", "status": "committed", "description": "…", "lines": 2, "created_at": "ISO", "tx_id": "…|null" }
  ]
}
```
`recent` = latest 8 write attempts (committed/pending/failed). `tx_id` is null for non-committed attempts.

## POST /reconcile
Run invariant checks across committed transactions.
- `200` →
```json
{
  "ok": true,
  "ran_at": "ISO",
  "duration_ms": 1.2,
  "checks": [
    { "name": "Every committed transaction balances to zero per currency", "ok": true,  "detail": "all transactions net to 0" },
    { "name": "No committed transaction has zero entries",                  "ok": true,  "detail": "none empty" },
    { "name": "No entries without a committed transaction",                 "ok": true,  "detail": "no orphaned entries" },
    { "name": "No lingering pending transactions",                          "ok": true,  "detail": "none pending" }
  ],
  "issues": [ { "tx_id": "…", "failure": "unbalanced", "detail": "USD +1" } ]
}
```
`ok` = all checks pass. `issues` lists offending transactions when a check fails.

---

## Demo-only endpoints (do NOT ship)
The prototype includes one affordance purely to reset state between demos. Gate it behind a demo flag or omit entirely in production.
- `POST /demo/reset` — rebuild the seed dataset (see below).

## Seed dataset (what /demo/reset rebuilds)
Accounts (`label`, protected?): `cash:operating` (no), `cash:fx_buffer` (no), `revenue:sales` (no), `fees:processing` (no), `liabilities:customer_a` (**yes**), `liabilities:customer_b` (**yes**).

Six committed transactions across ~3 weeks (so time travel visibly changes balances), keeping protected accounts non-negative at every step:
| When | Description | Entries |
|---|---|---|
| 21d ago | Customer funding — wire | cash:operating −10000 USD · customer_a +10000 USD |
| 14d ago | Customer funding — ACH | cash:operating −5000 USD · customer_b +5000 USD |
| 10d ago | Internal transfer a→b | customer_a −3000 USD · customer_b +3000 USD |
| 5d ago  | Processing fee | customer_b −100 USD · fees:processing +100 USD |
| 2d ago  | FX funding (EUR) | cash:fx_buffer −4000 EUR · customer_a +4000 EUR |
| 1d ago  | Customer payment — invoice 8821 | customer_a −2000 USD · revenue:sales +2000 USD |

After seeding, `liabilities:customer_a` holds **both USD and EUR**, so multi-currency balances and the per-currency checker are exercised. Reconciliation against this dataset always passes. The Overview's pending/failed counts come from two extra non-committed log entries the mock seeds; with a real backend these arise naturally from real pending/failed writes.
