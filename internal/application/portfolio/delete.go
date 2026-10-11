package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// deleteCascade requests deletion; physical cleanup is bounded worker work.
// Ending the active cycle keeps the queue from claiming it again; collection
// writes already refuse a deleting Portfolio.
func (s *Service) deleteCascade(ctx context.Context, userID, portfolioID []byte, version uint64) error {
	_, err := mysql.WithWriteContext(ctx, s.db, func(ctx context.Context) (struct{}, error) {
		q := mysql.Queries(ctx, s.db)
		now := s.now().UTC()
		n, err := q.RequestPortfolioDeletion(ctx, sqlcgen.RequestPortfolioDeletionParams{
			ID: portfolioID, UserID: userID, Version: version, RequestedAt: sql.NullTime{Time: now, Valid: true}, At: now,
		})
		if err != nil {
			return struct{}{}, err
		}
		if n != 1 {
			return struct{}{}, ErrPrecondition
		}
		_, err = q.FailActiveCollectionCycle(ctx, sqlcgen.FailActiveCollectionCycleParams{
			PortfolioID: portfolioID, FinishedAt: sql.NullTime{Time: now, Valid: true},
			ErrorClass:  sql.NullString{String: "portfolio_deleted", Valid: true},
			ErrorDetail: sql.NullString{String: "The Portfolio was deleted before the cycle finished.", Valid: true},
		})
		return struct{}{}, err
	})
	return err
}

// DeleteBatch removes at most one bounded set of rows owned by the oldest
// deleting Portfolio; the final batch removes the Portfolio itself. worked is
// false when no deletion is pending or another worker holds it.
func (s *Service) DeleteBatch(ctx context.Context) (worked bool, err error) {
	return mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) (bool, error) {
		portfolioID, err := q.GetNextDeletingPortfolioID(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		row, err := q.LockDeletingPortfolio(ctx, portfolioID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		steps := []struct {
			table  string
			delete func(context.Context, []byte) (int64, error)
		}{
			{"manual_balance_changes", q.DeletePortfolioManualBalanceChanges},
			{"calculated_asset_changes", q.DeletePortfolioCalculatedAssetChanges},
			{"boundary_events", q.DeletePortfolioBoundaryEvents},
			{"balance_valuations", q.DeletePortfolioBalanceValuations},
			{"account_valuations", q.DeletePortfolioAccountValuations},
			{"balances", q.DeletePortfolioBalances},
			{"futures_positions", q.DeletePortfolioPositions},
			{"earn_positions", q.DeletePortfolioEarnPositions},
			{"dual_positions", q.DeletePortfolioDualPositions},
			{"capital_flows", q.DeletePortfolioCapitalFlows},
			{"valuation_points", q.DeletePortfolioValuationPoints},
			{"audit_events", q.DeletePortfolioAuditEvents},
			{"asset_exclusion_versions", q.DeletePortfolioAssetExclusionVersions},
			{"account_reports", q.DeletePortfolioAccountReports},
			{"manual_revision_balances", q.DeletePortfolioManualRevisionBalances},
			{"manual_revisions", q.DeletePortfolioManualRevisions},
			{"futures_contracts", q.DeletePortfolioFuturesContracts},
			{"price_observations", q.DeletePortfolioPriceObservations},
			{"assets", q.DeletePortfolioAssets},
			{"accounts", q.DeletePortfolioAccounts},
			{"collection_cycles", q.DeletePortfolioCollectionCycles},
			{"identities", q.DeletePortfolioIdentities},
		}
		for _, step := range steps {
			count, err := step.delete(ctx, row.ID)
			if err != nil {
				return false, fmt.Errorf("delete from %s: %w", step.table, err)
			}
			if count > 0 {
				return true, nil
			}
		}
		if _, err := q.DeletePortfolio(ctx, sqlcgen.DeletePortfolioParams{ID: row.ID, UserID: row.UserID, Version: row.Version}); err != nil {
			return false, err
		}
		return true, nil
	})
}
