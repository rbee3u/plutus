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
	if scope, ok := ctx.Value(transactionKey{}).(transactionScope); ok && scope.db == db {
		return fn(sqlcgen.New(scope.tx))
	}
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

type (
	transactionKey   struct{}
	transactionScope struct {
		db *sql.DB
		tx *sql.Tx
	}
)

// TransactionContext composes application mutations under one explicit owner.
// Only that owner commits or rolls back; a different database cannot join it.
func TransactionContext(ctx context.Context, db *sql.DB, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, transactionKey{}, transactionScope{db: db, tx: tx})
}

// Queries joins the request's transaction when it belongs to this database.
func Queries(ctx context.Context, db *sql.DB) *sqlcgen.Queries {
	return sqlcgen.New(Executor(ctx, db))
}

// Executor shares transaction ownership for bounded bulk statements that sqlc
// cannot express as variable-length VALUES rows.
func Executor(ctx context.Context, db *sql.DB) sqlcgen.DBTX {
	if scope, ok := ctx.Value(transactionKey{}).(transactionScope); ok && scope.db == db {
		return scope.tx
	}
	return db
}

// WithReadContext propagates one consistent read snapshot across collaborators.
func WithReadContext[T any](ctx context.Context, db *sql.DB, fn func(context.Context) (T, error)) (T, error) {
	return withContext(ctx, db, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, fn)
}

// WithWriteContext propagates the transaction to nested application operations.
func WithWriteContext[T any](ctx context.Context, db *sql.DB, fn func(context.Context) (T, error)) (T, error) {
	return withContext(ctx, db, nil, fn)
}

func withContext[T any](ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(context.Context) (T, error)) (T, error) {
	if scope, ok := ctx.Value(transactionKey{}).(transactionScope); ok && scope.db == db {
		return fn(ctx)
	}
	var zero T
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := fn(TransactionContext(ctx, db, tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
