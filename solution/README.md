# Wallet Transfer Service

A wallet-to-wallet transfer service with idempotent, atomic, double-entry
ledger-backed transfers. Implements the brief in
[`../ASSIGNMENT.md`](../ASSIGNMENT.md).

## Architecture

```
cmd/server/main.go        entry point: wiring, HTTP server, graceful shutdown
internal/handler/http/    HTTP handlers, routing, request/response DTOs
internal/service/         business logic: validation, idempotency, orchestration
internal/repository/      persistence interfaces (postgres/ implements them)
internal/domain/          entities, state machine, sentinel errors
internal/db/              Postgres connection pool
internal/utils/           small cross-cutting helpers (Postgres error-code checks)
migrations/               SQL schema
scripts/seed.sql          wallet seed data for manual testing (make seed)
```

Layering: **handler** does request/response mapping only; **service**
owns validation and the transfer workflow; **repository** does
persistence and nothing else; **domain** has no knowledge of HTTP or SQL.
The service depends on repository *interfaces*
(`internal/repository/interfaces.go`), not the concrete Postgres types,
so it's testable without a database (see `transfer_service_test.go`'s
hand-rolled fakes).

### The transfer write path

```
POST /transfers
   │
   ▼
TransferHandler.Create          (decode JSON, map errors → HTTP status)
   │
   ▼
TransferService.CreateTransfer  (validate, check idempotency, orchestrate)
   │
   ├─ wallets.Get × 2            confirm both wallets exist
   ├─ transfers.GetByIdempotencyKey   replay if this key was already used
   ├─ transfers.Create           insert the transfer row as PENDING
   └─ executor.Execute           atomic debit + credit + ledger + PROCESSED
```

`TransferExecutor.Execute` (`internal/repository/postgres/transfer_executor.go`)
is the one place a database transaction spans more than one table. It's a
single purpose-built method, not a generic reusable unit-of-work —
transfers are the only operation in this service that need a
multi-statement transaction, so there's no second caller to justify that
abstraction.

The debit is a single conditional `UPDATE`:

```sql
UPDATE wallets SET balance = balance - $1, updated_at = now()
WHERE id = $2 AND balance >= $1
```

Postgres takes the row lock as part of the `UPDATE` itself, so the
balance check and the decrement happen atomically — no separate
`SELECT ... FOR UPDATE`, no window for a concurrent transfer to read a
stale balance, and no explicit lock-ordering rule needed to avoid
deadlocking against a transfer moving funds the other way.

## API

### `POST /transfers`

Request:

```json
{
  "idempotencyKey": "abc123",
  "fromWalletId": "wallet_1",
  "toWalletId": "wallet_2",
  "amount": 100
}
```

Response (201, both on the first call and on a replayed duplicate):

```json
{
  "id": "90c0261c-327a-4476-91b1-afe6dbec4615",
  "idempotencyKey": "abc123",
  "fromWalletId": "wallet_1",
  "toWalletId": "wallet_2",
  "amount": 100,
  "state": "PROCESSED"
}
```

| Case                                   | Status |
|-----------------------------------------|--------|
| Success                                 | 201    |
| Replayed duplicate (same `idempotencyKey`) | 201, identical body to the original |
| `amount <= 0` / self-transfer            | 422    |
| Insufficient funds                       | 422    |
| Unknown wallet                           | 404    |
| Malformed JSON body                      | 400    |

There is currently no endpoint to create a wallet or read a balance —
see Known Limitations below.

## How to Build

```bash
go build ./...
```

## How to Run

```bash
cp .env.example .env
make up             # starts Postgres on localhost:5432
make migrate-up      # applies migrations/0001_init.up.sql
make seed            # seeds wallet_1 (1000), wallet_2 (500), wallet_3 (0) — scripts/seed.sql
go run ./cmd/server
```

`make seed` is re-runnable — it resets those three wallets' balances
instead of erroring on conflict, so you can reset state between manual
test runs without recreating the database. There's no wallet-creation
API yet (see Known Limitations), so this script is the only way to get
a wallet to test against.

```bash
curl -X POST http://localhost:8080/transfers \
  -H "Content-Type: application/json" \
  -d '{"idempotencyKey":"key-1","fromWalletId":"wallet_1","toWalletId":"wallet_2","amount":100}'
```

## How to Test

```bash
go test ./...                    # unit tests only; Postgres tests self-skip
```

Two kinds of tests:

- **Unit tests** (`internal/service/transfer_service_test.go`) — no
  database, using hand-rolled fakes for the repository/executor
  interfaces. Cover validation, the idempotency-replay path, the
  same-key race, insufficient funds, and the success path.
- **Integration tests** (`internal/repository/postgres/integration_test.go`)
  — need a real Postgres with the migration applied; they skip
  themselves (not fail) if `DATABASE_URL` isn't set. Cover repository
  CRUD, `TransferExecutor.Execute` (success and insufficient funds), and
  a concurrency test that fires many simultaneous debits at the same
  wallet and asserts the final balance is exactly right — this is the
  test that actually proves the concurrency-safety claim, not just
  compiles it:

```bash
docker compose up -d
docker compose exec -T postgres psql -U postgres -d wallet_transfer -f - < migrations/0001_init.up.sql
DATABASE_URL="postgres://postgres:postgres@localhost:5432/wallet_transfer?sslmode=disable" go test ./...
```

Both were run against a real local Postgres as part of building this,
not just written and left unverified.

## Observability

Structured (`log/slog`, JSON in production) logs, not `log.Printf`:

- One line per HTTP request (`internal/handler/http/middleware.go`):
  method, path, status, duration.
- One line per meaningful transfer event
  (`internal/service/transfer_service.go`): created, processed, failed
  (insufficient funds), idempotency replay, idempotency race lost —
  each tagged with `transfer_id` and `idempotency_key` so one transfer's
  whole lifecycle can be traced through the logs.

Not implemented: metrics/counters (e.g. `PROCESSED` vs `FAILED` rate)
and anything that would detect a transfer stuck in `PENDING` after a
crash — see Known Limitations.

## Deploy

```bash
docker build -t wallet-transfer-service .
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/wallet_transfer?sslmode=disable" \
  wallet-transfer-service
```

Multi-stage build (`Dockerfile`): compiles a static binary in a
`golang:1.23-alpine` stage, ships it in a bare `alpine` image running as
a non-root user. Configuration is entirely via environment variables —
`DATABASE_URL` and `HTTP_ADDR` (default `:8080`) — no config files, so
it drops into whatever secrets/config mechanism the target platform
already has. This was built and run end-to-end (`docker build`, then the
container hitting a `docker compose` Postgres) as part of writing it.

Not included, and worth doing before this went anywhere near real
production traffic: a `/healthz` endpoint for orchestrator liveness
checks, and running the migration as an explicit release step rather
than the manual `psql` invocation above.

## Pros and Cons of the Key Decisions

**Conditional `UPDATE` instead of `SELECT ... FOR UPDATE` + application
check.**
✅ Simpler: one statement instead of read-then-decide-then-write; no
explicit lock-ordering rule needed for two wallets.
✅ Correct under concurrency by construction — verified by the
`TestTransferExecutor_Execute_ConcurrentDebits` test.
❌ The insufficient-funds check is now inside the SQL `WHERE` clause
rather than visible as service-layer business logic — a reviewer looking
only at `internal/service` wouldn't find it there.

**`TransferExecutor` (one purpose-built method) instead of a generic
`UnitOfWork`.**
✅ Less code, more readable — the entire effect of a transfer is visible
in one function.
✅ Matches how ledger-writing systems often prefer explicit,
one-operation-per-function code for auditability.
❌ Doesn't reuse `WalletRepository`/`LedgerRepository`'s own methods (it
duplicates a few lines of SQL instead) — the trade-off point is
documented in `transfer_executor.go`'s comment.
❌ If a second multi-table atomic operation shows up (a refund, say),
this pattern doesn't extend cleanly — that's the point at which a shared
transaction helper would earn its cost.

**Idempotency via a `UNIQUE` constraint on `transfers.idempotency_key`,
not a separate `idempotency_records` table.**
✅ One fewer table; the constraint is what actually prevents the
duplicate, so there's no gap between "checked" and "enforced."
❌ Can only dedupe by key — reusing a key with a *different* payload is
not detected (see Known Limitations).

**Wallet balance as a stored, updated column (not derived from summing
`ledger_entries`).**
✅ Reading a balance is O(1), and the conditional `UPDATE` technique
depends on it being a real column to check against.
❌ Balance and the ledger are two sources of truth that could drift if a
future code path updates one without the other — nothing currently
reconciles them against each other.

## Known Limitations

- **Idempotency-key reuse with a different payload isn't detected.** A
  client resending the same key with different amount/wallets silently
  gets the original transfer back. Fixing this needs storing (or
  hashing) the original request and comparing on lookup.
- **A crash between creating the `PENDING` row and `Execute` running
  leaves that transfer stuck in `PENDING` forever**, with no
  reconciliation job to find and resolve it.
- **No automated tests for the HTTP handler layer** (`TransferHandler`)
  — only the service and repository layers are covered.
- **No wallet-creation or balance-read API** — wallets only exist via
  `scripts/seed.sql` (`make seed`). Listed as optional in the assignment,
  but worth calling out since there's no way to create an arbitrary
  wallet through the running service itself.
- **On insufficient funds, the client doesn't get the transfer's id** —
  just `{"error": "insufficient funds"}` with 422, even though the
  `FAILED` transfer row does exist. Revisit if a client needs to look up
  a failed attempt later.
