package seeds

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Apply commits source facts and their initial Valuation Points together.
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
	q := mysql.Queries(ctx, db)
	v := valuation.NewService(db)
	for _, portfolioID := range portfolios {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return fmt.Errorf("initialize seed Portfolio %s: %w", portfolioID, err)
		}
		cycles, err := q.ListSucceededCycles(ctx, portfolioID[:])
		if err != nil {
			return err
		}
		for _, cycle := range cycles {
			if !cycle.CollectionFinishedAt.Valid {
				return fmt.Errorf("seed cycle %x lacks collection completion time", cycle.ID)
			}
		}
		slices.SortFunc(cycles, func(a, b sqlcgen.ListSucceededCyclesRow) int {
			if c := a.CollectionFinishedAt.Time.Compare(b.CollectionFinishedAt.Time); c != 0 {
				return c
			}
			return bytes.Compare(a.ID, b.ID)
		})
		for _, cycle := range cycles {
			cycleID, err := uuid.FromBytes(cycle.ID)
			if err != nil {
				return err
			}
			if _, err := v.WritePoint(ctx, portfolioID, cycleID, cycle.CollectionFinishedAt.Time); err != nil {
				return fmt.Errorf("initialize seed Valuation Point %s: %w", cycleID, err)
			}
		}
	}
	return nil
}
