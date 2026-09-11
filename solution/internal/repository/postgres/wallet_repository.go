package postgres

import (
	"context"
	"errors"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// WalletRepository is the pgx-backed implementation of repository.WalletRepository.
type WalletRepository struct {
	db DBTX
}

func NewWalletRepository(db DBTX) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) Get(ctx context.Context, id string) (*domain.Wallet, error) {
	// TODO: SELECT id, balance, created_at, updated_at FROM wallets WHERE id = $1
	return nil, errors.New("not implemented")
}

// GetForUpdate reads the wallet row with a row-level lock so concurrent
// transfers touching the same wallet serialize instead of racing.
// TODO: SELECT ... FROM wallets WHERE id = $1 FOR UPDATE, run inside the
// same transaction as the balance update in UpdateBalance.
func (r *WalletRepository) GetForUpdate(ctx context.Context, id string) (*domain.Wallet, error) {
	return nil, errors.New("not implemented")
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, id string, newBalance int64) error {
	// TODO: UPDATE wallets SET balance = $2, updated_at = now() WHERE id = $1
	return errors.New("not implemented")
}
