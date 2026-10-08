package financial

import (
	"context"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

const sparklineWindow = 7 * 24 * time.Hour

// Overview answers the landing page from the latest point.
func (s *Service) Overview(ctx context.Context, p PortfolioContext) (Overview, error) {
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Context: c, Completeness: valuation.CompletenessComplete, Issues: []valuation.Issue{}, ByPlatform: []GroupTotal{}, ByIdentity: []GroupTotal{}, Sparkline: []valuation.Point{}}
	if point == nil {
		out.Completeness = valuation.CompletenessIncomplete
		out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	out.Point = point
	total := point.TotalUSD
	out.TotalUSD = &total
	out.Completeness = point.Completeness()
	out.AccountCount, out.StaleAccountCount, out.IncompleteAccountCount = point.AccountCount, point.StaleAccountCount, point.IncompleteAccountCount
	out.Issues = point.Issues
	out.ByPlatform = groupAccounts(accounts, func(a *valuation.AccountValuation) (string, string, string) {
		return a.Platform, adapters.PlatformName(adapters.Platform(a.Platform)), a.Platform
	})
	out.ByIdentity = groupAccounts(accounts, func(a *valuation.AccountValuation) (string, string, string) {
		return a.IdentityID.String(), a.IdentityName, a.Platform
	})
	series, err := s.q.ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: point.EvaluatedAt.Add(-sparklineWindow), ToAt: point.EvaluatedAt})
	if err != nil {
		return Overview{}, err
	}
	points, err := pointsFromRows(series)
	if err != nil {
		return Overview{}, err
	}
	out.Sparkline = sample(points, "hour", p.location())
	return out, nil
}

func groupAccounts(accounts []valuation.AccountValuation, keyFn func(*valuation.AccountValuation) (key, label, platform string)) []GroupTotal {
	order := []string{}
	groups := map[string]*GroupTotal{}
	sums := map[string]*big.Rat{}
	for i := range accounts {
		a := &accounts[i]
		key, label, platform := keyFn(a)
		g, ok := groups[key]
		if !ok {
			g = &GroupTotal{Key: key, Label: label, Platform: platform, Completeness: valuation.CompletenessComplete}
			groups[key] = g
			sums[key] = decimal.Zero()
			order = append(order, key)
		}
		g.AccountCount++
		g.RowCount++
		if a.ValueUSD != nil {
			if value, ok := decimal.Parse(*a.ValueUSD); ok {
				sums[key] = decimal.Add(sums[key], value)
			}
		}
		if a.Completeness() == valuation.CompletenessIncomplete {
			g.Completeness = valuation.CompletenessIncomplete
		}
	}
	out := make([]GroupTotal, 0, len(order))
	for _, key := range order {
		g := groups[key]
		g.ValueUSD = decimal.FormatPtr(sums[key])
		out = append(out, *g)
	}
	return out
}
