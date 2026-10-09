package portfolio

import (
	"context"
	"fmt"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func (s *Service) deleteCascade(ctx context.Context, userID, portfolioID []byte, version uint64) error {
	return mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		// Source references are released only in this transaction. No modified report
		// survives a successful delete, and any failure restores all original data.
		if err := q.ReleasePortfolioReportSources(ctx, portfolioID); err != nil {
			return err
		}
		// Keep lifetime ownership explicit and foreign-key checks enabled. New owned
		// tables must be placed before their parents in this deletion order.
		steps := []struct {
			table  string
			delete func(context.Context, []byte) error
		}{
			{"balances", q.DeletePortfolioBalances},
			{"positions", q.DeletePortfolioPositions},
			{"earn_positions", q.DeletePortfolioEarnPositions},
			{"dual_positions", q.DeletePortfolioDualPositions},
			{"manual_cycle_accounts", q.DeletePortfolioManualCycleAccounts},
			{"manual_balances", q.DeletePortfolioManualBalances},
			{"manual_revisions", q.DeletePortfolioManualRevisions},
			{"capital_flows", q.DeletePortfolioCapitalFlows},
			{"valuation_points", q.DeletePortfolioValuationPoints},
			{"audit_events", q.DeletePortfolioAuditEvents},
			{"idempotency_records", q.DeletePortfolioIdempotencyRecords},
			{"portfolio_platform_asset_exclusion_events", q.DeletePortfolioPlatformAssetExclusionEvents},
			{"portfolio_platform_asset_exclusions", q.DeletePortfolioPlatformAssetExclusions},
			{"account_reports", q.DeletePortfolioAccountReports},
			{"manual_cycles", q.DeletePortfolioManualCycles},
			{"accounts", q.DeletePortfolioAccounts},
			{"jobs", q.DeletePortfolioJobs},
			{"identities", q.DeletePortfolioIdentities},
		}
		for _, step := range steps {
			if err := step.delete(ctx, portfolioID); err != nil {
				return fmt.Errorf("delete from %s: %w", step.table, err)
			}
		}
		n, err := q.DeletePortfolio(ctx, sqlcgen.DeletePortfolioParams{ID: portfolioID, UserID: userID, Version: version})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		return nil
	})
}
