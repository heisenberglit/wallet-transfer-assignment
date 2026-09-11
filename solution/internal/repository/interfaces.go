package repository

import (
	"context"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// WalletRepository handles persistence for wallets.
type WalletRepository interface {
	Get(ctx context.Context, id string) (*domain.Wallet, error)
	GetForUpdate(ctx context.Context, id string) (*domain.Wallet, error)
	UpdateBalance(ctx context.Context, id string, newBalance int64) error
}

// TransferRepository handles persistence for transfers and their state transitions.
type TransferRepository interface {
	Create(ctx context.Context, t *domain.Transfer) error
	Get(ctx context.Context, id string) (*domain.Transfer, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transfer, error)
	UpdateState(ctx context.Context, id string, state domain.TransferState) error
}

// LedgerRepository handles persistence for double-entry ledger rows.
type LedgerRepository interface {
	CreateEntries(ctx context.Context, entries []domain.LedgerEntry) error
	ListForWallet(ctx context.Context, walletID string) ([]domain.LedgerEntry, error)
}

// TransferExecutor atomically debits, credits, writes the ledger
// entries, and marks the transfer PROCESSED.
type TransferExecutor interface {
	Execute(ctx context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error
}
