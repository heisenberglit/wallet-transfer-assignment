package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

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

type TransferService struct {
	wallets   repository.WalletRepository
	transfers repository.TransferRepository
	executor  repository.TransferExecutor
}

func NewTransferService(
	wallets repository.WalletRepository,
	transfers repository.TransferRepository,
	executor repository.TransferExecutor,
) *TransferService {
	return &TransferService{
		wallets:   wallets,
		transfers: transfers,
		executor:  executor,
	}
}

func (s *TransferService) CreateTransfer(ctx context.Context, in CreateTransferInput) (*domain.Transfer, error) {
	if existing, err := s.transfers.GetByIdempotencyKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		slog.Info("transfer idempotency replay", "transfer_id", existing.ID,
			"idempotency_key", in.IdempotencyKey, "state", existing.State)
		return replay(existing)
	}

	if in.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if in.FromWalletID == in.ToWalletID {
		return nil, domain.ErrSameWallet
	}

	if _, err := s.wallets.Get(ctx, in.FromWalletID); err != nil {
		return nil, err
	}
	if _, err := s.wallets.Get(ctx, in.ToWalletID); err != nil {
		return nil, err
	}

	now := time.Now()
	transfer := &domain.Transfer{
		ID:             uuid.NewString(),
		IdempotencyKey: in.IdempotencyKey,
		FromWalletID:   in.FromWalletID,
		ToWalletID:     in.ToWalletID,
		Amount:         in.Amount,
		State:          domain.TransferPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.transfers.Create(ctx, transfer); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			existing, getErr := s.transfers.GetByIdempotencyKey(ctx, in.IdempotencyKey)
			if getErr != nil {
				return nil, getErr
			}
			if existing != nil {
				slog.Info("transfer idempotency race lost, returning winner",
					"transfer_id", existing.ID, "idempotency_key", in.IdempotencyKey, "state", existing.State)
				return replay(existing)
			}
		}
		slog.Error("transfer create failed", "idempotency_key", in.IdempotencyKey, "error", err)
		return nil, err
	}

	slog.Info("transfer created", "transfer_id", transfer.ID, "idempotency_key", in.IdempotencyKey,
		"from_wallet_id", in.FromWalletID, "to_wallet_id", in.ToWalletID, "amount", in.Amount)

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: in.FromWalletID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: in.Amount, CreatedAt: now}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: in.ToWalletID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: in.Amount, CreatedAt: now}

	if err := s.executor.Execute(ctx, transfer, debit, credit); err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) {
			if updateErr := s.transfers.UpdateState(ctx, transfer.ID, domain.TransferPending, domain.TransferFailed); updateErr != nil {
				slog.Error("transfer state update to FAILED failed", "transfer_id", transfer.ID, "error", updateErr)
				return nil, updateErr
			}
			slog.Warn("transfer failed: insufficient funds", "transfer_id", transfer.ID, "idempotency_key", in.IdempotencyKey)
			transfer.State = domain.TransferFailed
			return transfer, domain.ErrInsufficientFunds
		}
		slog.Error("transfer execute failed", "transfer_id", transfer.ID, "error", err)
		return nil, err
	}

	slog.Info("transfer processed", "transfer_id", transfer.ID, "idempotency_key", in.IdempotencyKey)
	transfer.State = domain.TransferProcessed
	return transfer, nil
}

// replay reproduces the original outcome for an already-seen idempotency key,
// error included — only a PROCESSED transfer may be reported as a success.
func replay(existing *domain.Transfer) (*domain.Transfer, error) {
	switch existing.State {
	case domain.TransferFailed:
		return existing, domain.ErrInsufficientFunds
	case domain.TransferPending:
		return existing, domain.ErrTransferInProgress
	default:
		return existing, nil
	}
}
