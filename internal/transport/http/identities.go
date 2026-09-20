package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListIdentities(ctx context.Context, request api.ListIdentitiesRequestObject) (api.ListIdentitiesResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListIdentities401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListIdentities404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	items, err := a.Access.ListIdentities(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	out := make([]api.Identity, 0, len(items))
	for i := range items {
		out = append(out, toAPIIdentity(&items[i]))
	}
	return api.ListIdentities200JSONResponse{Items: out}, nil
}

func (a *API) CreateIdentity(ctx context.Context, request api.CreateIdentityRequestObject) (api.CreateIdentityResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.CreateIdentity401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.CreateIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Access == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.CreateIdentity409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.Identity
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		return api.CreateIdentity201JSONResponse{Body: body, Headers: api.CreateIdentity201ResponseHeaders{ETag: &etag}}, nil
	}
	in := access.CreateIdentityInput{
		PlatformID:  request.Body.PlatformId,
		ExternalRef: request.Body.ExternalRef,
		DisplayName: request.Body.DisplayName,
		AccountMode: string(request.Body.AccountMode),
		Kind:        string(request.Body.Kind),
	}
	if request.Body.ParentIdentityId != nil {
		in.ParentIdentityID = request.Body.ParentIdentityId
	}
	item, err := a.Access.CreateIdentity(ctx, request.PortfolioId, &in)
	if err != nil {
		return a.createIdentityError(ctx, err), nil
	}
	body := toAPIIdentity(&item)
	etag := item.ETag()
	id := item.PortfolioID
	a.storeIdempotency(ctx, user.ID, &id, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreateIdentity201JSONResponse{Body: body, Headers: api.CreateIdentity201ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) createIdentityError(ctx context.Context, err error) api.CreateIdentityResponseObject {
	switch {
	case errors.Is(err, access.ErrConflict):
		return api.CreateIdentity409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error()))
	case errors.Is(err, access.ErrNotFound):
		return api.CreateIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx))
	default:
		return api.CreateIdentity400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}
	}
}

func (a *API) GetIdentity(ctx context.Context, request api.GetIdentityRequestObject) (api.GetIdentityResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetIdentity401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	item, ok := a.ownedIdentity(ctx, request.PortfolioId, request.IdentityId)
	if !ok {
		return api.GetIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	etag := item.ETag()
	return api.GetIdentity200JSONResponse{Body: toAPIIdentity(&item), Headers: api.GetIdentity200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) PatchIdentity(ctx context.Context, request api.PatchIdentityRequestObject) (api.PatchIdentityResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PatchIdentity401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PatchIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	version, ok := access.ParseIdentityETag(request.Params.IfMatch)
	if !ok {
		return api.PatchIdentity412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	if request.Body == nil {
		return api.PatchIdentity400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "A request body is required."))}, nil
	}
	in := access.PatchIdentityInput{DisplayName: request.Body.DisplayName, ExternalRef: request.Body.ExternalRef}
	if request.Body.Status != nil {
		status := string(*request.Body.Status)
		in.Status = &status
	}
	if request.Body.AccountMode != nil {
		mode := string(*request.Body.AccountMode)
		in.AccountMode = &mode
	}
	if request.Body.ParentIdentityId != nil {
		in.ParentIdentityID = request.Body.ParentIdentityId
	}
	item, err := a.Access.PatchIdentity(ctx, request.PortfolioId, request.IdentityId, version, in)
	switch {
	case errors.Is(err, access.ErrNotFound):
		return api.PatchIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, access.ErrPrecondition):
		return api.PatchIdentity412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, access.ErrIdentityFrozen):
		return api.PatchIdentity409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "identity_frozen", "Identity is frozen", err.Error())), nil
	case err != nil:
		return api.PatchIdentity400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	default:
		return api.PatchIdentity200JSONResponse(toAPIIdentity(&item)), nil
	}
}

func (a *API) ArchiveIdentity(ctx context.Context, request api.ArchiveIdentityRequestObject) (api.ArchiveIdentityResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.ArchiveIdentity401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ArchiveIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	version, ok := access.ParseIdentityETag(request.Params.IfMatch)
	if !ok {
		return api.ArchiveIdentity412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	item, err := a.Access.ArchiveIdentity(ctx, request.PortfolioId, request.IdentityId, version)
	switch {
	case errors.Is(err, access.ErrNotFound):
		return api.ArchiveIdentity404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, access.ErrPrecondition):
		return api.ArchiveIdentity412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case err != nil:
		return nil, err
	default:
		return api.ArchiveIdentity200JSONResponse(toAPIIdentity(&item)), nil
	}
}

func toAPIIdentity(item *access.Identity) api.Identity {
	out := api.Identity{
		AccountMode: api.IdentityAccountMode(item.AccountMode),
		DisplayName: item.DisplayName,
		ExternalRef: item.ExternalRef,
		Id:          item.ID,
		Kind:        api.IdentityKind(item.Kind),
		PlatformId:  item.PlatformID,
		PortfolioId: item.PortfolioID,
		Status:      api.IdentityStatus(item.Status),
		Version:     int(item.Version),
	}
	if item.ParentIdentityID != nil {
		out.ParentIdentityId = item.ParentIdentityID
	}
	return out
}

func (a *API) ownedIdentity(ctx context.Context, portfolioID, identityID uuid.UUID) (access.Identity, bool) {
	if a.Access == nil {
		return access.Identity{}, false
	}
	item, err := a.Access.GetIdentity(ctx, portfolioID, identityID)
	if err != nil {
		return access.Identity{}, false
	}
	return item, true
}
