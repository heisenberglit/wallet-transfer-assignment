package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool tuning is fixed rather than env-configurable — not needed at this scale.
const (
	maxConns        = 10
	maxConnLifetime = time.Hour
	maxConnIdleTime = 30 * time.Minute
)

// Connect opens a pgx connection pool against the given DSN, e.g.
// "postgres://user:pass@localhost:5432/wallet_transfer?sslmode=disable".
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = maxConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime

	return pgxpool.NewWithConfig(ctx, cfg)
}
