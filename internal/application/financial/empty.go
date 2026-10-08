package financial

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func (s *Service) confirmedEmpty(ctx context.Context, a *valuation.AccountValuation) (bool, error) {
	if a.Status != "ok" || a.Freshness != valuation.FreshnessFresh || a.ValueUSD == nil || a.Completeness() != valuation.CompletenessComplete {
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
	held, err := s.q.ReportHasHoldings(ctx, sqlcgen.ReportHasHoldingsParams{SourceID: id[:]})
	if err != nil {
		return false, err
	}
	return held.Valid && !held.Bool, nil
}
