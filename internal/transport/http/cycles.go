package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListCollectionCycles(ctx context.Context, request api.ListCollectionCyclesRequestObject) (api.ListCollectionCyclesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Cycles == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Cycles.List(ctx, sc.portfolio.ID, intValue(request.Params.Limit), stringValue(request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	var from, to time.Time
	for i := range page.Items {
		if point := page.Items[i].Point; point != nil {
			if from.IsZero() || point.EvaluatedAt.Before(from) {
				from = point.EvaluatedAt
			}
			if point.EvaluatedAt.After(to) {
				to = point.EvaluatedAt
			}
		}
	}
	totals, err := a.Valuation.BoundaryTotalsBetween(ctx, sc.portfolio.ID, from, to)
	if err != nil {
		return nil, err
	}
	items := make([]api.CollectionCycle, 0, len(page.Items))
	for i := range page.Items {
		cycle := &page.Items[i]
		contribution, withdrawal := totals(cycle.ID)
		items = append(items, toAPICycle(cycle, toAPICyclePoint(cycle.ID, cycle.Point, contribution, withdrawal)))
	}
	return api.ListCollectionCycles200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func (a *API) CreateCollectionCycle(ctx context.Context, request api.CreateCollectionCycleRequestObject) (api.CreateCollectionCycleResponseObject, error) {
	if a.Cycles == nil {
		return nil, ErrNotImplemented
	}
	body, err := createIdempotent(ctx, a, request.Params.XCSRFToken, stringValue(request.Params.IdempotencyKey), struct{}{}, http.StatusAccepted, func(ctx context.Context, user auth.User) (api.CollectionCycle, error) {
		owned, err := a.ownedPortfolio(ctx, user, request.PortfolioId)
		if err != nil {
			return api.CollectionCycle{}, err
		}
		cycle, _, err := a.Cycles.Enqueue(ctx, owned.ID, cycles.TriggerUser)
		if err != nil {
			return api.CollectionCycle{}, err
		}
		return toAPICycle(&cycle, nil), nil
	})
	if err != nil {
		return nil, err
	}
	return api.CreateCollectionCycle202JSONResponse(body), nil
}

func (a *API) GetCollectionCycle(ctx context.Context, request api.GetCollectionCycleRequestObject) (api.GetCollectionCycleResponseObject, error) {
	return mysql.WithReadContext(ctx, a.DB, func(ctx context.Context) (api.GetCollectionCycleResponseObject, error) {
		sc, err := a.scoped(ctx, request.PortfolioId)
		if err != nil {
			return nil, err
		}
		if a.Cycles == nil || a.Valuation == nil {
			return nil, ErrNotImplemented
		}
		cycle, err := a.Cycles.Get(ctx, sc.portfolio.ID, request.CycleId)
		if err != nil {
			return nil, err
		}
		detail := api.CollectionCycleDetail{Accounts: []api.AccountValuation{}, Issues: []api.Issue{}}
		var point *api.ValuationPointSummary
		stored, accounts, err := a.Valuation.PointByCycleWithAccounts(ctx, sc.portfolio.ID, cycle.ID)
		switch {
		case err == nil:
			point = toAPIPointPtr(&stored)
			detail.Issues = toAPIIssues(stored.Issues)
			detail.Accounts = toAPIAccountValuations(accounts)
		case errors.Is(err, valuation.ErrNotFound), errors.Is(err, valuation.ErrNoPoint):
		default:
			return nil, err
		}
		detail.Cycle = toAPICycle(&cycle, point)
		return api.GetCollectionCycle200JSONResponse(detail), nil
	})
}
