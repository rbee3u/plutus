package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAccountEarnPositions(ctx context.Context, request api.ListAccountEarnPositionsRequestObject) (api.ListAccountEarnPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Financial.AccountEarnPositions(ctx, sc.portfolioContext(), request.AccountId)
	if err != nil {
		return nil, err
	}
	return api.ListAccountEarnPositions200JSONResponse{
		Context:          toAPIContext(&view.Context),
		Account:          toAPIAccountValuation(&view.Account),
		Items:            toAPIEarnPositions(view.EarnPositions),
		BalanceSummaries: toAPIEarnSummaries(view.EarnBalanceSummaries),
		DetailsCollected: view.EarnDetailsCollected,
		Issues:           toAPIIssues(view.Issues),
	}, nil
}

func (a *API) ListEarnPositions(ctx context.Context, request api.ListEarnPositionsRequestObject) (api.ListEarnPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		Symbol:     stringValue(request.Params.Symbol),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	list, err := a.Financial.FlatEarnPositions(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListEarnPositions200JSONResponse{
		Context:          toAPIContext(&list.Context),
		Items:            toAPIEarnPositions(list.Items),
		BalanceSummaries: toAPIEarnSummaries(list.BalanceSummaries),
		DetailsComplete:  list.DetailsComplete,
		Page:             toOffsetPage(list.Offset, list.Limit, list.Total),
		Completeness:     api.Completeness(list.Completeness),
		Issues:           toAPIIssues(list.Issues),
	}, nil
}
