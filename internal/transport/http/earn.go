package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListEarnPositions(ctx context.Context, request api.ListEarnPositionsRequestObject) (api.ListEarnPositionsResponseObject, error) {
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
	list, err := a.Financial.FlatEarnPositions(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListEarnPositions200JSONResponse{
		Context:               toAPIContext(&list.Context),
		Items:                 toAPIEarnPositions(list.Items),
		Coverage:              toAPIReportCoverage(list.Coverage),
		Page:                  toOffsetPage(list.Offset, list.Limit, list.Total),
		ValuationCompleteness: api.Completeness(list.Completeness),
		Issues:                toAPIIssues(list.Issues),
	}, nil
}
