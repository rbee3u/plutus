// Package retention keeps immutable summaries and a bounded archive of detail.
package retention

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	detailAge  = 14 * 24 * time.Hour
	phaseCount = 11
)

var sweepStart = time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC)

// Service performs one small, resumable maintenance unit per idle worker tick.
// Cursors for archive decisions live in the database; disposable sweep cursors
// restart safely after a process restart. Only one worker calls Step at a time.
type Service struct {
	db        *sql.DB
	portfolio []byte
	phase     int
	busy      bool
	epochs    map[string]epoch
	sweeps    map[string]*sweep
}

// epoch advances whenever pinned candidates may have been released: a new
// cutoff day, archived or expired detail, newly anchored prices, or a failed
// unit whose sweep cursor moved past rolled-back work. Releases caused by
// collection, such as a newer last successful report, wait for the next cutoff
// day.
type epoch struct {
	cutoff time.Time
	n      uint64
}

// sweep is one pass over candidate rows. A finished pass restarts only in a
// later epoch, so permanently pinned history is not rescanned while nothing
// could have released it.
type sweep struct {
	after sweepCursor
	epoch uint64
	done  bool
}

// sweepCursor matches the generated candidate rows so they convert directly.
type sweepCursor struct {
	ID []byte
	At time.Time
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, epochs: map[string]epoch{}, sweeps: map[string]*sweep{}}
}

// Step never loops until the backlog is drained. Collection gets another chance
// to run between units, including while years of old details are being pruned.
// It reports idle once a full round of every phase over every Portfolio changed
// and scanned nothing.
func (s *Service) Step(ctx context.Context, now time.Time) (bool, error) {
	id, err := sqlcgen.New(s.db).GetNextRetentionPortfolioID(ctx, nonNil(s.portfolio))
	if errors.Is(err, sql.ErrNoRows) {
		s.portfolio = nil
		s.phase = (s.phase + 1) % phaseCount
		if s.phase != 0 {
			return false, nil
		}
		idle := !s.busy
		s.busy = false
		return idle, nil
	}
	if err != nil {
		return false, err
	}
	s.portfolio = id
	worked, err := s.step(ctx, id, now)
	if err != nil {
		s.advance(id)
		// Cursor movement is not committed when the transaction fails.
		prefix := fmt.Sprintf("%x/", id)
		for key := range s.sweeps {
			if strings.HasPrefix(key, prefix) {
				delete(s.sweeps, key)
			}
		}
		return false, err
	}
	s.busy = s.busy || worked
	return false, nil
}

func (s *Service) step(ctx context.Context, id []byte, now time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := sqlcgen.New(tx)
	p, err := q.LockRetentionPortfolio(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// An unfinished cycle can still adopt old prices and reports. Its evidence is
	// protected until the cycle reaches a terminal state.
	active, err := q.HasActiveCollectionCycle(ctx, id)
	if err != nil || active {
		return false, err
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return false, err
	}
	local := now.Add(-detailAge).In(loc)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).UTC()
	if e := s.epochs[string(id)]; !e.cutoff.Equal(cutoff) {
		s.epochs[string(id)] = epoch{cutoff: cutoff, n: e.n + 1}
	}
	historyBefore := historyCutoff(now, loc)
	if p.HistoryBefore.Valid && p.HistoryBefore.Time.After(historyBefore) {
		historyBefore = p.HistoryBefore.Time
	}
	if !p.HistoryBefore.Valid || p.HistoryBefore.Time.Before(historyBefore) {
		if err := q.AdvanceHistoryBoundary(ctx, sqlcgen.AdvanceHistoryBoundaryParams{ID: id, Before: sql.NullTime{Time: historyBefore, Valid: true}}); err != nil {
			return false, err
		}
	}
	var worked bool
	switch s.phase {
	case 0:
		worked, err = archiveDetails(ctx, q, id, cutoff, loc)
	case 1:
		worked, err = prunePoint(ctx, q, id)
	case 2:
		worked, err = s.pruneReport(ctx, q, id, cutoff)
	case 3:
		worked, err = anchorPrices(ctx, q, id, cutoff, loc, p.PriceAnchorsBefore)
	case 4:
		worked, err = s.prunePrices(ctx, q, id, cutoff, historyBefore, p.PriceAnchorsBefore)
	case 5:
		worked, err = s.pruneManual(ctx, q, id, cutoff)
	case 6:
		worked, err = s.pruneHistory(ctx, q, id, historyBefore)
	case 7:
		worked, err = pruneHistoricalRecords(ctx, q, id, historyBefore)
	case 8:
		worked, err = pruneHistoricalPolicies(ctx, q, id, historyBefore)
	case 9:
		worked, err = s.pruneCycles(ctx, q, id, historyBefore)
	case 10:
		worked, err = pruneHistoricalCatalog(ctx, q, id, historyBefore)
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if worked && (s.phase == 0 || s.phase == 1 || s.phase == 3 || s.phase == 7 || s.phase == 10) {
		s.advance(id)
	}
	return worked, nil
}

func (s *Service) advance(id []byte) {
	e := s.epochs[string(id)]
	e.n++
	s.epochs[string(id)] = e
}

func archiveDetails(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time, loc *time.Location) (bool, error) {
	oldest, err := q.GetOldestFullPointBefore(ctx, sqlcgen.GetOldestFullPointBeforeParams{PortfolioID: id, Before: cutoff})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	local := oldest.In(loc)
	end := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc).UTC()
	if end.After(cutoff) {
		return false, nil
	}
	// Only full points are candidates; timezone edits preserve existing daily anchors.
	anchor, err := q.GetLatestFullPointBefore(ctx, sqlcgen.GetLatestFullPointBeforeParams{PortfolioID: id, Before: end})
	if err != nil {
		return false, err
	}
	if err := q.SetPointDetailState(ctx, sqlcgen.SetPointDetailStateParams{DetailState: "daily", PortfolioID: id, CycleID: anchor}); err != nil {
		return false, err
	}
	return true, q.MarkFullPointsPruningBefore(ctx, sqlcgen.MarkFullPointsPruningBeforeParams{PortfolioID: id, Before: end})
}

func prunePoint(ctx context.Context, q *sqlcgen.Queries, id []byte) (bool, error) {
	cycle, err := q.GetNextPruningPoint(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Readers hide pruning details before any children are removed. REPEATABLE
	// READ keeps an already-open read on the complete pre-cleanup snapshot.
	if _, err := q.PrunePointBalanceValuations(ctx, sqlcgen.PrunePointBalanceValuationsParams{PortfolioID: id, CycleID: cycle}); err != nil {
		return false, err
	}
	if _, err := q.PrunePointAccountValuations(ctx, sqlcgen.PrunePointAccountValuationsParams{PortfolioID: id, CycleID: cycle}); err != nil {
		return false, err
	}
	return true, q.ExpireEmptyPoint(ctx, sqlcgen.ExpireEmptyPointParams{PortfolioID: id, CycleID: cycle})
}

// pass returns the unfinished sweep for a table, starting a new pass from the
// oldest row in a later epoch, or nil when this epoch's pass already finished.
// Candidate pages bound traversal even when nearly all rows are pinned.
func (s *Service) pass(id []byte, table string) *sweep {
	key := fmt.Sprintf("%x/%s", id, table)
	current := s.epochs[string(id)].n
	pass := s.sweeps[key]
	if pass == nil || (pass.done && pass.epoch != current) {
		pass = &sweep{after: sweepCursor{ID: []byte{}, At: sweepStart}, epoch: current}
		s.sweeps[key] = pass
	}
	if pass.done {
		return nil
	}
	return pass
}

func (s *Service) pruneReport(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	pass := s.pass(id, "account_reports")
	if pass == nil {
		return false, nil
	}
	rows, err := q.ListReportRetentionCandidates(ctx, sqlcgen.ListReportRetentionCandidatesParams{PortfolioID: id, Before: cutoff, AfterAt: pass.after.At, AfterID: pass.after.ID})
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
	pinnedIDs, err := q.ListPinnedReportIDs(ctx, sqlcgen.ListPinnedReportIDsParams{PortfolioID: id, Ids: ids})
	if err != nil {
		return false, err
	}
	pinned := make(map[string]bool, len(pinnedIDs))
	for _, report := range pinnedIDs {
		pinned[string(report)] = true
	}
	ids = ids[:0]
	for _, row := range rows {
		if !pinned[string(row.ID)] {
			ids = append(ids, row.ID)
		}
	}
	remaining := map[string]bool{}
	if len(ids) > 0 {
		// A visit deletes at most 1,000 rows from each child table and 64
		// parents. Small reports share this budget instead of each requiring
		// several complete worker rounds; oversized reports resume in place.
		if _, err := q.PruneReportBalances(ctx, sqlcgen.PruneReportBalancesParams{PortfolioID: id, Ids: ids}); err != nil {
			return false, err
		}
		if _, err := q.PruneReportFuturesPositions(ctx, sqlcgen.PruneReportFuturesPositionsParams{PortfolioID: id, Ids: ids}); err != nil {
			return false, err
		}
		if _, err := q.PruneReportEarnPositions(ctx, sqlcgen.PruneReportEarnPositionsParams{PortfolioID: id, Ids: ids}); err != nil {
			return false, err
		}
		if _, err := q.PruneReportDualPositions(ctx, sqlcgen.PruneReportDualPositionsParams{PortfolioID: id, Ids: ids}); err != nil {
			return false, err
		}
		if err := q.DeleteEmptyAccountReports(ctx, sqlcgen.DeleteEmptyAccountReportsParams{PortfolioID: id, Ids: ids}); err != nil {
			return false, err
		}
		left, err := q.ListRemainingReportIDs(ctx, sqlcgen.ListRemainingReportIDsParams{PortfolioID: id, Ids: ids})
		if err != nil {
			return false, err
		}
		for _, report := range left {
			remaining[string(report)] = true
		}
		if len(remaining) < len(ids) {
			// Deleted Manual reports can release revisions after their sweep
			// already found nothing eligible in this epoch.
			delete(s.sweeps, fmt.Sprintf("%x/manual_revisions", id))
			delete(s.sweeps, fmt.Sprintf("%x/collection_cycles", id))
		}
	}
	// Never advance past an unfinished report, even if later small reports
	// were removed in the same visit. Pinned rows can be skipped this epoch.
	for _, row := range rows {
		if remaining[string(row.ID)] {
			break
		}
		pass.after = sweepCursor(row)
	}
	return true, nil
}

func anchorPrices(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time, loc *time.Location, anchored sql.NullTime) (bool, error) {
	from := time.Time{}
	if anchored.Valid {
		from = anchored.Time
	}
	oldest, err := q.GetOldestPriceRetrievedBetween(ctx, sqlcgen.GetOldestPriceRetrievedBetweenParams{PortfolioID: id, FromAt: from, Before: cutoff})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	local := oldest.In(loc)
	end := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc).UTC()
	if end.After(cutoff) {
		return false, nil
	}
	if err := q.MarkDailyPriceAnchors(ctx, sqlcgen.MarkDailyPriceAnchorsParams{PortfolioID: id, FromAt: from, Before: end}); err != nil {
		return false, err
	}
	return true, q.SetPriceAnchorsBefore(ctx, sqlcgen.SetPriceAnchorsBeforeParams{Before: sql.NullTime{Time: end, Valid: true}, ID: id})
}

func (s *Service) prunePrices(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff, historyBefore time.Time, anchored sql.NullTime) (bool, error) {
	if !anchored.Valid {
		cutoff = historyBefore
	} else if anchored.Time.Before(cutoff) {
		cutoff = anchored.Time
		if cutoff.Before(historyBefore) {
			cutoff = historyBefore
		}
	}
	pass := s.pass(id, "price_observations")
	if pass == nil {
		return false, nil
	}
	rows, err := q.ListPriceRetentionCandidates(ctx, sqlcgen.ListPriceRetentionCandidatesParams{PortfolioID: id, Before: cutoff, HistoryBefore: historyBefore, AfterAt: pass.after.At, AfterID: pass.after.ID})
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
	if err := q.DeleteUnpinnedPriceObservations(ctx, sqlcgen.DeleteUnpinnedPriceObservationsParams{PortfolioID: id, Ids: ids, HistoryBefore: historyBefore}); err != nil {
		return false, err
	}
	pass.after = sweepCursor(rows[len(rows)-1])
	return true, nil
}

func (s *Service) pruneManual(ctx context.Context, q *sqlcgen.Queries, id []byte, cutoff time.Time) (bool, error) {
	pass := s.pass(id, "manual_revisions")
	if pass == nil {
		return false, nil
	}
	revision, err := q.GetPrunableManualRevision(ctx, sqlcgen.GetPrunableManualRevisionParams{PortfolioID: id, Before: sql.NullTime{Time: cutoff, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		pass.done = true
		return false, nil
	}
	if err != nil {
		return false, err
	}
	n, err := q.PruneManualRevisionBalances(ctx, sqlcgen.PruneManualRevisionBalancesParams{PortfolioID: id, AccountID: revision.AccountID, RevisionNumber: revision.RevisionNumber})
	if err != nil || n > 0 {
		return true, err
	}
	return true, q.DeleteManualRevision(ctx, sqlcgen.DeleteManualRevisionParams{PortfolioID: id, AccountID: revision.AccountID, RevisionNumber: revision.RevisionNumber})
}

func nonNil(id []byte) []byte {
	if id == nil {
		return []byte{}
	}
	return id
}
