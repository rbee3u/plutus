package valuation

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/listingexclusion"
)

const (
	rebuildCycleBatch = 64
	rebuildReadBatch  = 256
)

type reportInputs struct {
	sources  map[uuid.UUID]*sqlcgen.GetLatestOkReportForAccountRow
	balances map[uuid.UUID][]sqlcgen.ListBalancesByReportRow
}

type rebuildBatch struct {
	reports map[uuid.UUID][]sqlcgen.ListReportsByJobRow
	inputs  reportInputs
	prices  *historicalPrices
}

type rebuildPointTime struct {
	job uuid.UUID
	at  time.Time
}

// Rebuild recomputes succeeded cycles in their original order. Reads are
// bounded to a chunk of cycles; every point still commits independently.
func (s *Service) Rebuild(ctx context.Context, portfolioID uuid.UUID) (int, error) {
	jobs, err := s.q.ListSucceededCycleJobs(ctx, portfolioID[:])
	if err != nil || len(jobs) == 0 {
		return 0, err
	}
	stored, err := s.q.ListRebuildPointTimes(ctx, portfolioID[:])
	if err != nil {
		return 0, err
	}
	policy, err := listingexclusion.LoadHistory(ctx, s.db, portfolioID)
	if err != nil {
		return 0, err
	}
	timeline := make([]rebuildPointTime, 0, len(stored)+len(jobs))
	existing := map[uuid.UUID]time.Time{}
	for _, row := range stored {
		job, err := uuid.FromBytes(row.JobID)
		if err != nil {
			return 0, err
		}
		existing[job] = row.EvaluatedAt
		timeline = append(timeline, rebuildPointTime{job: job, at: row.EvaluatedAt})
	}
	var cachedJob uuid.UUID
	var cachedAccounts map[uuid.UUID]previousAccount
	count := 0
	for start := 0; start < len(jobs); start += rebuildCycleBatch {
		chunk := jobs[start:min(start+rebuildCycleBatch, len(jobs))]
		jobIDs := make([][]byte, len(chunk))
		for i := range chunk {
			jobIDs[i] = chunk[i].ID
		}
		batch, err := s.loadRebuildBatch(ctx, portfolioID, jobIDs)
		if err != nil {
			return count, err
		}
		for _, job := range chunk {
			if err := ctx.Err(); err != nil {
				return count, err
			}
			jobID, err := uuid.FromBytes(job.ID)
			if err != nil {
				return count, err
			}
			at, existed := existing[jobID]
			if !existed {
				at = job.CreatedAt
				if job.FinishedAt.Valid {
					at = job.FinishedAt.Time
				}
			}
			at = at.UTC().Truncate(time.Microsecond)
			i := sort.Search(len(timeline), func(i int) bool { return !timeline[i].at.Before(at) })
			if !existed && i < len(timeline) && timeline[i].at.Equal(at) {
				// The unique point time may belong to another job. Preserve the
				// ordinary writer's conflict behavior, including its returned error.
				if _, err := s.WritePoint(ctx, portfolioID, jobID, at); err != nil {
					return count, err
				}
			}
			previous, err := s.rebuildPrevious(ctx, portfolioID, jobID, at, timeline, cachedJob, cachedAccounts)
			if err != nil {
				return count, err
			}
			rows := batch.reports[jobID]
			ignored := policy.At(job.CreatedAt)
			accounts := make([]AccountValuation, 0, len(rows))
			for i := range rows {
				account, err := s.valueReport(ctx, batch.prices, &rows[i], ignored, &batch.inputs)
				if err != nil {
					return count, fmt.Errorf("rebuild report for job %s: %w", jobID, err)
				}
				accounts = append(accounts, account)
			}
			sortAccounts(accounts)
			point := buildPoint(portfolioID, jobID, at, accounts, previous)
			if err := s.persistPoint(ctx, point, accounts); err != nil {
				return count, fmt.Errorf("rebuild point for job %s: %w", jobID, err)
			}
			count++
			if !existed {
				timeline = append(timeline, rebuildPointTime{})
				copy(timeline[i+1:], timeline[i:])
				timeline[i] = rebuildPointTime{job: jobID, at: at}
			}
			cachedJob, cachedAccounts = jobID, previousFromAccounts(accounts)
		}
	}
	return count, nil
}

func (s *Service) rebuildPrevious(ctx context.Context, portfolioID, jobID uuid.UUID, at time.Time, timeline []rebuildPointTime, cachedJob uuid.UUID, cached map[uuid.UUID]previousAccount) (map[uuid.UUID]previousAccount, error) {
	i := sort.Search(len(timeline), func(i int) bool { return !timeline[i].at.Before(at) }) - 1
	if i < 0 {
		return nil, nil //nolint:nilnil // No preceding point means no boundary derivation.
	}
	if timeline[i].job == cachedJob && (i == 0 || !timeline[i-1].at.Equal(timeline[i].at)) {
		return cached, nil
	}
	// Existing evaluation times need not follow job creation order. Resolve
	// nonadjacent and tied predecessors through the original database lookup.
	return s.previousAccounts(ctx, portfolioID, at, jobID)
}

func previousFromAccounts(accounts []AccountValuation) map[uuid.UUID]previousAccount {
	previous := make(map[uuid.UUID]previousAccount, len(accounts))
	for _, a := range accounts {
		entry := previousAccount{manualState: a.manualState, scopeVersion: a.ScopeVersion, accountType: a.AccountType, name: a.AccountName, identityID: a.IdentityID, identityName: a.IdentityName, platform: a.Platform, method: a.Method}
		if a.ValueUSD != nil {
			entry.value, _ = decimal.Parse(*a.ValueUSD)
		}
		previous[a.AccountID] = entry
	}
	return previous
}

func (s *Service) loadRebuildBatch(ctx context.Context, portfolioID uuid.UUID, jobs [][]byte) (rebuildBatch, error) {
	b := rebuildBatch{reports: map[uuid.UUID][]sqlcgen.ListReportsByJobRow{}, inputs: reportInputs{sources: map[uuid.UUID]*sqlcgen.GetLatestOkReportForAccountRow{}, balances: map[uuid.UUID][]sqlcgen.ListBalancesByReportRow{}}}
	rows, err := s.q.ListRebuildReports(ctx, sqlcgen.ListRebuildReportsParams{PortfolioID: portfolioID[:], JobIds: jobs})
	if err != nil {
		return b, err
	}
	failed := [][]byte{}
	sourceTimes := map[uuid.UUID]time.Time{}
	for _, row := range rows {
		job, err := uuid.FromBytes(row.JobID)
		if err != nil {
			return b, err
		}
		b.reports[job] = append(b.reports[job], sqlcgen.ListReportsByJobRow(row))
		if row.Status != "ok" {
			failed = append(failed, row.ID)
		} else if !row.Equity.Valid {
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return b, err
			}
			sourceTimes[id] = row.ObservedAt.Time
		}
	}
	for start := 0; start < len(failed); start += rebuildReadBatch {
		sources, err := s.q.ListRebuildSources(ctx, sqlcgen.ListRebuildSourcesParams{PortfolioID: portfolioID[:], FailedIds: failed[start:min(start+rebuildReadBatch, len(failed))]})
		if err != nil {
			return b, err
		}
		for _, source := range sources {
			failedID, err := uuid.FromBytes(source.FailedReportID)
			if err != nil {
				return b, err
			}
			b.inputs.sources[failedID] = &sqlcgen.GetLatestOkReportForAccountRow{ID: source.ID, PortfolioID: source.PortfolioID, JobID: source.JobID, AccountID: source.AccountID, ScopeVersion: source.ScopeVersion, MarginMode: source.MarginMode, Status: source.Status, ObservedAt: source.ObservedAt, Equity: source.Equity, AdjustedEquity: source.AdjustedEquity, AvailableEquity: source.AvailableEquity, InitialMarginRatio: source.InitialMarginRatio, MaintenanceMarginRatio: source.MaintenanceMarginRatio, UnifiedMarginRatio: source.UnifiedMarginRatio, CreatedAt: source.CreatedAt}
			if !source.Equity.Valid {
				id, err := uuid.FromBytes(source.ID)
				if err != nil {
					return b, err
				}
				sourceTimes[id] = source.ObservedAt.Time
			}
		}
	}
	reports := make([][]byte, 0, len(sourceTimes))
	var from, to time.Time
	for id, at := range sourceTimes {
		reports = append(reports, id[:])
		b.inputs.balances[id] = []sqlcgen.ListBalancesByReportRow{}
		if from.IsZero() || at.Before(from) {
			from = at
		}
		if to.IsZero() || at.After(to) {
			to = at
		}
	}
	symbols := map[string]bool{}
	for start := 0; start < len(reports); start += rebuildReadBatch {
		balances, err := s.q.ListRebuildBalances(ctx, sqlcgen.ListRebuildBalancesParams{PortfolioID: portfolioID[:], ReportIds: reports[start:min(start+rebuildReadBatch, len(reports))]})
		if err != nil {
			return b, err
		}
		for _, row := range balances {
			id, err := uuid.FromBytes(row.ReportID)
			if err != nil {
				return b, err
			}
			b.inputs.balances[id] = append(b.inputs.balances[id], sqlcgen.ListBalancesByReportRow(row))
			switch row.AssetSymbol {
			case "USD", "USDT", "USDC":
			default:
				symbols[row.AssetSymbol] = true
			}
		}
	}
	assets := make([]string, 0, len(symbols))
	for symbol := range symbols {
		assets = append(assets, symbol)
	}
	prices := []sqlcgen.Price{}
	if len(assets) > 0 {
		prices, err = s.q.ListRebuildPrices(ctx, sqlcgen.ListRebuildPricesParams{AssetSymbols: assets, FromAt: from.Add(-maxPriceAge), ToAt: to})
		if err != nil {
			return b, err
		}
	}
	b.prices = newHistoricalPrices(s.q, prices)
	return b, nil
}
