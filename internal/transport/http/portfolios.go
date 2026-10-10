package httpserver

import (
	"context"
	"net/http"

	"github.com/rbee3u/plutus/internal/application/auth"

	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListPortfolios(ctx context.Context, _ api.ListPortfoliosRequestObject) (api.ListPortfoliosResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return nil, errUnauthenticated
	}
	if a.Portfolios == nil {
		return nil, ErrNotImplemented
	}
	items, err := a.Portfolios.List(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := make([]api.Portfolio, 0, len(items))
	for i := range items {
		out = append(out, toAPIPortfolio(&items[i]))
	}
	return api.ListPortfolios200JSONResponse{Items: out}, nil
}

func (a *API) CreatePortfolio(ctx context.Context, request api.CreatePortfolioRequestObject) (api.CreatePortfolioResponseObject, error) {
	if a.Portfolios == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	body, etag, err := createIdempotent(ctx, a, request.Params.XCSRFToken, stringValue(request.Params.IdempotencyKey), request.Body, http.StatusCreated, func(ctx context.Context, user auth.User) (api.Portfolio, string, error) {
		item, err := a.Portfolios.Create(ctx, user.ID, portfolio.CreateInput{Name: request.Body.Name, Timezone: stringValue(request.Body.Timezone), CollectionIntervalSeconds: request.Body.CollectionIntervalSeconds})
		if err != nil {
			return api.Portfolio{}, "", err
		}
		return toAPIPortfolio(&item), item.ETag(), nil
	})
	if err != nil {
		return nil, err
	}
	return api.CreatePortfolio201JSONResponse{Body: body, Headers: api.CreatePortfolio201ResponseHeaders{ETag: &etag}}, nil
}

func (a *API) GetPortfolio(ctx context.Context, request api.GetPortfolioRequestObject) (api.GetPortfolioResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	return api.GetPortfolio200JSONResponse{Body: toAPIPortfolio(&sc.portfolio), Headers: api.GetPortfolio200ResponseHeaders{ETag: new(sc.portfolio.ETag())}}, nil
}

func (a *API) PatchPortfolio(ctx context.Context, request api.PatchPortfolioRequestObject) (api.PatchPortfolioResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a portfolio ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	in := portfolio.PatchInput{Name: request.Body.Name, Timezone: request.Body.Timezone, CollectionIntervalSeconds: request.Body.CollectionIntervalSeconds}
	item, err := a.Portfolios.Patch(ctx, sc.user.ID, request.PortfolioId, version, in)
	if err != nil {
		return nil, err
	}
	return api.PatchPortfolio200JSONResponse{Body: toAPIPortfolio(&item), Headers: api.PatchPortfolio200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) DeletePortfolio(ctx context.Context, request api.DeletePortfolioRequestObject) (api.DeletePortfolioResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	if err := a.Portfolios.Delete(ctx, sc.user.ID, request.PortfolioId, version); err != nil {
		return nil, err
	}
	return api.DeletePortfolio202Response{}, nil
}
