package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// WalletRepository is the pgx-backed implementation of repository.WalletRepository.
type WalletRepository struct {
	pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

func (r *WalletRepository) Get(ctx context.Context, id string) (*domain.Wallet, error) {
	const query = `SELECT id, balance, created_at, updated_at FROM wallets WHERE id = $1`
	return scanWallet(r.pool.QueryRow(ctx, query, id))
}

// GetForUpdate row-locks the wallet; only holds inside an explicit
// transaction. Not used by TransferExecutor — kept for future read-then-decide flows.
func (r *WalletRepository) GetForUpdate(ctx context.Context, id string) (*domain.Wallet, error) {
	const query = `SELECT id, balance, created_at, updated_at FROM wallets WHERE id = $1 FOR UPDATE`
	return scanWallet(r.pool.QueryRow(ctx, query, id))
}

func scanWallet(row pgx.Row) (*domain.Wallet, error) {
	var wallet domain.Wallet
	if err := row.Scan(&wallet.ID, &wallet.Balance, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, id string, newBalance int64) error {
	const query = `UPDATE wallets SET balance = $2, updated_at = now() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, newBalance)
	return err
}
