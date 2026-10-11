package valuation

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/assetexclusion"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

const historyBatch = 64

// InitializeHistory writes a point for every succeeded cycle of a Portfolio
// that has none yet, in completion order. It shares WritePoint's calculation
// but loads sources in batches, carries the previous point in memory, and skips
// the per-Account evidence rereads. Only development seeds use it; their tests
// compare every edge-case point with WritePoint on the same source facts.
func (s *Service) InitializeHistory(ctx context.Context, portfolioID uuid.UUID) error {
	_, err := mysql.WithWriteContext(ctx, s.db, func(ctx context.Context) (struct{}, error) {
		q := mysql.Queries(ctx, s.db)
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return struct{}{}, err
		}
		if _, err := q.GetLatestValuationPoint(ctx, portfolioID[:]); !errors.Is(err, sql.ErrNoRows) {
			if err == nil {
				err = errors.New("history initialization requires a Portfolio without Valuation Points")
			}
			return struct{}{}, err
		}
		history, err := q.ListSucceededCycles(ctx, portfolioID[:])
		if err != nil {
			return struct{}{}, err
		}
		for _, cycle := range history {
			if !cycle.CollectionFinishedAt.Valid {
				return struct{}{}, fmt.Errorf("succeeded cycle %x lacks collection completion time", cycle.ID)
			}
		}
		slices.SortFunc(history, func(a, b sqlcgen.ListSucceededCyclesRow) int {
			return cmp.Or(a.CollectionFinishedAt.Time.Compare(b.CollectionFinishedAt.Time), bytes.Compare(a.ID, b.ID))
		})
		var previous map[uuid.UUID]previousAccount
		for start := 0; start < len(history); start += historyBatch {
			batch := history[start:min(start+historyBatch, len(history))]
			reports, inputs, err := s.historyInputs(ctx, q, portfolioID, batch)
			if err != nil {
				return struct{}{}, err
			}
			for _, cycle := range batch {
				cycleID, err := uuid.FromBytes(cycle.ID)
				if err != nil {
					return struct{}{}, err
				}
				accounts, err := s.valueHistoryCycle(ctx, q, portfolioID, cycleID, reports[cycleID], inputs)
				if err != nil {
					return struct{}{}, fmt.Errorf("initialize Valuation Point %s: %w", cycleID, err)
				}
				evaluatedAt := cycle.CollectionFinishedAt.Time.UTC().Truncate(time.Microsecond)
				point := buildPoint(portfolioID, cycleID, evaluatedAt, accounts, previous)
				if err := s.insertHistoryPoint(ctx, q, &point, accounts); err != nil {
					return struct{}{}, fmt.Errorf("initialize Valuation Point %s: %w", cycleID, err)
				}
				previous = previousFromAccounts(accounts)
			}
		}
		return struct{}{}, nil
	})
	return err
}

// historyInputs loads the reports of a cycle batch, the stale sources their
// failures fall back to, and every source's balances.
func (s *Service) historyInputs(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, batch []sqlcgen.ListSucceededCyclesRow) (map[uuid.UUID][]sqlcgen.ListReportsByCycleRow, *reportInputs, error) {
	cycleIDs := make([][]byte, len(batch))
	for i := range batch {
		cycleIDs[i] = batch[i].ID
	}
	rows, err := q.ListReportsByCycles(ctx, sqlcgen.ListReportsByCyclesParams{PortfolioID: portfolioID[:], CycleIds: cycleIDs})
	if err != nil {
		return nil, nil, err
	}
	reports := map[uuid.UUID][]sqlcgen.ListReportsByCycleRow{}
	inputs := &reportInputs{sources: map[uuid.UUID]*sqlcgen.AccountReport{}, balances: map[uuid.UUID][]sqlcgen.ListBalancesByReportRow{}}
	sourceIDs := [][]byte{}
	for _, stored := range rows {
		row := sqlcgen.ListReportsByCycleRow(stored)
		cycleID, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return nil, nil, err
		}
		reports[cycleID] = append(reports[cycleID], row)
		source := row.ID
		if row.Status != adapters.StatusSucceeded {
			src, err := q.GetLatestSucceededReportForAccount(ctx, sqlcgen.GetLatestSucceededReportForAccountParams{PortfolioID: row.PortfolioID, AccountID: row.AccountID, Before: row.RecordedAt})
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			reportID, err := uuid.FromBytes(row.ID)
			if err != nil {
				return nil, nil, err
			}
			inputs.sources[reportID] = &src
			source = src.ID
		}
		sourceID, err := uuid.FromBytes(source)
		if err != nil {
			return nil, nil, err
		}
		if _, loaded := inputs.balances[sourceID]; !loaded {
			inputs.balances[sourceID] = nil
			sourceIDs = append(sourceIDs, source)
		}
	}
	for start := 0; start < len(sourceIDs); start += valuationReadBatch {
		lines, err := q.ListBalancesByReports(ctx, sqlcgen.ListBalancesByReportsParams{PortfolioID: portfolioID[:], ReportIds: sourceIDs[start:min(start+valuationReadBatch, len(sourceIDs))]})
		if err != nil {
			return nil, nil, err
		}
		for _, line := range lines {
			sourceID, err := uuid.FromBytes(line.ReportID)
			if err != nil {
				return nil, nil, err
			}
			inputs.balances[sourceID] = append(inputs.balances[sourceID], sqlcgen.ListBalancesByReportRow(line))
		}
	}
	return reports, inputs, nil
}

func (s *Service) valueHistoryCycle(ctx context.Context, q *sqlcgen.Queries, portfolioID, cycleID uuid.UUID, rows []sqlcgen.ListReportsByCycleRow, inputs *reportInputs) ([]AccountValuation, error) {
	exclusions, err := assetexclusion.ActiveForCycle(ctx, s.db, portfolioID, cycleID)
	if err != nil {
		return nil, err
	}
	book := NewPriceBook(q, portfolioID)
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		input, err := s.prepareReport(ctx, book, &rows[i], exclusions, inputs)
		if err != nil {
			return nil, err
		}
		out = append(out, evaluateReport(input))
	}
	sortAccounts(out)
	return out, nil
}

func (s *Service) insertHistoryPoint(ctx context.Context, q *sqlcgen.Queries, point *Point, accounts []AccountValuation) error {
	if err := q.InsertValuationPoint(ctx, sqlcgen.InsertValuationPointParams{
		PortfolioID:            point.PortfolioID[:],
		CycleID:                point.CycleID[:],
		EvaluatedAt:            point.EvaluatedAt,
		TotalUsd:               point.TotalUSD,
		AccountCount:           uint32(point.AccountCount),           // #nosec G115 -- The count is bounded by the in-memory Account slice, far below 2^32.
		StaleAccountCount:      uint32(point.StaleAccountCount),      // #nosec G115 -- The count is a subset of the in-memory Account slice.
		IncompleteAccountCount: uint32(point.IncompleteAccountCount), // #nosec G115 -- The count is a subset of the in-memory Account slice.
		CalculationVersion:     CalculationVersion,
	}); err != nil {
		return err
	}
	if len(accounts) > 0 {
		args := make([]any, 0, len(accounts)*6)
		for _, account := range accounts {
			if err := validateBalanceValuations(account.BalanceValuations); err != nil {
				return err
			}
			var source []byte
			if account.SourceReportID != nil {
				source = account.SourceReportID[:]
			}
			var value sql.NullString
			if account.ValueUSD != nil {
				value = sql.NullString{String: *account.ValueUSD, Valid: true}
			}
			args = append(args, point.PortfolioID[:], point.CycleID[:], account.AccountID[:], source, account.Method, value)
		}
		statement := "INSERT INTO account_valuations (portfolio_id,cycle_id,account_id,source_report_id,valuation_method,value_usd) VALUES " + strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?),", len(accounts)), ",")
		if _, err := mysql.Executor(ctx, s.db).ExecContext(ctx, statement, args...); err != nil {
			return err
		}
	}
	balanceRows := [][]any{}
	for _, account := range accounts {
		var source []byte
		if account.SourceReportID != nil {
			source = account.SourceReportID[:]
		}
		for _, line := range account.BalanceValuations {
			var price []byte
			if line.PriceObservationID != nil {
				price = line.PriceObservationID[:]
			}
			var value sql.NullString
			if line.ValueUSD != nil {
				value = sql.NullString{String: *line.ValueUSD, Valid: true}
			}
			balanceRows = append(balanceRows, []any{point.PortfolioID[:], point.CycleID[:], account.AccountID[:], source, line.AssetID[:], line.CanonicalAssetSymbol, line.PricingMethod, price, value})
			if len(balanceRows) == 256 {
				if err := s.insertBalanceRows(ctx, balanceRows); err != nil {
					return err
				}
				balanceRows = balanceRows[:0]
			}
		}
	}
	if err := s.insertBalanceRows(ctx, balanceRows); err != nil {
		return err
	}
	for _, event := range point.BoundaryEvents {
		var value sql.NullString
		if event.ValueUSD != nil {
			value = sql.NullString{String: *event.ValueUSD, Valid: true}
		}
		if err := q.InsertBoundaryEvent(ctx, sqlcgen.InsertBoundaryEventParams{PortfolioID: point.PortfolioID[:], CycleID: point.CycleID[:], AccountID: event.AccountID[:], Direction: event.Direction, ValuationMethod: event.Method, ValueUsd: value, ValuationComplete: event.ValuationComplete}); err != nil {
			return err
		}
	}
	return s.persistComparisonEvidence(ctx, q, point)
}
