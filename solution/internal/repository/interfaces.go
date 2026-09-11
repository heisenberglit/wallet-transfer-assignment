package repository

import (
	"context"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// WalletRepository handles persistence for wallets.
//
// TODO: decide whether GetForUpdate takes an explicit row lock (SELECT ...
// FOR UPDATE) or whether concurrency is handled via optimistic locking on
// an UpdateBalance version column. Document the choice in docs/design.md.
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

// UnitOfWork wraps a set of repository operations in a single atomic
// transaction so the service layer can compose them without knowing about
// the underlying database driver.
//
// TODO: implement in internal/repository/postgres using pgx.Tx.
type UnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
