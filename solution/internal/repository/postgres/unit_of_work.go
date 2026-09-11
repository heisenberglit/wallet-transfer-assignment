package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UnitOfWork implements repository.UnitOfWork using a pgx transaction.
//
// TODO: pick and document an isolation level (READ COMMITTED is pgx's
// default; consider SERIALIZABLE or explicit row locks per
// docs/design.md's concurrency strategy) and wire repositories to use the
// tx from ctx when one is present instead of the bare pool.
type UnitOfWork struct {
	pool *pgxpool.Pool
}

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

func (u *UnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	// TODO: stash tx in ctx (e.g. via a private context key) so repository
	// methods pick it up instead of using u.pool directly.
	if err := fn(ctx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
