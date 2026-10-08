package mysql

import (
	"context"
	"database/sql"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// ExecTx runs a write that has no result in a transaction.
func ExecTx(ctx context.Context, db *sql.DB, fn func(q *sqlcgen.Queries) error) error {
	_, err := WithTx(ctx, db, func(q *sqlcgen.Queries) (struct{}, error) {
		return struct{}{}, fn(q)
	})
	return err
}

// WithTx runs fn in a read-write transaction, committing when fn returns nil and
// rolling back on any error or panic.
func WithTx[T any](ctx context.Context, db *sql.DB, fn func(q *sqlcgen.Queries) (T, error)) (T, error) {
	return withTx(ctx, db, nil, fn)
}

// WithReadTx runs fn in a read-only transaction.
func WithReadTx[T any](ctx context.Context, db *sql.DB, fn func(q *sqlcgen.Queries) (T, error)) (T, error) {
	return withTx(ctx, db, &sql.TxOptions{ReadOnly: true}, fn)
}

func withTx[T any](ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(q *sqlcgen.Queries) (T, error)) (T, error) {
	var zero T
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := fn(sqlcgen.New(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
