package mysql

import (
	"context"
	"database/sql"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Beginner begins a transaction. *sql.DB and *sql.Conn both implement it.
type Beginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// WithTx runs fn in a read-write transaction, committing when fn returns nil and
// rolling back on any error or panic.
func WithTx[T any](ctx context.Context, b Beginner, fn func(tx *sql.Tx, q *sqlcgen.Queries) (T, error)) (T, error) {
	return withTx(ctx, b, nil, fn)
}

// WithReadTx runs fn in a read-only transaction.
func WithReadTx[T any](ctx context.Context, b Beginner, fn func(tx *sql.Tx, q *sqlcgen.Queries) (T, error)) (T, error) {
	return withTx(ctx, b, &sql.TxOptions{ReadOnly: true}, fn)
}

func withTx[T any](ctx context.Context, b Beginner, opts *sql.TxOptions, fn func(tx *sql.Tx, q *sqlcgen.Queries) (T, error)) (T, error) {
	var zero T
	tx, err := b.BeginTx(ctx, opts)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := fn(tx, sqlcgen.New(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
