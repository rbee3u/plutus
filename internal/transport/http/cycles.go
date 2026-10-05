package httpserver

import (
	"context"
	"errors"

	"github.com/rbee3u/plutus/internal/domain/jobs"
	"github.com/rbee3u/plutus/internal/domain/valuation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListCollectionCycles(ctx context.Context, request api.ListCollectionCyclesRequestObject) (api.ListCollectionCyclesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Jobs.ListCycles(ctx, sc.portfolio.ID, intValue(request.Params.Limit), stringValue(request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]api.CollectionCycle, 0, len(page.Items))
	for i := range page.Items {
		cycle := &page.Items[i]
		items = append(items, toAPICycle(&cycle.Job, toAPICyclePoint(cycle.ID, cycle.Point)))
	}
	return api.ListCollectionCycles200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func (a *API) CreateCollectionCycle(ctx context.Context, request api.CreateCollectionCycleRequestObject) (api.CreateCollectionCycleResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	job, _, err := a.Jobs.EnqueueCycle(ctx, sc.portfolio.ID, jobs.TriggerManual, &sc.user.ID)
	if err != nil {
		return nil, err
	}
	return api.CreateCollectionCycle202JSONResponse(toAPICycle(&job, nil)), nil
}

func (a *API) GetCollectionCycle(ctx context.Context, request api.GetCollectionCycleRequestObject) (api.GetCollectionCycleResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Jobs == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	job, err := a.Jobs.Get(ctx, sc.portfolio.ID, request.CycleId)
	if err != nil {
		return nil, err
	}
	if job.Kind != jobs.KindCollectionCycle {
		return nil, errNotFound
	}
	detail := api.CollectionCycleDetail{Accounts: []api.AccountValuation{}, Issues: []api.Issue{}}
	var point *api.ValuationPointSummary
	stored, err := a.Valuation.PointByJob(ctx, sc.portfolio.ID, job.ID)
	switch {
	case err == nil:
		point = toAPIPointPtr(&stored)
		detail.Issues = toAPIIssues(stored.Issues)
	case errors.Is(err, valuation.ErrNotFound), errors.Is(err, valuation.ErrNoPoint):
	default:
		return nil, err
	}
	accounts, err := a.Valuation.Accounts(ctx, sc.portfolio.ID, job.ID)
	if err != nil {
		return nil, err
	}
	detail.Accounts = toAPIAccountValuations(accounts)
	detail.Cycle = toAPICycle(&job, point)
	return api.GetCollectionCycle200JSONResponse(detail), nil
}
