package seeds

import (
	"context"
	"database/sql"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// Apply inserts source facts atomically. Connection activation and each
// Valuation Point have their own transactions after this source transaction.
func Apply(ctx context.Context, db *sql.DB, seed Seed) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := mysql.ExecSQLStatements(ctx, tx, seed.SQL); err != nil {
		return err
	}
	return tx.Commit()
}
