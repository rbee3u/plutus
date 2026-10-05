package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

func (a *API) ListAccountEarnPositions(ctx context.Context, request api.ListAccountEarnPositionsRequestObject) (api.ListAccountEarnPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Valuation.AccountEarnPositions(ctx, sc.valuation(), request.AccountId)
	if err != nil {
		return nil, err
	}
	return api.ListAccountEarnPositions200JSONResponse{Context: toAPIContext(&view.Context), Account: toAPIAccountValuation(&view.Account), Items: toAPIEarnPositions(view.EarnPositions), BalanceSummaries: toAPIEarnSummaries(view.EarnBalanceSummaries), DetailsCollected: view.EarnDetailsCollected, Issues: toAPIIssues(view.Issues)}, nil
}

func (a *API) ListEarnPositions(ctx context.Context, request api.ListEarnPositionsRequestObject) (api.ListEarnPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	f := valuation.ListFilter{Platform: stringValue(request.Params.Platform), IdentityID: request.Params.IdentityId, Symbol: stringValue(request.Params.Symbol), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)}
	list, err := a.Valuation.FlatEarnPositions(ctx, sc.valuation(), &f)
	if err != nil {
		return nil, err
	}
	return api.ListEarnPositions200JSONResponse{Context: toAPIContext(&list.Context), Items: toAPIEarnPositions(list.Items), BalanceSummaries: toAPIEarnSummaries(list.BalanceSummaries), DetailsComplete: list.DetailsComplete, Page: toOffsetPage(list.Offset, list.Limit, list.Total), Completeness: api.Completeness(list.Completeness), Issues: toAPIIssues(list.Issues)}, nil
}

func issueSlicePointer(issues []valuation.Issue) *[]api.Issue {
	out := toAPIIssues(issues)
	return &out
}

func toAPIEarnAmounts(in []adapters.EarnAmount) []api.EarnAmount {
	out := make([]api.EarnAmount, 0, len(in))
	for _, a := range in {
		out = append(out, api.EarnAmount{Symbol: a.Symbol, Amount: a.Amount, AssetSymbol: a.AssetSymbol})
	}
	return out
}

func toAPIEarnPositions(in []valuation.EarnPositionRow) []api.EarnPositionRow {
	out := make([]api.EarnPositionRow, 0, len(in))
	for i := range in {
		r := &in[i]
		rates := make([]api.EarnRate, 0, len(r.Rates))
		rewards := make([]api.EarnReward, 0, len(r.Rewards))
		details := make([]api.EarnDetail, 0, len(r.Details))
		for _, rate := range r.Rates {
			rates = append(rates, api.EarnRate{Value: rate.Value, Upper: rate.Upper, Kind: rate.Kind, Qualifier: rate.Qualifier, Label: rate.Label, Symbol: rate.Symbol})
		}
		for _, reward := range r.Rewards {
			rewards = append(rewards, api.EarnReward{Symbol: reward.Symbol, Amount: reward.Amount, Kind: reward.Kind})
		}
		for _, detail := range r.Details {
			details = append(details, api.EarnDetail{Label: detail.Label, Value: detail.Value, At: detail.At})
		}
		out = append(out, api.EarnPositionRow{AccountId: r.AccountID, AccountName: r.AccountName, AccountType: r.AccountType, IdentityId: r.IdentityID, IdentityName: r.IdentityName, Platform: r.Platform, PlatformPositionRef: r.PlatformPositionRef, ProductRef: r.ProductRef, ProductName: r.ProductName, Type: r.Type, Granularity: r.Granularity, AmountBasis: r.AmountBasis, Assets: toAPIEarnAmounts(r.Assets), Rates: rates, Rewards: rewards, Details: details, SubscribedAt: r.SubscribedAt, MaturityAt: r.MaturityAt, Status: r.Status, Freshness: api.Freshness(r.Freshness), ReportObservedAt: r.ReportObservedAt, Issues: toAPIIssues(r.Issues)})
	}
	return out
}

func toAPIEarnSummaries(in []valuation.EarnBalanceSummary) []api.EarnBalanceSummary {
	out := make([]api.EarnBalanceSummary, 0, len(in))
	for _, r := range in {
		out = append(out, api.EarnBalanceSummary{AccountId: r.AccountID, AccountName: r.AccountName, IdentityName: r.IdentityName, Platform: r.Platform, Assets: toAPIEarnAmounts(r.Assets), Freshness: api.Freshness(r.Freshness), ReportObservedAt: r.ReportObservedAt, Issues: toAPIIssues(r.Issues)})
	}
	return out
}
