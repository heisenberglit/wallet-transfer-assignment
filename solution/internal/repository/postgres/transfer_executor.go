package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
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

// Execute performs the debit, credit, ledger writes and state transition in a
// single transaction. Three deliberate choices:
//
//   - Wallet UPDATEs go in ascending wallet-id order, not source-first: an
//     UPDATE holds its row lock until commit, so A->B racing B->A would
//     otherwise deadlock. When the credit sorts first it lands before the
//     debit is known to succeed — a failed debit rolls the whole transaction
//     back, so nothing leaks.
//
//   - The debit is a conditional UPDATE (WHERE balance >= $amount). Under
//     READ COMMITTED, Postgres re-evaluates that condition against the current
//     row version once the lock is granted, so the check and the decrement are
//     atomic with no separate read. Zero rows means insufficient funds (also a
//     missing wallet, which cannot happen today).
//
//   - The state change is a compare-and-swap (WHERE state = PENDING), so a
//     second Execute against the same transfer fails instead of re-applying it.
func (e *TransferExecutor) Execute(ctx context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error {
	if debit.Amount != transfer.Amount || credit.Amount != transfer.Amount {
		return fmt.Errorf("ledger entries inconsistent with transfer %s", transfer.ID)
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.WithoutCancel(ctx))

	applyDebit := func() error { return debitWallet(ctx, tx, transfer) }
	applyCredit := func() error { return creditWallet(ctx, tx, transfer) }

	if transfer.FromWalletID < transfer.ToWalletID {
		if err := applyDebit(); err != nil {
			return err
		}
		if err := applyCredit(); err != nil {
			return err
		}
	} else {
		if err := applyCredit(); err != nil {
			return err
		}
		if err := applyDebit(); err != nil {
			return err
		}
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

	tag, err := tx.Exec(ctx,
		`UPDATE transfers SET state = $3, updated_at = now() WHERE id = $1 AND state = $2`,
		transfer.ID, domain.TransferPending, domain.TransferProcessed)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidStateTransition
	}

	return tx.Commit(ctx)
}

func debitWallet(ctx context.Context, tx pgx.Tx, transfer *domain.Transfer) error {
	tag, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = balance - $1, updated_at = now() WHERE id = $2 AND balance >= $1`,
		transfer.Amount, transfer.FromWalletID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInsufficientFunds
	}
	return nil
}

func creditWallet(ctx context.Context, tx pgx.Tx, transfer *domain.Transfer) error {
	_, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1, updated_at = now() WHERE id = $2`,
		transfer.Amount, transfer.ToWalletID)
	return err
}
