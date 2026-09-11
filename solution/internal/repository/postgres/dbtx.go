package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the subset of *pgxpool.Pool's query methods each repository
// needs. Both *pgxpool.Pool and pgx.Tx satisfy it, so a repository built
// with a bare pool can later be handed a transaction (e.g. once
// UnitOfWork starts threading a pgx.Tx through ctx) without changing its
// signature.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Beginner is the subset of *pgxpool.Pool that UnitOfWork needs to start
// a transaction. pgx.Tx does not implement this (no nested transactions),
// which is intentional — only the pool can begin a top-level transaction.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
