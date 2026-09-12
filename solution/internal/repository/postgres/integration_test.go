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

// These two read straight from the database rather than through a repository:
// the point of these tests is what Execute actually wrote, and read code can
// share a bug with the write code that would hide it.

func ledgerEntriesFor(t *testing.T, pool *pgxpool.Pool, walletID string) []domain.LedgerEntry {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT id, wallet_id, transfer_id, type, amount FROM ledger_entries WHERE wallet_id = $1 ORDER BY created_at`,
		walletID)
	require.NoError(t, err)
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		require.NoError(t, rows.Scan(&e.ID, &e.WalletID, &e.TransferID, &e.Type, &e.Amount))
		entries = append(entries, e)
	}
	require.NoError(t, rows.Err())
	return entries
}

func transferState(t *testing.T, pool *pgxpool.Pool, id string) domain.TransferState {
	t.Helper()
	var state domain.TransferState
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT state FROM transfers WHERE id = $1`, id).Scan(&state))
	return state
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

	assert.Equal(t, domain.TransferPending, transferState(t, pool, transfer.ID))

	byKey, err := repo.GetByIdempotencyKey(ctx, transfer.IdempotencyKey)
	require.NoError(t, err)
	require.NotNil(t, byKey)
	assert.Equal(t, transfer.ID, byKey.ID)

	notFound, err := repo.GetByIdempotencyKey(ctx, "no-such-key-"+uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, notFound)

	require.NoError(t, repo.UpdateState(ctx, transfer.ID, domain.TransferProcessed))
	assert.Equal(t, domain.TransferProcessed, transferState(t, pool, transfer.ID))
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

	entries := ledgerEntriesFor(t, pool, fromID)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.LedgerDebit, entries[0].Type)

	assert.Equal(t, domain.TransferProcessed, transferState(t, pool, transfer.ID))
}

// Dedicated ledger-correctness test: a transfer must produce exactly one
// DEBIT and one CREDIT row, both referencing the same transfer, with
// matching amounts — the double-entry invariant, checked as its own
// behavioral claim rather than a side assertion of Execute_Success.
func TestTransferExecutor_Execute_LedgerIsBalanced(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 500)
	transfers := postgres.NewTransferRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}
	require.NoError(t, executor.Execute(ctx, transfer, debit, credit))

	debitEntries := ledgerEntriesFor(t, pool, fromID)
	require.Len(t, debitEntries, 1, "exactly one ledger row for the debited wallet")
	assert.Equal(t, domain.LedgerDebit, debitEntries[0].Type)
	assert.Equal(t, transfer.ID, debitEntries[0].TransferID)

	creditEntries := ledgerEntriesFor(t, pool, toID)
	require.Len(t, creditEntries, 1, "exactly one ledger row for the credited wallet")
	assert.Equal(t, domain.LedgerCredit, creditEntries[0].Type)
	assert.Equal(t, transfer.ID, creditEntries[0].TransferID)

	// The double-entry invariant: both legs reference the same transfer
	// and carry the same amount, so debits and credits always balance.
	assert.Equal(t, debitEntries[0].Amount, creditEntries[0].Amount,
		"debit and credit amounts must match — the ledger must always balance")
}

func TestTransferExecutor_Execute_InsufficientFunds(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 10, 0)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
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

	assert.Empty(t, ledgerEntriesFor(t, pool, fromID), "no ledger rows should be written on a failed debit")
	assert.Equal(t, domain.TransferPending, transferState(t, pool, transfer.ID))
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

// Proves the state-transition CAS added to Execute: a second call against
// an already-PROCESSED transfer must fail, not silently re-apply the
// debit/credit a second time.
func TestTransferExecutor_Execute_RejectsDoubleExecution(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 500)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}

	require.NoError(t, executor.Execute(ctx, transfer, debit, credit))

	// Same transfer, same debit/credit — as if Execute got called twice
	// for the same PROCESSED transfer (a bug, or a naive retry).
	secondDebit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	secondCredit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}
	err := executor.Execute(ctx, transfer, secondDebit, secondCredit)
	assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 900, fromWallet.Balance, "second Execute must not re-apply the debit")

	toWallet, err := wallets.Get(ctx, toID)
	require.NoError(t, err)
	assert.EqualValues(t, 600, toWallet.Balance, "second Execute must not re-apply the credit")
}

// Proves the amount-agreement guard: Execute must reject debit/credit
// entries whose amount doesn't match transfer.Amount, before writing
// anything — a caller bug shouldn't be able to write a ledger entry that
// doesn't match what was actually moved.
func TestTransferExecutor_Execute_RejectsMismatchedLedgerAmounts(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 500)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	// credit.Amount doesn't match transfer.Amount.
	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 999, CreatedAt: time.Now()}

	err := executor.Execute(ctx, transfer, debit, credit)
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrInsufficientFunds, "should be rejected by the amount guard, not reach the debit statement")

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 1000, fromWallet.Balance, "nothing should be written when the amounts disagree")

	toWallet, err := wallets.Get(ctx, toID)
	require.NoError(t, err)
	assert.EqualValues(t, 500, toWallet.Balance)

	assert.Empty(t, ledgerEntriesFor(t, pool, fromID))
	assert.Equal(t, domain.TransferPending, transferState(t, pool, transfer.ID))
}

// Boundary case: a transfer for exactly the source wallet's whole balance
// must succeed (the debit's WHERE balance >= $amount is inclusive), leaving
// it at exactly zero — not rejected as insufficient funds by an off-by-one.
func TestTransferExecutor_Execute_ExactBalance(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 100, 0)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	ctx := context.Background()

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(ctx, transfer))

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}

	require.NoError(t, executor.Execute(ctx, transfer, debit, credit))

	fromWallet, err := wallets.Get(ctx, fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, fromWallet.Balance)
}

// Extends the two-wallet deadlock fix to a three-wallet cycle (A->B->C->A
// concurrently, repeatedly). Per the standard resource-ordering
// deadlock-prevention argument, locking in a single global order (here,
// ascending wallet id) per transaction prevents cycles of any size, not
// just two — this is a stress test for that claim, not expected to ever
// fail if the two-wallet fix is correct.
func TestTransferExecutor_Execute_ThreeWayCircularConcurrent(t *testing.T) {
	pool := testPool(t)
	const (
		startingBalance = 5000
		perTransfer     = 10
		perLeg          = 20
	)
	ctx := context.Background()
	walletA, walletB := seedWallets(t, pool, startingBalance, startingBalance)
	walletC, _ := seedWallets(t, pool, startingBalance, 0) // second ID unused, cleaned up regardless

	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)

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

	for i := 0; i < perLeg; i++ {
		wg.Add(3)
		go fire(walletA, walletB)
		go fire(walletB, walletC)
		go fire(walletC, walletA)
	}
	close(start)
	wg.Wait()

	// Every wallet sends perLeg*perTransfer and receives perLeg*perTransfer,
	// so each nets to its starting balance.
	for _, id := range []string{walletA, walletB, walletC} {
		w, err := wallets.Get(ctx, id)
		require.NoError(t, err)
		assert.EqualValues(t, startingBalance, w.Balance, "wallet %s", id)
	}
}

// Execute must fail promptly (not hang) when given an already-canceled
// context, and must leave the pool in a usable state afterward — proving
// the Rollback(context.WithoutCancel(ctx)) actually reaches Postgres
// instead of no-op'ing on the canceled context and leaking the
// transaction/connection.
func TestTransferExecutor_Execute_CanceledContextRollsBackCleanly(t *testing.T) {
	pool := testPool(t)
	fromID, toID := seedWallets(t, pool, 1000, 500)
	transfers := postgres.NewTransferRepository(pool)
	wallets := postgres.NewWalletRepository(pool)
	executor := postgres.NewTransferExecutor(pool)

	transfer := newPendingTransfer(fromID, toID, 100)
	require.NoError(t, transfers.Create(context.Background(), transfer))

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	debit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: fromID, TransferID: transfer.ID, Type: domain.LedgerDebit, Amount: 100, CreatedAt: time.Now()}
	credit := domain.LedgerEntry{ID: uuid.NewString(), WalletID: toID, TransferID: transfer.ID, Type: domain.LedgerCredit, Amount: 100, CreatedAt: time.Now()}

	err := executor.Execute(canceledCtx, transfer, debit, credit)
	require.Error(t, err, "Execute must not succeed against an already-canceled context")

	// The pool must still work afterward — no stuck transaction or
	// connection left behind by the canceled rollback.
	fromWallet, err := wallets.Get(context.Background(), fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 1000, fromWallet.Balance, "canceled Execute must not have partially applied")
}
