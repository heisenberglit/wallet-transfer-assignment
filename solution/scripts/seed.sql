BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM transfers
        WHERE (from_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
            OR to_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3'))
          AND NOT (from_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3')
               AND to_wallet_id IN ('wallet_1', 'wallet_2', 'wallet_3'))
    ) THEN
        RAISE EXCEPTION 'seed.sql: transfers exist between a seeded wallet and one outside the seed set; refusing to reset. Use a clean database (make down && make up && make migrate-up).';
    END IF;
END $$;

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
