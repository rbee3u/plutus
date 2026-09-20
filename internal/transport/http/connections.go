package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/secret"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListConnections(ctx context.Context, request api.ListConnectionsRequestObject) (api.ListConnectionsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListConnections401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListConnections404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	items, err := a.Access.ListConnections(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	out := make([]api.Connection, 0, len(items))
	for i := range items {
		out = append(out, toAPIConnection(&items[i]))
	}
	return api.ListConnections200JSONResponse{Items: out}, nil
}

func (a *API) CreateConnection(ctx context.Context, request api.CreateConnectionRequestObject) (api.CreateConnectionResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.CreateConnection401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.CreateConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
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
		return api.CreateConnection409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.Connection
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		return api.CreateConnection201JSONResponse(body), nil
	}
	in := access.CreateConnectionInput{
		IdentityID:                request.Body.IdentityId,
		Label:                     request.Body.Label,
		CollectionIntervalSeconds: request.Body.CollectionIntervalSeconds,
		FreshnessAfterSeconds:     request.Body.FreshnessAfterSeconds,
		ActorUserID:               user.ID,
		RequestID:                 requestTraceID(ctx),
	}
	if request.Body.Status != nil {
		in.Status = string(*request.Body.Status)
	}
	creds, err := credentialsFromAPI(request.Body.Credentials)
	if err != nil {
		return api.CreateConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	}
	in.Credentials = creds
	item, err := a.Access.CreateConnection(ctx, request.PortfolioId, &in)
	if err != nil {
		return a.createConnectionError(ctx, err), nil
	}
	body := toAPIConnection(&item)
	id := item.PortfolioID
	a.storeIdempotency(ctx, user.ID, &id, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, nil, body)
	return api.CreateConnection201JSONResponse(body), nil
}

func (a *API) createConnectionError(ctx context.Context, err error) api.CreateConnectionResponseObject {
	switch {
	case errors.Is(err, access.ErrConflict):
		return api.CreateConnection409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error()))
	case errors.Is(err, access.ErrNotFound):
		return api.CreateConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx))
	case errors.Is(err, access.ErrMasterKeyRequired):
		return api.CreateConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "master_key_required", "Master key required", "Connection credentials cannot be stored without a configured master key."))}
	default:
		return api.CreateConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}
	}
}

func (a *API) GetConnection(ctx context.Context, request api.GetConnectionRequestObject) (api.GetConnectionResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetConnection401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	item, ok := a.ownedConnection(ctx, request.PortfolioId, request.ConnectionId)
	if !ok {
		return api.GetConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	return api.GetConnection200JSONResponse(toAPIConnection(&item)), nil
}

func (a *API) PatchConnection(ctx context.Context, request api.PatchConnectionRequestObject) (api.PatchConnectionResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PatchConnection401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PatchConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	version, ok := access.ParseConnectionETag(request.Params.IfMatch)
	if !ok {
		return api.PatchConnection412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	if request.Body == nil {
		return api.PatchConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "A request body is required."))}, nil
	}
	in := access.PatchConnectionInput{
		Label:                     request.Body.Label,
		CollectionIntervalSeconds: request.Body.CollectionIntervalSeconds,
		FreshnessAfterSeconds:     request.Body.FreshnessAfterSeconds,
		ActorUserID:               user.ID,
		RequestID:                 requestTraceID(ctx),
	}
	if request.Body.Status != nil {
		status := string(*request.Body.Status)
		in.Status = &status
	}
	creds, err := credentialsFromAPI(request.Body.Credentials)
	if err != nil {
		return api.PatchConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	}
	in.Credentials = creds
	item, err := a.Access.PatchConnection(ctx, request.PortfolioId, request.ConnectionId, version, in)
	switch {
	case errors.Is(err, access.ErrNotFound):
		return api.PatchConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, access.ErrPrecondition):
		return api.PatchConnection412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, access.ErrMasterKeyRequired):
		return api.PatchConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "master_key_required", "Master key required", "Connection credentials cannot be stored without a configured master key."))}, nil
	case err != nil:
		return api.PatchConnection400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	default:
		return api.PatchConnection200JSONResponse(toAPIConnection(&item)), nil
	}
}

func (a *API) DeleteConnection(ctx context.Context, request api.DeleteConnectionRequestObject) (api.DeleteConnectionResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.DeleteConnection401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.DeleteConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	version, ok := access.ParseConnectionETag(request.Params.IfMatch)
	if !ok {
		return api.DeleteConnection412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	err := a.Access.DeleteConnection(ctx, request.PortfolioId, request.ConnectionId, version, user.ID, requestTraceID(ctx))
	switch {
	case errors.Is(err, access.ErrNotFound):
		return api.DeleteConnection404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, access.ErrPrecondition):
		return api.DeleteConnection412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case err != nil:
		return nil, err
	default:
		return api.DeleteConnection204Response{}, nil
	}
}

func toAPIConnection(item *access.Connection) api.Connection {
	interval := item.CollectionIntervalSeconds
	fresh := item.FreshnessAfterSeconds
	return api.Connection{
		CollectionIntervalSeconds: &interval,
		CredentialsConfigured:     item.CredentialsConfigured,
		CredentialsUpdatedAt:      item.CredentialsUpdatedAt,
		FreshnessAfterSeconds:     &fresh,
		Id:                        item.ID,
		IdentityId:                item.IdentityID,
		Label:                     item.Label,
		LastValidatedAt:           item.LastValidatedAt,
		PortfolioId:               item.PortfolioID,
		Status:                    api.ConnectionStatus(item.Status),
		Version:                   int(item.Version),
	}
}

func (a *API) ownedConnection(ctx context.Context, portfolioID, connectionID uuid.UUID) (access.Connection, bool) {
	if a.Access == nil {
		return access.Connection{}, false
	}
	item, err := a.Access.GetConnection(ctx, portfolioID, connectionID)
	if err != nil {
		return access.Connection{}, false
	}
	return item, true
}

func credentialsFromAPI(in *api.ConnectionCredentials) (*secret.Credentials, error) {
	if in == nil {
		return nil, nil //nolint:nilnil // omitting credentials is valid
	}
	key := ""
	secretValue := ""
	passphrase := ""
	if in.ApiKey != nil {
		key = strings.TrimSpace(*in.ApiKey)
	}
	if in.ApiSecret != nil {
		secretValue = strings.TrimSpace(*in.ApiSecret)
	}
	if in.Passphrase != nil {
		passphrase = strings.TrimSpace(*in.Passphrase)
	}
	if key == "" || secretValue == "" {
		return nil, secret.ErrInvalidCredentials
	}
	return &secret.Credentials{APIKey: key, APISecret: secretValue, Passphrase: passphrase}, nil
}
