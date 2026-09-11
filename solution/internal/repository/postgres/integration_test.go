package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/db"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/domain"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/repository/postgres"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))

	t.Cleanup(pool.Close)
	return pool
}

func seedWallets(t *testing.T, pool *pgxpool.Pool, fromBalance, toBalance int64) (fromID, toID string) {
	t.Helper()
	ctx := context.Background()

	fromID = "test_" + uuid.NewString()
	toID = "test_" + uuid.NewString()

	_, err := pool.Exec(ctx, `INSERT INTO wallets (id, balance) VALUES ($1, $2), ($3, $4)`,
		fromID, fromBalance, toID, toBalance)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM ledger_entries WHERE wallet_id IN ($1, $2)`, fromID, toID)                             //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM transfers WHERE from_wallet_id IN ($1, $2) OR to_wallet_id IN ($1, $2)`, fromID, toID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM wallets WHERE id IN ($1, $2)`, fromID, toID)                                           //nolint:errcheck
	})

	return fromID, toID
}

func newPendingTransfer(fromID, toID string, amount int64) *domain.Transfer {
	now := time.Now()
	return &domain.Transfer{
		ID:             uuid.NewString(),
		IdempotencyKey: uuid.NewString(),
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         amount,
		State:          domain.TransferPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestTransferRepository_CreateGetAndIdempotencyKeyLookup(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 0)
	repo := postgres.NewTransferRepository(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, repo.Create(ctx, transfer))

	byID, err := repo.Get(ctx, transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, transfer.ID, byID.ID)
	assert.Equal(t, domain.TransferPending, byID.State)

	byKey, err := repo.GetByIdempotencyKey(ctx, transfer.IdempotencyKey)
	require.NoError(t, err)
	require.NotNil(t, byKey)
	assert.Equal(t, transfer.ID, byKey.ID)

	notFound, err := repo.GetByIdempotencyKey(ctx, "no-such-key-"+uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, notFound)

	require.NoError(t, repo.UpdateState(ctx, transfer.ID, domain.TransferProcessed))
	updated, err := repo.Get(ctx, transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransferProcessed, updated.State)
}

func TestTransferRepository_Create_DuplicateIdempotencyKey(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 0)
	repo := postgres.NewTransferRepository(pool)
	ctx := context.Background()

	key := uuid.NewString()
	first := newPendingTransfer(fromID, toID, 100)
	first.IdempotencyKey = key
	require.NoError(t, repo.Create(ctx, first))

	second := newPendingTransfer(fromID, toID, 200)
	second.IdempotencyKey = key
	err := repo.Create(ctx, second)
	assert.ErrorIs(t, err, domain.ErrIdempotencyConflict)
}

func TestTransferRepository_Create_UnknownWallet(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTransferRepository(pool)

	transfer := newPendingTransfer("no-such-wallet-"+uuid.NewString(), "also-missing-"+uuid.NewString(), 100)
	err := repo.Create(context.Background(), transfer)
	assert.ErrorIs(t, err, domain.ErrWalletNotFound)
}

func TestTransferExecutor_Execute_Success(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 500)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	ledger := postgres.NewLedgerRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}

	require.NoError(t, executor.Execute(ctx, transfer, debit, credit))

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 900, fromWallet.Balance)

	toWallet, err := wallets.Get(ctx, toID)
	require.NoError(t, err)
	assert.EqualValues(t, 600, toWallet.Balance)

	entries, err := ledger.ListForWallet(ctx, fromID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.LedgerDebit, entries[0].Type)

	updated, err := transfers.Get(ctx, transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransferProcessed, updated.State)
}

func TestTransferExecutor_Execute_InsufficientFunds(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 10, 0)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	ledger := postgres.NewLedgerRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}

	err := executor.Execute(ctx, transfer, debit, credit)
	assert.ErrorIs(t, err, domain.ErrInsufficientFunds)

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 10, fromWallet.Balance, "balance must be untouched on a failed debit")

	entries, err := ledger.ListForWallet(ctx, fromID)
	require.NoError(t, err)
	assert.Empty(t, entries, "no ledger rows should be written on a failed debit")

	unchanged, err := transfers.Get(ctx, transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransferPending, unchanged.State)
}

// Proves the concurrency claim: fires many concurrent debits at one wallet and checks the final balance.
func TestTransferExecutor_Execute_ConcurrentDebits(t *testing.T) {
	pool := testPool(t)
	const (
		startingBalance = 500
		perTransfer     = 100
		attempts        = 10 // exactly startingBalance/perTransfer should succeed
	)
	fromID, toID := seedWallets(t, pool, startingBalance, 0)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	var wg sync.WaitGroup
	var succeeded, insufficientFunds int64

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			transfer := newPendingTransfer(fromID, toID, perTransfer)
			if err := transfers.Create(ctx, transfer); err != nil {
				t.Errorf("transfers.Create: %v", err)
				return
			}

			debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: perTransfer, CreatedAt: time.Now()}
			credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: perTransfer, CreatedAt: time.Now()}

			switch err := executor.Execute(ctx, transfer, debit, credit); {
			case err == nil:
				atomic.AddInt64(&succeeded, 1)
			case errors.Is(err, domain.ErrInsufficientFunds):
				atomic.AddInt64(&insufficientFunds, 1)
			default:
				t.Errorf("executor.Execute: unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	assert.EqualValues(t, startingBalance/perTransfer, succeeded, "exactly the number the balance can cover should succeed")
	assert.EqualValues(t, attempts-startingBalance/perTransfer, insufficientFunds)

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, fromWallet.Balance, "no lost updates and no overdraft")

	toWallet, err := wallets.Get(ctx, toID)
	require.NoError(t, err)
	assert.EqualValues(t, succeeded*perTransfer, toWallet.Balance)
}

func TestTransferExecutor_Execute_OppositeDirectionDeadlock(t *testing.T) {
	pool := testPool(t)
	const (
		startingBalance = 10000
		perTransfer     = 10
		perSide         = 30
	)
	walletA, walletB := seedWallets(t, pool, startingBalance, startingBalance)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	start := make(chan struct{})
	var wg sync.WaitGroup

	fire := func(from, to string) {
		defer wg.Done()
		transfer := newPendingTransfer(from, to, perTransfer)
		if err := transfers.Create(ctx, transfer); err != nil {
			t.Errorf("transfers.Create: %v", err)
			return
		}
		debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: from, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: perTransfer, CreatedAt: time.Now()}
		credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: to, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: perTransfer, CreatedAt: time.Now()}

		<-start
		if err := executor.Execute(ctx, transfer, debit, credit); err != nil {
			t.Errorf("executor.Execute (from=%s to=%s): %v", from, to, err)
		}
	}

	for i := 0; i < perSide; i++ {
		wg.Add(2)
		go fire(walletA, walletB)
		go fire(walletB, walletA)
	}
	close(start)
	wg.Wait()

	balA, err := wallets.Get(ctx, walletA)
	require.NoError(t, err)
	balB, err := wallets.Get(ctx, walletB)
	require.NoError(t, err)

	assert.EqualValues(t, startingBalance, balA.Balance)
	assert.EqualValues(t, startingBalance, balB.Balance)
}
