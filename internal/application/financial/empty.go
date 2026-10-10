package financial

import (
	"context"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func (s *Service) confirmedEmpty(ctx context.Context, p PortfolioContext, a *valuation.AccountValuation) (bool, error) {
	if a.Status != adapters.StatusSucceeded || a.Freshness != valuation.FreshnessFresh || a.ValueUSD == nil || a.Completeness() != valuation.CompletenessComplete {
		return false, nil
	}
	value, ok := decimal.Parse(*a.ValueUSD)
	if !ok || value.Sign() != 0 {
		return false, nil
	}
	for _, metric := range []*string{a.Equity, a.AdjustedEquity, a.AvailableEquity} {
		if metric != nil {
			amount, ok := decimal.Parse(*metric)
			if !ok || amount.Sign() != 0 {
				return false, nil
			}
		}
	}
	id := a.ReportID
	held, err := mysql.Queries(ctx, s.db).ReportHasHoldings(ctx, sqlcgen.ReportHasHoldingsParams{PortfolioID: p.ID[:], SourceID: id[:]})
	if err != nil {
		return false, err
	}
	return held.Valid && !held.Bool, nil
}
