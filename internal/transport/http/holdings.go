package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) GetOverview(ctx context.Context, request api.GetOverviewRequestObject) (api.GetOverviewResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	view, err := a.Financial.Overview(ctx, sc.portfolioContext())
	if err != nil {
		return nil, err
	}
	platforms := make([]api.TreePlatform, 0, len(view.Platforms))
	for i := range view.Platforms {
		p := &view.Platforms[i]
		identities := make([]api.TreeIdentity, 0, len(p.Identities))
		for j := range p.Identities {
			id := &p.Identities[j]
			accounts := make([]api.AccountValuation, 0, len(id.Accounts))
			for k := range id.Accounts {
				accounts = append(accounts, toAPIAccountValuation(&id.Accounts[k]))
			}
			identities = append(identities, api.TreeIdentity{Id: id.ID, DisplayName: id.DisplayName, Status: api.IdentityStatus(id.Status), ConnectionStatus: api.ConnectionStatus(id.ConnectionStatus), ValueUsd: id.ValueUSD, Completeness: api.Completeness(id.Completeness), Accounts: accounts})
		}
		platforms = append(platforms, api.TreePlatform{Platform: p.Platform, Label: p.Label, ValueUsd: p.ValueUSD, Completeness: api.Completeness(p.Completeness), Identities: identities})
	}
	return api.GetOverview200JSONResponse{Context: toAPIContext(&view.Context), Point: toAPIPointPtr(view.Point), Platforms: platforms, Issues: toAPIIssues(view.Issues)}, nil
}

func (a *API) ListBalances(ctx context.Context, request api.ListBalancesRequestObject) (api.ListBalancesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		CycleID:              request.Params.CycleId,
		IncludePendingManual: request.Params.IncludePendingManual != nil && *request.Params.IncludePendingManual,
		Platform:             stringValue(request.Params.Platform),
		IdentityID:           request.Params.IdentityId,
		Symbol:               stringValue(request.Params.Q),
		Offset:               intValue(request.Params.Offset),
		Limit:                intValue(request.Params.Limit),
	}
	list, err := a.Financial.FlatBalances(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListBalances200JSONResponse{
		Context:                   toAPIContext(&list.Context),
		Coverage:                  toAPIReportCoverage(list.Coverage),
		ManualOverlay:             toAPIManualOverlay(list.ManualOverlay),
		FilteredTotalCompleteness: api.Completeness(list.TotalCompleteness),
		FilteredTotalFreshness:    api.Freshness(list.TotalFreshness),
		FilteredTotalIssues:       toAPIIssues(list.TotalIssues),
		Items:                     toAPIBalanceRows(list.Items),
		Page:                      toOffsetPage(list.Offset, list.Limit, list.Total),
		FilteredTotalUsd:          list.TotalUSD,
		ValuationCompleteness:     api.Completeness(list.Completeness),
		Issues:                    toAPIIssues(list.Issues),
	}, nil
}

func (a *API) ListPositions(ctx context.Context, request api.ListPositionsRequestObject) (api.ListPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		CycleID:    request.Params.CycleId,
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		Symbol:     stringValue(request.Params.Q),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	list, err := a.Financial.FlatPositions(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListPositions200JSONResponse{
		Context:                   toAPIContext(&list.Context),
		Coverage:                  toAPIReportCoverage(list.Coverage),
		Items:                     toAPIPositionRows(list.Items),
		Page:                      toOffsetPage(list.Offset, list.Limit, list.Total),
		FilteredTotalUsd:          list.TotalUSD,
		FilteredTotalCompleteness: api.Completeness(list.TotalCompleteness),
		FilteredTotalFreshness:    api.Freshness(list.TotalFreshness),
		FilteredTotalIssues:       toAPIIssues(list.TotalIssues),
		Issues:                    toAPIIssues(list.Issues),
	}, nil
}

func (a *API) ListDualPositions(ctx context.Context, request api.ListDualPositionsRequestObject) (api.ListDualPositionsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		CycleID:    request.Params.CycleId,
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		Symbol:     stringValue(request.Params.Q),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	list, err := a.Financial.FlatDualPositions(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListDualPositions200JSONResponse{
		Context:                   toAPIContext(&list.Context),
		Coverage:                  toAPIReportCoverage(list.Coverage),
		Items:                     toAPIDualPositionRows(list.Items),
		Page:                      toOffsetPage(list.Offset, list.Limit, list.Total),
		ValuationCompleteness:     api.Completeness(list.Completeness),
		FilteredTotalUsd:          list.TotalUSD,
		FilteredTotalCompleteness: api.Completeness(list.TotalCompleteness),
		FilteredTotalFreshness:    api.Freshness(list.TotalFreshness),
		FilteredTotalIssues:       toAPIIssues(list.TotalIssues),
		Issues:                    toAPIIssues(list.Issues),
	}, nil
}

func (a *API) GetAccountReport(ctx context.Context, request api.GetAccountReportRequestObject) (api.GetAccountReportResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	pending := request.Params.IncludePendingManual != nil && *request.Params.IncludePendingManual
	view, err := a.Financial.AccountReport(ctx, sc.portfolioContext(), request.AccountId, request.Params.CycleId, pending)
	if err != nil {
		return nil, err
	}
	return api.GetAccountReport200JSONResponse{ManualOverlay: toAPIManualOverlay(view.ManualOverlay), Context: toAPIContext(&view.Context), Account: toAPIAccountValuation(&view.Account), Balances: toAPIBalanceRows(view.Balances), Positions: toAPIPositionRows(view.Positions), EarnPositions: toAPIEarnPositions(view.EarnPositions), DualPositions: toAPIDualPositionRows(view.DualPositions), Issues: toAPIIssues(view.Issues)}, nil
}
