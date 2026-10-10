// Package retention keeps immutable summaries and a bounded archive of detail.
package retention

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const detailAge = 30 * 24 * time.Hour

var sweepStart = time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC)

// Service performs one small, resumable maintenance unit per idle worker tick.
// Cursors for archive decisions live in the database; disposable sweep cursors
// restart safely after a process restart. Only one worker calls Step at a time.
type Service struct {
	db        *sql.DB
	portfolio []byte
	phase     int
	sweeps    map[string]sweepCursor
}

// sweepCursor matches the generated candidate rows so they convert directly.
type sweepCursor struct {
	ID []byte
	At time.Time
}

func NewService(db *sql.DB) *Service { return &Service{db: db, sweeps: map[string]sweepCursor{}} }

// Step never loops until the backlog is drained. Collection gets another chance
// to run between units, including while years of old details are being pruned.
func (s *Service) Step(ctx context.Context, now time.Time) error {
	id, err := sqlcgen.New(s.db).GetNextRetentionPortfolioID(ctx, nonNil(s.portfolio))
	if errors.Is(err, sql.ErrNoRows) {
		s.portfolio = nil
		s.phase = (s.phase + 1) % 6
		return nil
	}
	if err != nil {
		return err
	}
	s.portfolio = id
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := sqlcgen.New(tx)
	p, err := q.LockRetentionPortfolio(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// An unfinished cycle can still adopt old prices and reports. Its evidence is
	// protected until the cycle reaches a terminal state.
	active, err := q.HasActiveCollectionCycle(ctx, id)
	if err != nil || active {
		return err
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return err
	}
	local := now.Add(-detailAge).In(loc)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).UTC()
	switch s.phase {
	case 0:
		err = archiveDetails(ctx, q, id, cutoff, loc)
	case 1:
		err = prunePoint(ctx, q, id)
	case 2:
		err = s.pruneReport(ctx, q, id, cutoff)
	case 3:
		err = anchorPrices(ctx, q, id, cutoff, loc, p.PriceAnchorsBefore)
	case 4:
		err = s.prunePrices(ctx, q, id, cutoff, p.PriceAnchorsBefore)
	case 5:
		err = pruneManual(ctx, q, id, cutoff)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func archiveDetails(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time, loc *time.Location) error {
	oldest, err := q.GetOldestFullPointBefore(ctx, sqlcgen.GetOldestFullPointBeforeParams{PortfolioID: id, Before: cutoff})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	local := oldest.In(loc)
	end := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc).UTC()
	if end.After(cutoff) {
		return nil
	}
	// Only full points are candidates; an existing daily anchor is permanent.
	anchor, err := q.GetLatestFullPointBefore(ctx, sqlcgen.GetLatestFullPointBeforeParams{PortfolioID: id, Before: end})
	if err != nil {
		return err
	}
	if err := q.SetPointDetailState(ctx, sqlcgen.SetPointDetailStateParams{DetailState: "daily", PortfolioID: id, CycleID: anchor}); err != nil {
		return err
	}
	return q.MarkFullPointsPruningBefore(ctx, sqlcgen.MarkFullPointsPruningBeforeParams{PortfolioID: id, Before: end})
}

func prunePoint(ctx context.Context, q *sqlcgen.Queries, id []byte) error {
	cycle, err := q.GetNextPruningPoint(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Readers hide pruning details before any children are removed. REPEATABLE
	// READ keeps an already-open read on the complete pre-cleanup snapshot.
	n, err := q.PrunePointBalanceValuations(ctx, sqlcgen.PrunePointBalanceValuationsParams{PortfolioID: id, CycleID: cycle})
	if err != nil || n > 0 {
		return err
	}
	n, err = q.PrunePointAccountValuations(ctx, sqlcgen.PrunePointAccountValuationsParams{PortfolioID: id, CycleID: cycle})
	if err != nil || n > 0 {
		return err
	}
	return q.SetPointDetailState(ctx, sqlcgen.SetPointDetailStateParams{DetailState: "expired", PortfolioID: id, CycleID: cycle})
}

// after returns the sweep position; an exhausted sweep restarts from the oldest
// row. Candidate pages bound traversal even when nearly all rows are pinned.
func (s *Service) after(id []byte, table string) (string, sweepCursor) {
	key := fmt.Sprintf("%x/%s", id, table)
	cursor, ok := s.sweeps[key]
	if !ok {
		cursor = sweepCursor{ID: []byte{}, At: sweepStart}
	}
	return key, cursor
}

func (s *Service) pruneReport(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) error {
	key, after := s.after(id, "account_reports")
	rows, err := q.ListReportRetentionCandidates(ctx, sqlcgen.ListReportRetentionCandidatesParams{PortfolioID: id, Before: cutoff, AfterAt: after.At, AfterID: after.ID})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		delete(s.sweeps, key)
	}
	for _, row := range rows {
		report := sweepCursor(row)
		pinned, err := q.IsReportPinned(ctx, sqlcgen.IsReportPinnedParams{PortfolioID: id, ID: report.ID})
		if err != nil {
			return err
		}
		if pinned.Bool {
			s.sweeps[key] = report
			continue
		}
		n, err := q.PruneReportBalances(ctx, sqlcgen.PruneReportBalancesParams{PortfolioID: id, ReportID: report.ID})
		if err != nil || n > 0 {
			return err
		}
		n, err = q.PruneReportFuturesPositions(ctx, sqlcgen.PruneReportFuturesPositionsParams{PortfolioID: id, ReportID: report.ID})
		if err != nil || n > 0 {
			return err
		}
		n, err = q.PruneReportEarnPositions(ctx, sqlcgen.PruneReportEarnPositionsParams{PortfolioID: id, ReportID: report.ID})
		if err != nil || n > 0 {
			return err
		}
		n, err = q.PruneReportDualPositions(ctx, sqlcgen.PruneReportDualPositionsParams{PortfolioID: id, ReportID: report.ID})
		if err != nil || n > 0 {
			return err
		}
		if err := q.DeleteAccountReport(ctx, sqlcgen.DeleteAccountReportParams{PortfolioID: id, ID: report.ID}); err != nil {
			return err
		}
		s.sweeps[key] = report
		return nil
	}
	return nil
}

func anchorPrices(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time, loc *time.Location, anchored sql.NullTime) error {
	from := time.Time{}
	if anchored.Valid {
		from = anchored.Time
	}
	oldest, err := q.GetOldestPriceRetrievedBetween(ctx, sqlcgen.GetOldestPriceRetrievedBetweenParams{PortfolioID: id, FromAt: from, Before: cutoff})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	local := oldest.In(loc)
	end := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc).UTC()
	if end.After(cutoff) {
		return nil
	}
	if err := q.MarkDailyPriceAnchors(ctx, sqlcgen.MarkDailyPriceAnchorsParams{PortfolioID: id, FromAt: from, Before: end}); err != nil {
		return err
	}
	return q.SetPriceAnchorsBefore(ctx, sqlcgen.SetPriceAnchorsBeforeParams{Before: sql.NullTime{Time: end, Valid: true}, ID: id})
}

func (s *Service) prunePrices(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time, anchored sql.NullTime) error {
	if !anchored.Valid {
		return nil
	}
	if anchored.Time.Before(cutoff) {
		cutoff = anchored.Time
	}
	key, after := s.after(id, "price_observations")
	rows, err := q.ListPriceRetentionCandidates(ctx, sqlcgen.ListPriceRetentionCandidatesParams{PortfolioID: id, Before: cutoff, AfterAt: after.At, AfterID: after.ID})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		delete(s.sweeps, key)
	}
	for _, row := range rows {
		price := sweepCursor(row)
		if err := q.DeleteUnpinnedPriceObservation(ctx, sqlcgen.DeleteUnpinnedPriceObservationParams{PortfolioID: id, ID: price.ID}); err != nil {
			return err
		}
		s.sweeps[key] = price
	}
	return nil
}

func pruneManual(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) error {
	revision, err := q.GetPrunableManualRevision(ctx, sqlcgen.GetPrunableManualRevisionParams{PortfolioID: id, Before: sql.NullTime{Time: cutoff, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	n, err := q.PruneManualRevisionBalances(ctx, sqlcgen.PruneManualRevisionBalancesParams{PortfolioID: id, AccountID: revision.AccountID, RevisionNumber: revision.RevisionNumber})
	if err != nil || n > 0 {
		return err
	}
	return q.DeleteManualRevision(ctx, sqlcgen.DeleteManualRevisionParams{PortfolioID: id, AccountID: revision.AccountID, RevisionNumber: revision.RevisionNumber})
}

func nonNil(id []byte) []byte {
	if id == nil {
		return []byte{}
	}
	return id
}
