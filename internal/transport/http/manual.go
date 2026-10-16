package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) GetManualBalances(ctx context.Context, request api.GetManualBalancesRequestObject) (api.GetManualBalancesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Manual == nil {
		return nil, ErrNotImplemented
	}
	state, err := a.Manual.Get(ctx, sc.portfolio.ID, request.IdentityId)
	if err != nil {
		return nil, err
	}
	return api.GetManualBalances200JSONResponse(toAPIManual(state)), nil
}

func (a *API) CreateManualBalance(ctx context.Context, request api.CreateManualBalanceRequestObject) (api.CreateManualBalanceResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Manual == nil {
		return nil, ErrNotImplemented
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid manual ledger version.")
	}
	state, err := a.Manual.Create(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{
		AssetSymbol: request.Body.AssetSymbol,
		Quantity:    request.Body.Quantity,
		Version:     version,
		ActorUserID: sc.user.ID,
		RequestID:   requestTraceID(ctx),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateManualBalance200JSONResponse(toAPIManual(state)), nil
}

func (a *API) UpdateManualBalance(ctx context.Context, request api.UpdateManualBalanceRequestObject) (api.UpdateManualBalanceResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Manual == nil {
		return nil, ErrNotImplemented
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid manual ledger version.")
	}
	state, err := a.Manual.Update(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{
		AssetSymbol: request.AssetSymbol,
		Quantity:    request.Body.Quantity,
		Version:     version,
		ActorUserID: sc.user.ID,
		RequestID:   requestTraceID(ctx),
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateManualBalance200JSONResponse(toAPIManual(state)), nil
}

func (a *API) DeleteManualBalance(ctx context.Context, request api.DeleteManualBalanceRequestObject) (api.DeleteManualBalanceResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Manual == nil {
		return nil, ErrNotImplemented
	}
	version, ok := parseEditVersion(request.Params.IfEditVersion, false)
	if !ok {
		return nil, validationError("If-Edit-Version must be a valid manual ledger version.")
	}
	state, err := a.Manual.Delete(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{AssetSymbol: request.AssetSymbol, Version: version, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DeleteManualBalance200JSONResponse(toAPIManual(state)), nil
}
