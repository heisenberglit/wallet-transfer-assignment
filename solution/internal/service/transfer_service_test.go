package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

// Hand-rolled fakes, not a mocking framework — the interfaces are tiny.

type fakeWallets struct {
	get func(ctx context.Context, id string) (*domain.Wallet, error)
}

func (f *fakeWallets) Get(ctx context.Context, id string) (*domain.Wallet, error) {
	return f.get(ctx, id)
}

func walletFound(_ context.Context, id string) (*domain.Wallet, error) {
	return &domain.Wallet{ID: id, Balance: 1000}, nil
}

type fakeTransfers struct {
	create              func(ctx context.Context, t *domain.Transfer) error
	getByIdempotencyKey func(ctx context.Context, key string) (*domain.Transfer, error)
	updateState         func(ctx context.Context, id string, state domain.TransferState) error
}

func (f *fakeTransfers) Create(ctx context.Context, t *domain.Transfer) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, t)
}
func (f *fakeTransfers) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transfer, error) {
	if f.getByIdempotencyKey == nil {
		return nil, nil
	}
	return f.getByIdempotencyKey(ctx, key)
}
func (f *fakeTransfers) UpdateState(ctx context.Context, id string, state domain.TransferState) error {
	if f.updateState == nil {
		return nil
	}
	return f.updateState(ctx, id, state)
}

type fakeExecutor struct {
	execute func(ctx context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error
}

func (f *fakeExecutor) Execute(ctx context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error {
	return f.execute(ctx, transfer, debit, credit)
}

func validInput() service.CreateTransferInput {
	return service.CreateTransferInput{
		IdempotencyKey: "key-1",
		FromWalletID:   "wallet_1",
		ToWalletID:     "wallet_2",
		Amount:         100,
	}
}

func TestCreateTransfer_InvalidAmount(t *testing.T) {
	svc := service.NewTransferService(
		&fakeWallets{get: func(context.Context, string) (*domain.Wallet, error) {
			t.Fatal("wallets.Get should not be called")
			return nil, nil
		}},
		&fakeTransfers{},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			t.Fatal("executor.Execute should not be called")
			return nil
		}},
	)

	in := validInput()
	in.Amount = 0

	_, err := svc.CreateTransfer(context.Background(), in)
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
}

func TestCreateTransfer_SameWallet(t *testing.T) {
	svc := service.NewTransferService(
		&fakeWallets{get: func(context.Context, string) (*domain.Wallet, error) {
			t.Fatal("wallets.Get should not be called")
			return nil, nil
		}},
		&fakeTransfers{},
		&fakeExecutor{},
	)

	in := validInput()
	in.ToWalletID = in.FromWalletID

	_, err := svc.CreateTransfer(context.Background(), in)
	assert.ErrorIs(t, err, domain.ErrSameWallet)
}

func TestCreateTransfer_WalletNotFound(t *testing.T) {
	svc := service.NewTransferService(
		&fakeWallets{get: func(_ context.Context, id string) (*domain.Wallet, error) {
			return nil, domain.ErrWalletNotFound
		}},
		&fakeTransfers{},
		&fakeExecutor{},
	)

	_, err := svc.CreateTransfer(context.Background(), validInput())
	assert.ErrorIs(t, err, domain.ErrWalletNotFound)
}

func TestCreateTransfer_IdempotencyReplay(t *testing.T) {
	existing := &domain.Transfer{ID: "existing-id", IdempotencyKey: "key-1", State: domain.TransferProcessed}

	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{
			getByIdempotencyKey: func(context.Context, string) (*domain.Transfer, error) { return existing, nil },
			create: func(context.Context, *domain.Transfer) error {
				t.Fatal("Create should not be called on an idempotency replay")
				return nil
			},
		},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			t.Fatal("Execute should not be called on an idempotency replay")
			return nil
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	require.NoError(t, err)
	assert.Same(t, existing, got)
}

// A retried key whose original attempt FAILED must reproduce the original
// outcome — error included. Returning it as a plain success would tell a
// retrying client the money moved when it never did.
func TestCreateTransfer_ReplayOfFailedTransferReturnsOriginalError(t *testing.T) {
	failed := &domain.Transfer{ID: "failed-id", IdempotencyKey: "key-1", State: domain.TransferFailed}

	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{
			getByIdempotencyKey: func(context.Context, string) (*domain.Transfer, error) { return failed, nil },
			create: func(context.Context, *domain.Transfer) error {
				t.Fatal("Create should not be called on an idempotency replay")
				return nil
			},
		},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			t.Fatal("Execute should not be called on an idempotency replay")
			return nil
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	assert.ErrorIs(t, err, domain.ErrInsufficientFunds)
	require.NotNil(t, got)
	assert.Equal(t, domain.TransferFailed, got.State)
}

// Same rule on the other replay path: losing the concurrent-insert race to a
// transfer that went on to FAIL must also surface the failure.
func TestCreateTransfer_RaceLostToFailedTransferReturnsOriginalError(t *testing.T) {
	failed := &domain.Transfer{ID: "failed-id", IdempotencyKey: "key-1", State: domain.TransferFailed}

	calls := 0
	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{
			getByIdempotencyKey: func(context.Context, string) (*domain.Transfer, error) {
				calls++
				if calls == 1 {
					return nil, nil
				}
				return failed, nil
			},
			create: func(context.Context, *domain.Transfer) error { return domain.ErrIdempotencyConflict },
		},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			t.Fatal("Execute should not be called when Create lost the idempotency race")
			return nil
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	assert.ErrorIs(t, err, domain.ErrInsufficientFunds)
	assert.Same(t, failed, got)
}

func TestCreateTransfer_Success(t *testing.T) {
	var executed bool

	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{},
		&fakeExecutor{execute: func(_ context.Context, transfer *domain.Transfer, debit, credit domain.LedgerEntry) error {
			executed = true
			assert.Equal(t, domain.TransferPending, transfer.State)
			assert.Equal(t, int64(100), transfer.Amount)
			assert.Equal(t, domain.LedgerDebit, debit.Type)
			assert.Equal(t, domain.LedgerCredit, credit.Type)
			assert.Equal(t, transfer.ID, debit.TransferID)
			assert.Equal(t, transfer.ID, credit.TransferID)
			return nil
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	require.NoError(t, err)
	require.True(t, executed)
	assert.Equal(t, domain.TransferProcessed, got.State)
	assert.NotEmpty(t, got.ID)
}

func TestCreateTransfer_InsufficientFunds(t *testing.T) {
	var failedStateSet bool

	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{
			updateState: func(_ context.Context, id string, state domain.TransferState) error {
				assert.Equal(t, domain.TransferFailed, state)
				failedStateSet = true
				return nil
			},
		},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			return domain.ErrInsufficientFunds
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	assert.ErrorIs(t, err, domain.ErrInsufficientFunds)
	require.NotNil(t, got)
	assert.Equal(t, domain.TransferFailed, got.State)
	assert.True(t, failedStateSet)
}

func TestCreateTransfer_IdempotencyConflictRace(t *testing.T) {
	winner := &domain.Transfer{ID: "winner-id", IdempotencyKey: "key-1", State: domain.TransferProcessed}

	calls := 0
	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{
			getByIdempotencyKey: func(context.Context, string) (*domain.Transfer, error) {
				calls++
				if calls == 1 {
					return nil, nil // first check: not found yet
				}
				return winner, nil // second check, after losing the race: found
			},
			create: func(context.Context, *domain.Transfer) error {
				return domain.ErrIdempotencyConflict
			},
		},
		&fakeExecutor{execute: func(context.Context, *domain.Transfer, domain.LedgerEntry, domain.LedgerEntry) error {
			t.Fatal("Execute should not be called when Create lost the idempotency race")
			return nil
		}},
	)

	got, err := svc.CreateTransfer(context.Background(), validInput())
	require.NoError(t, err)
	assert.Same(t, winner, got)
}

// CreatedAt should be stamped to roughly now, not left zero-value.
func TestCreateTransfer_StampsTimestamps(t *testing.T) {
	before := time.Now()

	var gotTransfer *domain.Transfer
	svc := service.NewTransferService(
		&fakeWallets{get: walletFound},
		&fakeTransfers{},
		&fakeExecutor{execute: func(_ context.Context, transfer *domain.Transfer, _, _ domain.LedgerEntry) error {
			gotTransfer = transfer
			return nil
		}},
	)

	_, err := svc.CreateTransfer(context.Background(), validInput())
	require.NoError(t, err)
	require.NotNil(t, gotTransfer)
	assert.False(t, gotTransfer.CreatedAt.Before(before))
}
