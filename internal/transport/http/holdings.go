package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

func (a *API) GetOverview(ctx context.Context, request api.GetOverviewRequestObject) (api.GetOverviewResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Valuation.Overview(ctx, sc.valuation())
	if err != nil {
		return nil, err
	}
	return api.GetOverview200JSONResponse{
		Context:                toAPIContext(&view.Context),
		Point:                  toAPIPointPtr(view.Point),
		TotalUsd:               view.TotalUSD,
		Completeness:           api.Completeness(view.Completeness),
		AccountCount:           view.AccountCount,
		StaleAccountCount:      view.StaleAccountCount,
		IncompleteAccountCount: view.IncompleteAccountCount,
		ByPlatform:             toAPIGroupTotals(view.ByPlatform),
		ByIdentity:             toAPIGroupTotals(view.ByIdentity),
		Issues:                 toAPIIssues(view.Issues),
		Sparkline:              toAPIPoints(view.Sparkline),
	}, nil
}

func (a *API) GetHoldingsTree(ctx context.Context, request api.GetHoldingsTreeRequestObject) (api.GetHoldingsTreeResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	tree, err := a.Valuation.Tree(ctx, sc.valuation())
	if err != nil {
		return nil, err
	}
	platforms := make([]api.TreePlatform, 0, len(tree.Platforms))
	for i := range tree.Platforms {
		p := &tree.Platforms[i]
		identities := make([]api.TreeIdentity, 0, len(p.Identities))
		for j := range p.Identities {
			id := &p.Identities[j]
			accounts := make([]api.AccountValuation, 0, len(id.Accounts))
			for k := range id.Accounts {
				accounts = append(accounts, toAPIAccountValuation(&id.Accounts[k].AccountValuation))
			}
			identities = append(identities, api.TreeIdentity{Id: id.ID, DisplayName: id.DisplayName, Status: id.Status, ConnectionStatus: id.ConnectionStatus, ValueUsd: id.ValueUSD, Completeness: api.Completeness(id.Completeness), Accounts: accounts})
		}
		platforms = append(platforms, api.TreePlatform{Platform: p.Platform, Label: p.Label, ValueUsd: p.ValueUSD, Completeness: api.Completeness(p.Completeness), Identities: identities})
	}
	return api.GetHoldingsTree200JSONResponse{Context: toAPIContext(&tree.Context), TotalUsd: tree.TotalUSD, Completeness: api.Completeness(tree.Completeness), Platforms: platforms, Issues: toAPIIssues(tree.Issues)}, nil
}

func (a *API) ListAccountBalances(ctx context.Context, request api.ListAccountBalancesRequestObject) (api.ListAccountBalancesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Valuation.AccountBalances(ctx, sc.valuation(), request.AccountId)
	if err != nil {
		return nil, err
	}
	return api.ListAccountBalances200JSONResponse{Context: toAPIContext(&view.Context), Account: toAPIAccountValuation(&view.Account), Items: toAPIBalanceRows(view.Balances), Issues: toAPIIssues(view.Issues)}, nil
}

func (a *API) ListAccountPositions(ctx context.Context, request api.ListAccountPositionsRequestObject) (api.ListAccountPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Valuation.AccountPositions(ctx, sc.valuation(), request.AccountId)
	if err != nil {
		return nil, err
	}
	return api.ListAccountPositions200JSONResponse{Context: toAPIContext(&view.Context), Account: toAPIAccountValuation(&view.Account), Items: toAPIPositionRows(view.Positions), Issues: toAPIIssues(view.Issues)}, nil
}

func (a *API) ListBalances(ctx context.Context, request api.ListBalancesRequestObject) (api.ListBalancesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	filter := valuation.ListFilter{
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		AccountID:  request.Params.AccountId,
		Symbol:     stringValue(request.Params.Symbol),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	if request.Params.GroupBy != nil {
		filter.GroupBy = string(*request.Params.GroupBy)
	}
	list, err := a.Valuation.FlatBalances(ctx, sc.valuation(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListBalances200JSONResponse{
		Context:      toAPIContext(&list.Context),
		Items:        toAPIBalanceRows(list.Items),
		Groups:       toAPIGroupTotals(list.Groups),
		Page:         toOffsetPage(list.Offset, list.Limit, list.Total),
		TotalUsd:     list.TotalUSD,
		Completeness: api.Completeness(list.Completeness),
		Issues:       toAPIIssues(list.Issues),
	}, nil
}

func (a *API) ListPositions(ctx context.Context, request api.ListPositionsRequestObject) (api.ListPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	filter := valuation.ListFilter{
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		AccountID:  request.Params.AccountId,
		Symbol:     stringValue(request.Params.Symbol),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	list, err := a.Valuation.FlatPositions(ctx, sc.valuation(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListPositions200JSONResponse{
		Context:          toAPIContext(&list.Context),
		Items:            toAPIPositionRows(list.Items),
		Page:             toOffsetPage(list.Offset, list.Limit, list.Total),
		TotalNotionalUsd: list.TotalUSD,
		Completeness:     api.Completeness(list.Completeness),
		Issues:           toAPIIssues(list.Issues),
	}, nil
}
