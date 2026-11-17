package financial

import (
	"context"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func emptyCandidate(a *valuation.AccountValuation) bool {
	if a.Status != adapter.StatusSucceeded || a.Freshness != valuation.FreshnessFresh || a.ValueUSD == nil || a.Completeness() != valuation.CompletenessComplete {
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

// emptyReports identifies confirmed-empty reports. A nil current map means no
// current manual overlay applies, as for a historical point read.
func (s *Service) emptyReports(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation, current map[uuid.UUID][]BalanceRow) (map[uuid.UUID]bool, error) {
	ids := [][]byte{}
	for i := range accounts {
		a := &accounts[i]
		if a.ReportID != uuid.Nil && len(current[a.AccountID]) == 0 && emptyCandidate(a) {
			ids = append(ids, a.ReportID[:])
		}
	}
	return s.reportHoldings(ctx, p, ids)
}

// historicalEmptyReports identifies confirmed-empty reports at a historical
// point. Current manual quantities are presentation state, not point facts, so
// the point's own manual report decides whether its account was empty.
func (s *Service) historicalEmptyReports(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation) (map[uuid.UUID]bool, error) {
	return s.emptyReports(ctx, p, accounts, nil)
}

func (s *Service) reportHoldings(ctx context.Context, p PortfolioContext, ids [][]byte) (map[uuid.UUID]bool, error) {
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

// hideHistoricalEmpty removes accounts whose report at this point was a fresh,
// successful, complete zero valuation without any holdings. Unknown and
// exceptional accounts stay visible, matching Overview.
func hideHistoricalEmpty(accounts []valuation.AccountValuation, empty map[uuid.UUID]bool) []valuation.AccountValuation {
	out := make([]valuation.AccountValuation, 0, len(accounts))
	for i := range accounts {
		if !empty[accounts[i].ReportID] {
			out = append(out, accounts[i])
		}
	}
	return out
}
