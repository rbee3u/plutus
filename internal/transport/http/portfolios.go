package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/domain/portfolio"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
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
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return nil, errUnauthenticated
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if a.Portfolios == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return nil, problemf(http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.Portfolio
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		return api.CreatePortfolio201JSONResponse{Body: body, Headers: api.CreatePortfolio201ResponseHeaders{ETag: &etag}}, nil
	}
	in := portfolio.CreateInput{Name: request.Body.Name, Timezone: stringValue(request.Body.Timezone), CollectionIntervalSeconds: request.Body.CollectionIntervalSeconds}
	item, err := a.Portfolios.Create(ctx, user.ID, in)
	if err != nil {
		return nil, err
	}
	body := toAPIPortfolio(&item)
	etag := item.ETag()
	a.storeIdempotency(ctx, user.ID, &item.ID, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreatePortfolio201JSONResponse{Body: body, Headers: api.CreatePortfolio201ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) GetPortfolio(ctx context.Context, request api.GetPortfolioRequestObject) (api.GetPortfolioResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	return api.GetPortfolio200JSONResponse{Body: toAPIPortfolio(&sc.portfolio), Headers: api.GetPortfolio200ResponseHeaders{ETag: etagPtr(sc.portfolio.ETag())}}, nil
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
	return api.PatchPortfolio200JSONResponse{Body: toAPIPortfolio(&item), Headers: api.PatchPortfolio200ResponseHeaders{ETag: etagPtr(item.ETag())}}, nil
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
	return api.DeletePortfolio204Response{}, nil
}
