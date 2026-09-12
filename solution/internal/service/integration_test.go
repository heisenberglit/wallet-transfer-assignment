package service_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/heisenberglit/wallet-transfer-assignment/internal/db"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/repository/postgres"
	"github.com/heisenberglit/wallet-transfer-assignment/internal/service"
)

// TestCreateTransfer_ConcurrentSameIdempotencyKey exercises the real
// idempotency race against a real Postgres, not fakes: many concurrent
// CreateTransfer calls with the same idempotencyKey must all resolve to
// exactly one PROCESSED transfer, with exactly one debit/credit applied —
// the unit tests in transfer_service_test.go assert this behavior against
// fakes standing in for both sides of the race, which proves the service's
// logic is correct but can't prove the database actually enforces it.
func TestCreateTransfer_ConcurrentSameIdempotencyKey(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	fromID := "test_" + uuid.NewString()
	toID := "test_" + uuid.NewString()
	_, err = pool.Exec(context.Background(), `INSERT INTO wallets (id, balance) VALUES ($1, 1000), ($2, 0)`, fromID, toID)
	require.NoError(t, err)
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM ledger_entries WHERE wallet_id IN ($1, $2)`, fromID, toID)                             //nolint:errcheck
		pool.Exec(bg, `DELETE FROM transfers WHERE from_wallet_id IN ($1, $2) OR to_wallet_id IN ($1, $2)`, fromID, toID) //nolint:errcheck
		pool.Exec(bg, `DELETE FROM wallets WHERE id IN ($1, $2)`, fromID, toID)                                           //nolint:errcheck
	})

	wallets := postgres.NewWalletRepository(pool)
	transfers := postgres.NewTransferRepository(pool)
	executor := postgres.NewTransferExecutor(pool)
	svc := service.NewTransferService(wallets, transfers, executor)

	const concurrency = 20
	idempotencyKey := "race-" + uuid.NewString()

	start := make(chan struct{})
	results := make([]struct {
		id  string
		err error
	}, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			transfer, err := svc.CreateTransfer(context.Background(), service.CreateTransferInput{
				IdempotencyKey: idempotencyKey,
				FromWalletID:   fromID,
				ToWalletID:     toID,
				Amount:         100,
			})
			results[i].err = err
			if transfer != nil {
				results[i].id = transfer.ID
			}
		}(i)
	}
	close(start)
	wg.Wait()

	firstID := ""
	for i, r := range results {
		require.NoErrorf(t, r.err, "call %d", i)
		if firstID == "" {
			firstID = r.id
		}
		assert.Equalf(t, firstID, r.id, "call %d returned a different transfer than the others", i)
	}

	fromWallet, err := wallets.Get(context.Background(), fromID)
	require.NoError(t, err)
	assert.EqualValues(t, 900, fromWallet.Balance, "the transfer must have been applied exactly once")

	toWallet, err := wallets.Get(context.Background(), toID)
	require.NoError(t, err)
	assert.EqualValues(t, 100, toWallet.Balance)
}
