package financial

import (
	"context"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func emptyCandidate(a *valuation.AccountValuation) bool {
	if a.Status != adapters.StatusSucceeded || a.Freshness != valuation.FreshnessFresh || a.ValueUSD == nil || a.Completeness() != valuation.CompletenessComplete {
		return false
	}
	value, ok := decimal.Parse(*a.ValueUSD)
	if !ok || value.Sign() != 0 {
		return false
	}
	for _, metric := range []*string{a.Equity, a.AdjustedEquity, a.AvailableEquity} {
		if metric != nil {
			amount, ok := decimal.Parse(*metric)
			if !ok || amount.Sign() != 0 {
				return false
			}
		}
	}
	return true
}

func (s *Service) emptyReports(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation, current map[uuid.UUID][]BalanceRow) (map[uuid.UUID]bool, error) {
	ids := [][]byte{}
	for i := range accounts {
		a := &accounts[i]
		if a.ReportID != uuid.Nil && len(current[a.AccountID]) == 0 && emptyCandidate(a) {
			ids = append(ids, a.ReportID[:])
		}
	}
	empty := map[uuid.UUID]bool{}
	for start := 0; start < len(ids); start += 256 {
		rows, err := mysql.Queries(ctx, s.db).ListReportHoldings(ctx, sqlcgen.ListReportHoldingsParams{PortfolioID: p.ID[:], ReportIds: ids[start:min(start+256, len(ids))]})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return nil, err
			}
			empty[id] = row.HasHoldings.Valid && !row.HasHoldings.Bool
		}
	}
	return empty, nil
}
