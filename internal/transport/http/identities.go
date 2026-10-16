package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/application/auth"

	"github.com/rbee3u/plutus/internal/application/identity"
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
	if a.Identities == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	body, err := createIdempotent(ctx, a, request.Params.XCSRFToken, stringValue(request.Params.IdempotencyKey), request.Body, http.StatusCreated, func(ctx context.Context, user auth.User) (api.Identity, error) {
		sc, err := a.scoped(ctx, request.PortfolioId)
		if err != nil {
			return api.Identity{}, err
		}
		types := make([]adapters.AccountType, 0, len(request.Body.SelectedAccountTypes))
		for _, kind := range request.Body.SelectedAccountTypes {
			types = append(types, adapters.AccountType(kind))
		}
		item, err := a.Identities.Create(ctx, sc.portfolio.ID, &identity.CreateInput{ActorUserID: user.ID, RequestID: requestTraceID(ctx), Platform: adapters.Platform(request.Body.Platform), DisplayName: request.Body.DisplayName, AccountStructure: adapters.AccountStructure(request.Body.AccountStructure), SelectedAccountTypes: types})
		if err != nil {
			return api.Identity{}, err
		}
		return toAPIIdentity(&item), nil
	})
	if err != nil {
		return nil, err
	}
	return api.CreateIdentity201JSONResponse(body), nil
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
	return api.GetIdentity200JSONResponse(toAPIIdentity(&item)), nil
}

func (a *API) PatchIdentity(ctx context.Context, request api.PatchIdentityRequestObject) (api.PatchIdentityResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	in := identity.PatchInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx), DisplayName: request.Body.DisplayName}
	if request.Body.Accounts != nil {
		for _, patch := range *request.Body.Accounts {
			in.Accounts = append(in.Accounts, identity.AccountPatch{ID: patch.Id, Selected: patch.Selected})
		}
	}
	item, err := a.Identities.Patch(ctx, sc.portfolio.ID, request.IdentityId, version, in)
	if err != nil {
		return nil, err
	}
	return api.PatchIdentity200JSONResponse(toAPIIdentity(&item)), nil
}

func (a *API) ArchiveIdentity(ctx context.Context, request api.ArchiveIdentityRequestObject) (api.ArchiveIdentityResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	item, err := a.Identities.Archive(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.ArchiveIdentity200JSONResponse(toAPIIdentity(&item)), nil
}

func (a *API) PutIdentityCredentials(ctx context.Context, request api.PutIdentityCredentialsRequestObject) (api.PutIdentityCredentialsResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	creds := adapters.Credentials{APIKey: request.Body.ApiKey, APISecret: request.Body.ApiSecret, Passphrase: stringValue(request.Body.Passphrase), Futures: &adapters.KeyPair{APIKey: stringValue(request.Body.FuturesApiKey), APISecret: stringValue(request.Body.FuturesApiSecret)}}
	item, err := a.Identities.PutCredentials(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)}, creds)
	if err != nil {
		return nil, err
	}
	return api.PutIdentityCredentials200JSONResponse(toAPIIdentity(&item)), nil
}

func (a *API) DeleteIdentityCredentials(ctx context.Context, request api.DeleteIdentityCredentialsRequestObject) (api.DeleteIdentityCredentialsResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	item, err := a.Identities.ClearCredentials(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DeleteIdentityCredentials200JSONResponse(toAPIIdentity(&item)), nil
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
	return api.ValidateIdentityConnection200JSONResponse(toAPIOutcome(&item, &outcome)), nil
}

func (a *API) EnableIdentityConnection(ctx context.Context, request api.EnableIdentityConnectionRequestObject) (api.EnableIdentityConnectionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	item, outcome, err := a.Identities.Enable(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if errors.Is(err, identity.ErrValidationFailed) {
		problem := a.problem(ctx, http.StatusUnprocessableEntity, "connection_validation_failed", "Connection validation failed", "The connection could not be enabled.")
		problem.ValidationOutcome = new(toAPIOutcome(&item, &outcome))
		return api.EnableIdentityConnection422ApplicationProblemPlusJSONResponse(problem), nil
	}
	if err != nil {
		return nil, err
	}
	return api.EnableIdentityConnection200JSONResponse(toAPIOutcome(&item, &outcome)), nil
}

func (a *API) DisableIdentityConnection(ctx context.Context, request api.DisableIdentityConnectionRequestObject) (api.DisableIdentityConnectionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Identities == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid edit version.")
	}
	item, err := a.Identities.Disable(ctx, sc.portfolio.ID, request.IdentityId, version, identity.ActionInput{ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DisableIdentityConnection200JSONResponse(toAPIIdentity(&item)), nil
}
