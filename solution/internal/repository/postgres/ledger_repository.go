package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// LedgerRepository is the pgx-backed implementation of repository.LedgerRepository.
type LedgerRepository struct {
	pool *pgxpool.Pool
}

func NewLedgerRepository(pool *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{pool: pool}
}

func (r *LedgerRepository) CreateEntries(ctx context.Context, entries []domain.LedgerEntry) error {
	const query = `
		INSERT INTO ledger_entries (id, wallet_id, transfer_id, type, amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	for _, entry := range entries {
		if _, err := r.pool.Exec(ctx, query, entry.ID, entry.WalletID, entry.TransferID, entry.Type, entry.Amount, entry.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *LedgerRepository) ListForWallet(ctx context.Context, walletID string) ([]domain.LedgerEntry, error) {
	const query = `
		SELECT id, wallet_id, transfer_id, type, amount, created_at
		FROM ledger_entries
		WHERE wallet_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var entry domain.LedgerEntry
		if err := rows.Scan(&entry.ID, &entry.WalletID, &entry.TransferID, &entry.Type, &entry.Amount, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
