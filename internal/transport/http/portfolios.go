package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListPortfolios(ctx context.Context, request api.ListPortfoliosRequestObject) (api.ListPortfoliosResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListPortfolios401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if a.Portfolios == nil {
		return nil, ErrNotImplemented
	}
	include := false
	if request.Params.IncludeArchived != nil {
		include = *request.Params.IncludeArchived
	}
	items, err := a.Portfolios.List(ctx, user.ID, include)
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
		return api.CreatePortfolio401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
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
		return api.CreatePortfolio409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
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
	in := portfolio.CreateInput{Name: request.Body.Name}
	if request.Body.Timezone != nil {
		in.Timezone = *request.Body.Timezone
	}
	if request.Body.DisplayAssetId != nil {
		in.DisplayAssetID = request.Body.DisplayAssetId
	}
	item, err := a.Portfolios.Create(ctx, user.ID, in)
	if err != nil {
		if resp := a.createPortfolioError(ctx, err); resp != nil {
			return resp, nil
		}
		return nil, err
	}
	body := toAPIPortfolio(&item)
	etag := item.ETag()
	a.storeIdempotency(ctx, user.ID, &item.ID, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreatePortfolio201JSONResponse{Body: body, Headers: api.CreatePortfolio201ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) createPortfolioError(ctx context.Context, err error) api.CreatePortfolioResponseObject {
	switch {
	case errors.Is(err, portfolio.ErrConflict):
		return api.CreatePortfolio409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error()))
	case errors.Is(err, portfolio.ErrInvalidName), errors.Is(err, portfolio.ErrInvalidTimezone), errors.Is(err, portfolio.ErrInvalidDisplayAsset):
		return api.CreatePortfolio400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}
	default:
		return nil
	}
}

func (a *API) GetPortfolio(ctx context.Context, request api.GetPortfolioRequestObject) (api.GetPortfolioResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetPortfolio401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	item, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId)
	if !ok {
		return api.GetPortfolio404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	etag := item.ETag()
	return api.GetPortfolio200JSONResponse{Body: toAPIPortfolio(&item), Headers: api.GetPortfolio200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) PatchPortfolio(ctx context.Context, request api.PatchPortfolioRequestObject) (api.PatchPortfolioResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PatchPortfolio401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return api.PatchPortfolio400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "If-Match must be a portfolio ETag."))}, nil
	}
	if request.Body == nil {
		return api.PatchPortfolio400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "A request body is required."))}, nil
	}
	in := portfolio.PatchInput{Name: request.Body.Name, Timezone: request.Body.Timezone}
	if request.Body.DisplayAssetId != nil {
		in.DisplayAssetID = request.Body.DisplayAssetId
	}
	item, err := a.Portfolios.Patch(ctx, user.ID, request.PortfolioId, version, in)
	return a.mutatePortfolio(ctx, &item, err)
}

func (a *API) mutatePortfolio(ctx context.Context, item *portfolio.Portfolio, err error) (api.PatchPortfolioResponseObject, error) {
	switch {
	case errors.Is(err, portfolio.ErrNotFound):
		return api.PatchPortfolio404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, portfolio.ErrPrecondition):
		return api.PatchPortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, portfolio.ErrConflict):
		return api.PatchPortfolio400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case errors.Is(err, portfolio.ErrInvalidName), errors.Is(err, portfolio.ErrInvalidTimezone), errors.Is(err, portfolio.ErrInvalidDisplayAsset):
		return api.PatchPortfolio400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	default:
		etag := item.ETag()
		return api.PatchPortfolio200JSONResponse{Body: toAPIPortfolio(item), Headers: api.PatchPortfolio200ResponseHeaders{ETag: etagPtr(etag)}}, nil
	}
}

func (a *API) DeletePortfolio(ctx context.Context, request api.DeletePortfolioRequestObject) (api.DeletePortfolioResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.DeletePortfolio401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return api.DeletePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	err := a.Portfolios.Delete(ctx, user.ID, request.PortfolioId, version)
	switch {
	case errors.Is(err, portfolio.ErrNotFound):
		return api.DeletePortfolio404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, portfolio.ErrPrecondition):
		return api.DeletePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, portfolio.ErrNotEmpty):
		return api.DeletePortfolio409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "not_empty", "Portfolio is not empty", "Only an empty portfolio can be deleted.")), nil
	case err != nil:
		return nil, err
	default:
		return api.DeletePortfolio204Response{}, nil
	}
}

func (a *API) ArchivePortfolio(ctx context.Context, request api.ArchivePortfolioRequestObject) (api.ArchivePortfolioResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.ArchivePortfolio401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return api.ArchivePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	item, err := a.Portfolios.Archive(ctx, user.ID, request.PortfolioId, version)
	switch {
	case errors.Is(err, portfolio.ErrNotFound):
		return api.ArchivePortfolio404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, portfolio.ErrPrecondition):
		return api.ArchivePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case err != nil:
		return nil, err
	default:
		return api.ArchivePortfolio200JSONResponse(toAPIPortfolio(&item)), nil
	}
}

func (a *API) RestorePortfolio(ctx context.Context, request api.RestorePortfolioRequestObject) (api.RestorePortfolioResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.RestorePortfolio401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	version, ok := portfolio.ParseETag(request.Params.IfMatch)
	if !ok {
		return api.RestorePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	item, err := a.Portfolios.Restore(ctx, user.ID, request.PortfolioId, version)
	switch {
	case errors.Is(err, portfolio.ErrNotFound):
		return api.RestorePortfolio404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, portfolio.ErrPrecondition):
		return api.RestorePortfolio412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case err != nil:
		return nil, err
	default:
		return api.RestorePortfolio200JSONResponse(toAPIPortfolio(&item)), nil
	}
}

func (a *API) loadPortfolio(ctx context.Context, userID, portfolioID uuid.UUID) (portfolio.Portfolio, error) {
	if a.Portfolios == nil {
		return portfolio.Portfolio{}, ErrNotImplemented
	}
	return a.Portfolios.Get(ctx, userID, portfolioID)
}

func (a *API) ownedPortfolio(ctx context.Context, userID, portfolioID uuid.UUID) (portfolio.Portfolio, bool) {
	item, err := a.loadPortfolio(ctx, userID, portfolioID)
	if err != nil {
		return portfolio.Portfolio{}, false
	}
	return item, true
}

func (a *API) notFound(ctx context.Context) api.Problem {
	return a.problem(ctx, http.StatusNotFound, "not_found", "Not found", "The requested resource was not found.")
}

func toAPIPortfolio(item *portfolio.Portfolio) api.Portfolio {
	return api.Portfolio{
		ArchivedAt:   item.ArchivedAt,
		CreatedAt:    item.CreatedAt,
		DisplayAsset: api.AssetRef{Id: item.DisplayAssetID, Code: item.DisplayAsset},
		Id:           item.ID,
		Name:         item.Name,
		Status:       api.PortfolioStatus(item.Status),
		Timezone:     item.Timezone,
		UpdatedAt:    item.UpdatedAt,
		Version:      int(item.Version),
	}
}
