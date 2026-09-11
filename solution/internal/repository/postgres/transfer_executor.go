package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// TransferExecutor is the pgx-backed implementation of repository.TransferExecutor.
type TransferExecutor struct {
	pool *pgxpool.Pool
}

func NewTransferExecutor(pool *pgxpool.Pool) *TransferExecutor {
	return &TransferExecutor{pool: pool}
}

// Execute runs the debit/credit/ledger/state-transition as one transaction;
// the debit is a single conditional UPDATE so the row lock covers the balance check.
func (e *TransferExecutor) Execute(ctx context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	tag, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = balance - $1, updated_at = now() WHERE id = $2 AND balance >= $1`,
		transfer.Amount, transfer.FromWalletID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInsufficientFunds
	}

	if _, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1, updated_at = now() WHERE id = $2`,
		transfer.Amount, transfer.ToWalletID); err != nil {
		return err
	}

	const insertLedgerEntry = `
		INSERT INTO ledger_entries (id, wallet_id, transfer_id, type, amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	for _, entry := range []domain.LedgerEntry{debit, credit} {
		if _, err := tx.Exec(ctx, insertLedgerEntry,
			entry.ID, entry.WalletID, entry.TransferID, entry.Type, entry.Amount, entry.CreatedAt); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE transfers SET state = $2, updated_at = now() WHERE id = $1`,
		transfer.ID, domain.TransferProcessed); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
