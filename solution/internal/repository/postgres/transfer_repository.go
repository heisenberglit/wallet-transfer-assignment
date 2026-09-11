package postgres

import (
	"context"
	"errors"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
)

// TransferRepository is the pgx-backed implementation of repository.TransferRepository.
type TransferRepository struct {
	db DBTX
}

func NewTransferRepository(db DBTX) *TransferRepository {
	return &TransferRepository{db: db}
}

func (r *TransferRepository) Create(ctx context.Context, t *domain.Transfer) error {
	// TODO: INSERT INTO transfers (...) VALUES (...)
	// Rely on a UNIQUE constraint on idempotency_key so a race between two
	// concurrent requests with the same key fails one of them cleanly
	// instead of creating two transfers.
	return errors.New("not implemented")
}

func (r *TransferRepository) Get(ctx context.Context, id string) (*domain.Transfer, error) {
	return nil, errors.New("not implemented")
}

func (r *TransferRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transfer, error) {
	// TODO: SELECT ... FROM transfers WHERE idempotency_key = $1
	return nil, errors.New("not implemented")
}

func (r *TransferRepository) UpdateState(ctx context.Context, id string, state domain.TransferState) error {
	// TODO: UPDATE transfers SET state = $2, updated_at = now() WHERE id = $1
	return errors.New("not implemented")
}
