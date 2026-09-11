package postgres

import (
	"context"
	"errors"

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

// CreateEntries inserts the DEBIT and CREDIT rows for a transfer.
// TODO: batch insert both rows in the same transaction as the balance
// updates and the transfer state transition.
func (r *LedgerRepository) CreateEntries(ctx context.Context, entries []domain.LedgerEntry) error {
	return errors.New("not implemented")
}

func (r *LedgerRepository) ListForWallet(ctx context.Context, walletID string) ([]domain.LedgerEntry, error) {
	return nil, errors.New("not implemented")
}
