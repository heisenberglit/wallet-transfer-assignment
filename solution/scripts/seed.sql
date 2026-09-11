-- Seed wallets for manual/local testing (there's no wallet-creation API yet).
-- Re-runnable: resets balances to these values instead of erroring on conflict.

INSERT INTO wallets (id, balance) VALUES
    ('wallet_1', 1000),
    ('wallet_2', 500),
    ('wallet_3', 0)
ON CONFLICT (id) DO UPDATE SET balance = EXCLUDED.balance, updated_at = now();
