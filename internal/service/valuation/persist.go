package valuation

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/service/cycle"
)

// persistPoint commits account valuations and their aggregate atomically.
func (s *Service) persistPoint(ctx context.Context, point Point, accounts []AccountValuation, claims ...*cycle.Claim) error {
	_, err := mysql.WithWriteContext(ctx, s.db, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, s.persistPointRows(ctx, point, accounts, claims...)
	})
	return err
}

func (s *Service) persistPointRows(ctx context.Context, point Point, accounts []AccountValuation, claims ...*cycle.Claim) error {
	q := mysql.Queries(ctx, s.db)

	if _, err := q.LockPortfolio(ctx, point.PortfolioID[:]); err != nil {
		return err
	}
	var claim *cycle.Claim
	if len(claims) > 0 {
		claim = claims[0]
	}
	now := s.now().UTC()
	if claim != nil {
		if claim.PortfolioID != point.PortfolioID || claim.ID != point.CycleID {
			return cycle.ErrLostLease
		}
		if err := cycle.LockLease(ctx, q, claim, now); err != nil {
			return err
		}
	}
	if err := q.InsertValuationPoint(ctx, sqlcgen.InsertValuationPointParams{
		PortfolioID:            point.PortfolioID[:],
		CycleID:                point.CycleID[:],
		EvaluatedAt:            point.EvaluatedAt,
		TotalUsd:               point.TotalUSD,
		AccountCount:           uint32(point.AccountCount),           // #nosec G115 -- The count is bounded by the in-memory account slice, far below 2^32.
		StaleAccountCount:      uint32(point.StaleAccountCount),      // #nosec G115 -- The count is a subset of the in-memory account slice.
		IncompleteAccountCount: uint32(point.IncompleteAccountCount), // #nosec G115 -- The count is a subset of the in-memory account slice.
		CalculationVersion:     CalculationVersion,
	}); err != nil {
		return err
	}
	prices := map[uuid.UUID]sqlcgen.PriceObservation{}
	priceIDs := [][]byte{}
	seen := map[uuid.UUID]bool{}
	for _, account := range accounts {
		for _, line := range account.BalanceValuations {
			if line.PriceObservationID != nil {
				id := *line.PriceObservationID
				if !seen[id] {
					seen[id] = true
					priceIDs = append(priceIDs, id[:])
				}
			}
		}
	}
	for start := 0; start < len(priceIDs); start += 256 {
		rows, err := q.ListPriceObservationsByIDs(ctx, sqlcgen.ListPriceObservationsByIDsParams{PortfolioID: point.PortfolioID[:], Ids: priceIDs[start:min(start+256, len(priceIDs))]})
		if err != nil {
			return err
		}
		for _, row := range rows {
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return err
			}
			prices[id] = row
		}
	}
	balanceRows := [][]any{}
	for _, account := range accounts {
		lines := account.BalanceValuations
		if lines == nil {
			lines = []BalanceValuation{}
		}
		if err := validateBalanceValuations(lines); err != nil {
			return err
		}
		if err := validateEvidence(ctx, q, point.PortfolioID, &account, prices); err != nil {
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
		if err := q.InsertAccountValuation(ctx, sqlcgen.InsertAccountValuationParams{PortfolioID: point.PortfolioID[:], CycleID: point.CycleID[:], AccountID: account.AccountID[:], SourceReportID: source, ValuationMethod: account.Method, ValueUsd: value}); err != nil {
			return err
		}
		for _, line := range lines {
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
	if err := s.persistComparisonEvidence(ctx, q, &point); err != nil {
		return err
	}
	if claim != nil {
		return cycle.MarkSucceeded(ctx, q, claim, now)
	}
	return nil
}

func (s *Service) insertBalanceRows(ctx context.Context, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	args := make([]any, 0, len(rows)*9)
	for _, row := range rows {
		args = append(args, row...)
	}
	statement := "INSERT INTO balance_valuations (portfolio_id,cycle_id,account_id,source_report_id,asset_id,canonical_asset_symbol,pricing_method,price_observation_id,value_usd) VALUES " + strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?,?,?,?),", len(rows)), ",")
	_, err := mysql.Executor(ctx, s.db).ExecContext(ctx, statement, args...)
	return err
}
