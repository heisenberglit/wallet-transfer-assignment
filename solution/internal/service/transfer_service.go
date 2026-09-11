package service

import (
	"context"
	"errors"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/repository"
)

// CreateTransferInput is the service-layer request for a new transfer.
type CreateTransferInput struct {
	IdempotencyKey string
	FromWalletID   string
	ToWalletID     string
	Amount         int64
}

// TransferService owns the transfer workflow: validation, idempotency,
// the debit/credit/ledger transaction, and state transitions. Handlers
// should contain no business logic beyond calling into this layer.
type TransferService struct {
	wallets   repository.WalletRepository
	transfers repository.TransferRepository
	ledger    repository.LedgerRepository
	uow       repository.UnitOfWork
}

func NewTransferService(
	wallets repository.WalletRepository,
	transfers repository.TransferRepository,
	ledger repository.LedgerRepository,
	uow repository.UnitOfWork,
) *TransferService {
	return &TransferService{
		wallets:   wallets,
		transfers: transfers,
		ledger:    ledger,
		uow:       uow,
	}
}

// CreateTransfer executes a wallet-to-wallet transfer.
//
// TODO implement, following the "Documentation-First Workflow": update
// docs/design.md with the exact idempotency and concurrency strategy
// before filling this in. At minimum the implementation must:
//  1. Validate input (amount > 0, fromWalletID != toWalletID).
//  2. Check transfers.GetByIdempotencyKey — if found, return the existing
//     transfer instead of re-executing (exactly-once semantics).
//  3. Within a single DB transaction (uow.WithinTransaction):
//     - lock and re-check the source wallet's balance
//     - debit the source wallet, credit the destination wallet
//     - write both ledger entries
//     - transition the transfer PENDING -> PROCESSED (or -> FAILED on
//     insufficient funds / error, without leaking a partially applied
//     state)
//  4. Return the resulting domain.Transfer.
func (s *TransferService) CreateTransfer(ctx context.Context, in CreateTransferInput) (*domain.Transfer, error) {
	return nil, errors.New("not implemented")
}
