# Design Notes

Following the assignment's "Documentation-First Workflow": fill this in
*before* writing non-trivial code, and keep it updated as decisions change.

## Problem Statement

Wallet-to-wallet transfer service with idempotent, atomic, ledger-backed
transfers. See [`../../ASSIGNMENT.md`](../../ASSIGNMENT.md) for the full brief.

## API / Event Contract

```
POST /transfers
{
  "idempotencyKey": "abc123",
  "fromWalletId": "wallet_1",
  "toWalletId": "wallet_2",
  "amount": 100
}
```

- TODO: document the response shape for both the first call and replayed
  duplicate calls, and the status codes for each failure mode
  (validation error, wallet not found, insufficient funds, idempotency key
  reused with a different payload).

## Side Effects

- TODO: list every side effect a successful transfer causes (balance
  updates, ledger rows, state transition) and confirm they all happen
  atomically.

## Failure Modes

- TODO: insufficient funds, unknown wallet, self-transfer, concurrent
  debit on the same wallet, crash mid-transfer, duplicate request with a
  different payload under the same idempotency key.

## Idempotency Strategy

- TODO: where is `idempotencyKey` stored — a unique constraint on
  `transfers.idempotency_key`, or a separate `idempotency_records` table?
  This scaffold picked the former (see
  `migrations/0001_init.up.sql`); revisit if a stored
  request/response snapshot is needed to detect key reuse with a
  different payload.
- TODO: what happens when two requests with the same key race each other?

## Concurrency Strategy

- TODO: row-level locks (`SELECT ... FOR UPDATE`) vs. optimistic locking
  vs. database isolation level. Justify the choice for the "two transfers
  debit the same wallet concurrently" scenario.

## Consistency Expectations

- TODO: the ledger must always balance (sum of debits == sum of credits
  per transfer); balances must never go negative.

## Observability Expectations

- TODO: what gets logged/measured for a transfer (state transitions,
  latency, failure reasons)?

## Testing Strategy

- TODO: unit tests per layer, a concurrency test that fires concurrent
  transfers at the same wallet, and idempotency replay tests. Follow
  Red -> Blue -> Green per the assignment.
