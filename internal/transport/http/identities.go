package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListIdentities(ctx context.Context, request api.ListIdentitiesRequestObject) (api.ListIdentitiesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	items, err := a.Identities.List(ctx, sc.portfolio.ID)
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
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := stringValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, sc.user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return nil, newProblemError(http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")
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
	types := make([]adapters.AccountType, 0, len(request.Body.SelectedAccountTypes))
	for _, t := range request.Body.SelectedAccountTypes {
		types = append(types, adapters.AccountType(t))
	}
	item, err := a.Identities.Create(ctx, sc.portfolio.ID, &identity.CreateInput{
		ActorUserID:          sc.user.ID,
		RequestID:            requestTraceID(ctx),
		Platform:             adapters.Platform(request.Body.Platform),
		DisplayName:          request.Body.DisplayName,
		AccountStructure:     adapters.AccountStructure(request.Body.AccountStructure),
		SelectedAccountTypes: types,
	})
	if err != nil {
		return nil, err
	}
	body := toAPIIdentity(&item)
	etag := item.ETag()
	a.storeIdempotency(ctx, sc.user.ID, &sc.portfolio.ID, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreateIdentity201JSONResponse{Body: body, Headers: api.CreateIdentity201ResponseHeaders{ETag: new(etag)}}, nil
}

func (a *API) GetIdentity(ctx context.Context, request api.GetIdentityRequestObject) (api.GetIdentityResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Identities.Get(ctx, sc.portfolio.ID, request.IdentityId)
	if err != nil {
		return nil, err
	}
	return api.GetIdentity200JSONResponse{Body: toAPIIdentity(&item), Headers: api.GetIdentity200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) PatchIdentity(ctx context.Context, request api.PatchIdentityRequestObject) (api.PatchIdentityResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be an identity ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	in := identity.PatchInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx), DisplayName: request.Body.DisplayName}
	if request.Body.Accounts != nil {
		for _, patch := range *request.Body.Accounts {
			in.Accounts = append(in.Accounts, identity.AccountPatch{ID: patch.Id, Selected: patch.Selected, DisplayName: patch.DisplayName})
		}
	}
	item, err := a.Identities.Patch(ctx, sc.portfolio.ID, request.IdentityId, version, in)
	if err != nil {
		return nil, err
	}
	return api.PatchIdentity200JSONResponse{Body: toAPIIdentity(&item), Headers: api.PatchIdentity200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) ArchiveIdentity(ctx context.Context, request api.ArchiveIdentityRequestObject) (api.ArchiveIdentityResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	item, err := a.Identities.Archive(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.ArchiveIdentity200JSONResponse{Body: toAPIIdentity(&item), Headers: api.ArchiveIdentity200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) PutIdentityCredentials(ctx context.Context, request api.PutIdentityCredentialsRequestObject) (api.PutIdentityCredentialsResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	creds := adapters.Credentials{APIKey: request.Body.ApiKey, APISecret: request.Body.ApiSecret, Passphrase: stringValue(request.Body.Passphrase), Futures: &adapters.KeyPair{APIKey: stringValue(request.Body.FuturesApiKey), APISecret: stringValue(request.Body.FuturesApiSecret)}}
	item, err := a.Identities.PutCredentials(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)}, creds)
	if err != nil {
		return nil, err
	}
	return api.PutIdentityCredentials200JSONResponse{Body: toAPIIdentity(&item), Headers: api.PutIdentityCredentials200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) DeleteIdentityCredentials(ctx context.Context, request api.DeleteIdentityCredentialsRequestObject) (api.DeleteIdentityCredentialsResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	item, err := a.Identities.ClearCredentials(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DeleteIdentityCredentials200JSONResponse{Body: toAPIIdentity(&item), Headers: api.DeleteIdentityCredentials200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) ValidateIdentityConnection(ctx context.Context, request api.ValidateIdentityConnectionRequestObject) (api.ValidateIdentityConnectionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	item, outcome, err := a.Identities.Validate(ctx, sc.portfolio.ID, request.IdentityId, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.ValidateIdentityConnection200JSONResponse{Body: toAPIOutcome(&item, &outcome), Headers: api.ValidateIdentityConnection200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) EnableIdentityConnection(ctx context.Context, request api.EnableIdentityConnectionRequestObject) (api.EnableIdentityConnectionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	item, outcome, err := a.Identities.Enable(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if errors.Is(err, identity.ErrValidationFailed) {
		return api.EnableIdentityConnection422JSONResponse{Body: toAPIOutcome(&item, &outcome), Headers: api.EnableIdentityConnection422ResponseHeaders{ETag: new(item.ETag())}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.EnableIdentityConnection200JSONResponse{Body: toAPIOutcome(&item, &outcome), Headers: api.EnableIdentityConnection200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) DisableIdentityConnection(ctx context.Context, request api.DisableIdentityConnectionRequestObject) (api.DisableIdentityConnectionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := identity.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	item, err := a.Identities.Disable(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DisableIdentityConnection200JSONResponse{Body: toAPIIdentity(&item), Headers: api.DisableIdentityConnection200ResponseHeaders{ETag: new(item.ETag())}}, nil
}
