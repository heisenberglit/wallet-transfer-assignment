-- Seed wallets for manual/local testing (there's no wallet-creation API yet).
-- Re-runnable and a true reset: prior transfers, ledger rows and idempotency
-- keys for these wallets are cleared, so balances cannot disagree with the
-- ledger and an old key cannot replay. Local development only.

BEGIN;

DELETE FROM ledger_entries
WHERE wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
   OR transfer_id IN (
        SELECT id FROM transfers
        WHERE from_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
           OR to_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
   );

DELETE FROM transfers
WHERE from_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
   OR to_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3');

INSERT INTO wallets (id, balance) VALUES
    ('wallet_1', 1000),
    ('wallet_2', 500),
    ('wallet_3', 0)
ON CONFLICT (id) DO UPDATE SET balance = EXCLUDED.balance, updated_at = now();

COMMIT;
