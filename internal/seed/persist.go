package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// Apply commits source facts and their initial valuation points together.
// Connection activation follows this transaction and can be retried separately.
func Apply(ctx context.Context, db *sql.DB, seed Seed) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := mysql.ExecSQLStatements(ctx, tx, seed.SQL); err != nil {
		return err
	}
	if err := initializePoints(mysql.TransactionContext(ctx, db, tx), db, seed.Portfolios); err != nil {
		return err
	}
	return tx.Commit()
}

func initializePoints(ctx context.Context, db *sql.DB, portfolios []uuid.UUID) error {
	v := valuation.NewService(db)
	for _, portfolioID := range portfolios {
		if err := v.InitializeHistory(ctx, portfolioID); err != nil {
			return fmt.Errorf("initialize seed portfolio %s: %w", portfolioID, err)
		}
	}
	return nil
}
