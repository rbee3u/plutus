package retention

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Calendar years follow the Portfolio's local day, clamping February 29 to the
// last day of February. Detail retention independently guarantees 14 full days.
func historyCutoff(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	year, month, day := local.Date()
	year -= 10
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	return time.Date(year, month, min(day, last), 0, 0, 0, 0, loc).UTC()
}

func historyBoundary(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (sqlcgen.GetHistoryBoundaryRow, error) {
	boundary, err := q.GetHistoryBoundary(ctx, sqlcgen.GetHistoryBoundaryParams{PortfolioID: id, Before: cutoff})
	if errors.Is(err, sql.ErrNoRows) {
		return sqlcgen.GetHistoryBoundaryRow{CycleID: []byte{}, EvaluatedAt: cutoff, CreatedAt: cutoff}, nil
	}
	return boundary, err
}

// A page protects the opening snapshot and each Asset's state at that snapshot.
// Other old Points lose their children before their parent. In-memory cursors
// are only an optimization: every deletion can safely be retried after restart.
func (s *Service) pruneHistory(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	pass := s.pass(id, "valuation_points")
	if pass == nil {
		return false, nil
	}
	boundary, err := historyBoundary(ctx, q, id, cutoff)
	if err != nil {
		return false, err
	}
	rows, err := q.ListExpiredPointCandidates(ctx, sqlcgen.ListExpiredPointCandidatesParams{PortfolioID: id, Before: cutoff, AfterAt: pass.after.At, AfterID: pass.after.ID})
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		pass.done = true
		return false, nil
	}
	ids := make([][]byte, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	pins, err := q.ListPinnedHistoricalPoints(ctx, sqlcgen.ListPinnedHistoricalPointsParams{PortfolioID: id, Ids: ids, BoundaryCycleID: boundary.CycleID, BoundaryAt: boundary.EvaluatedAt})
	if err != nil {
		return false, err
	}
	pinned := map[string]bool{}
	for _, pin := range pins {
		pinned[string(pin)] = true
	}
	changed, err := q.MarkHistoricalDetailsPruning(ctx, sqlcgen.MarkHistoricalDetailsPruningParams{PortfolioID: id, Ids: ids, BoundaryCycleID: boundary.CycleID})
	if err != nil {
		return false, err
	}
	doomed := make([][]byte, 0, len(ids))
	for _, candidate := range ids {
		if !pinned[string(candidate)] {
			doomed = append(doomed, candidate)
		}
	}
	if len(doomed) > 0 {
		if _, err := q.DeleteExpiredManualChanges(ctx, sqlcgen.DeleteExpiredManualChangesParams{PortfolioID: id, Ids: doomed}); err != nil {
			return false, err
		}
		if _, err := q.DeleteExpiredCalculatedChanges(ctx, sqlcgen.DeleteExpiredCalculatedChangesParams{PortfolioID: id, Ids: doomed}); err != nil {
			return false, err
		}
		if _, err := q.DeleteExpiredBoundaryEvents(ctx, sqlcgen.DeleteExpiredBoundaryEventsParams{PortfolioID: id, Ids: doomed}); err != nil {
			return false, err
		}
		n, err := q.DeleteEmptyHistoricalPoints(ctx, sqlcgen.DeleteEmptyHistoricalPointsParams{PortfolioID: id, Ids: doomed})
		if err != nil {
			return false, err
		}
		changed += n
	}
	remaining, err := q.ListRemainingHistoricalPoints(ctx, sqlcgen.ListRemainingHistoricalPointsParams{PortfolioID: id, Ids: ids})
	if err != nil {
		return false, err
	}
	blocked := map[string]bool{}
	for _, row := range remaining {
		// The opening snapshot remains complete, including for a dormant Portfolio.
		if bytes.Equal(row.CycleID, boundary.CycleID) {
			continue
		}
		if !pinned[string(row.CycleID)] || row.DetailState != "expired" {
			blocked[string(row.CycleID)] = true
		}
	}
	for _, row := range rows {
		if blocked[string(row.ID)] {
			break
		}
		pass.after = sweepCursor(row)
	}
	if changed > 0 {
		for _, table := range []string{"account_reports", "price_observations", "collection_cycles"} {
			delete(s.sweeps, fmt.Sprintf("%x/%s", id, table))
		}
	}
	return true, nil
}

func pruneHistoricalRecords(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	boundary, err := historyBoundary(ctx, q, id, cutoff)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteExpiredCapitalFlows(ctx, sqlcgen.DeleteExpiredCapitalFlowsParams{PortfolioID: id, Before: boundary.EvaluatedAt})
	if err != nil {
		return false, err
	}
	m, err := q.DeleteExpiredAuditEvents(ctx, sqlcgen.DeleteExpiredAuditEventsParams{PortfolioID: id, Before: cutoff})
	return n+m > 0, err
}

func pruneHistoricalPolicies(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	boundary, err := historyBoundary(ctx, q, id, cutoff)
	if err != nil {
		return false, err
	}
	earlier, err := q.ListEarlierRetainedPolicyBoundaries(ctx, sqlcgen.ListEarlierRetainedPolicyBoundariesParams{PortfolioID: id, Before: boundary.CreatedAt, HistoryBefore: cutoff})
	if err != nil {
		return false, err
	}
	if len(earlier) > 0 {
		boundary.CreatedAt = earlier[0]
	}
	rows, err := q.ListExpiredExclusionVersions(ctx, sqlcgen.ListExpiredExclusionVersionsParams{PortfolioID: id, Before: boundary.CreatedAt})
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if err := q.DeleteExpiredExclusionVersion(ctx, sqlcgen.DeleteExpiredExclusionVersionParams{PortfolioID: id, AssetID: row.AssetID, Version: row.Version}); err != nil {
			return false, err
		}
	}
	return len(rows) > 0, nil
}

func (s *Service) pruneCycles(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	pass := s.pass(id, "collection_cycles")
	if pass == nil {
		return false, nil
	}
	rows, err := q.ListExpiredCycleCandidates(ctx, sqlcgen.ListExpiredCycleCandidatesParams{PortfolioID: id, Before: cutoff, AfterAt: pass.after.At, AfterID: pass.after.ID})
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		pass.done = true
		return false, nil
	}
	ids := make([][]byte, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if _, err := q.DeleteUnreferencedHistoricalCycles(ctx, sqlcgen.DeleteUnreferencedHistoricalCyclesParams{PortfolioID: id, Ids: ids}); err != nil {
		return false, err
	}
	pass.after = sweepCursor(rows[len(rows)-1])
	return true, nil
}

func pruneHistoricalCatalog(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	n, err := q.DeleteUnusedHistoricalContracts(ctx, sqlcgen.DeleteUnusedHistoricalContractsParams{PortfolioID: id, Before: cutoff})
	if err != nil {
		return false, err
	}
	boundary, err := historyBoundary(ctx, q, id, cutoff)
	if err != nil {
		return false, err
	}
	ids, err := q.ListUnusedHistoricalAssets(ctx, sqlcgen.ListUnusedHistoricalAssetsParams{PortfolioID: id, Before: cutoff, StateBefore: boundary.EvaluatedAt})
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return n > 0, nil
	}
	if _, err := q.DeleteUnusedAssetChanges(ctx, sqlcgen.DeleteUnusedAssetChangesParams{PortfolioID: id, Ids: ids}); err != nil {
		return false, err
	}
	_, err = q.DeleteUnusedHistoricalAssets(ctx, sqlcgen.DeleteUnusedHistoricalAssetsParams{PortfolioID: id, Ids: ids})
	return true, err
}
