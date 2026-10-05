package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/domain/manual"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIManual(state manual.State) api.ManualBalanceState {
	convert := func(items []manual.Balance) []api.ManualBalance {
		rows := make([]api.ManualBalance, 0, len(items))
		for _, item := range items {
			rows = append(rows, api.ManualBalance{Symbol: item.Symbol, ListingId: item.ListingID, AssetSymbol: item.AssetSymbol, Quantity: item.Quantity, ModifiedAt: item.ModifiedAt})
		}
		return rows
	}
	return api.ManualBalanceState{AccountId: state.AccountID, IdentityId: state.IdentityID, Version: state.Version, AppliedVersion: state.AppliedVersion, Items: convert(state.Items), AppliedItems: convert(state.AppliedItems), Pending: state.Pending}
}

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
	return api.GetManualBalances200JSONResponse{Body: toAPIManual(state), Headers: api.GetManualBalances200ResponseHeaders{ETag: etagPtr(state.ETag())}}, nil
}

func manualVersion(etag string) (uint64, error) {
	version, ok := manual.ParseETag(etag)
	if !ok {
		return 0, validationError("If-Match must be a manual ledger ETag.")
	}
	return version, nil
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
	version, err := manualVersion(request.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	state, err := a.Manual.Create(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{Symbol: request.Body.Symbol, Quantity: request.Body.Quantity, Version: version, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.CreateManualBalance200JSONResponse{Body: toAPIManual(state), Headers: api.CreateManualBalance200ResponseHeaders{ETag: etagPtr(state.ETag())}}, nil
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
	version, err := manualVersion(request.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	state, err := a.Manual.Update(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{Symbol: request.Symbol, Quantity: request.Body.Quantity, Version: version, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.UpdateManualBalance200JSONResponse{Body: toAPIManual(state), Headers: api.UpdateManualBalance200ResponseHeaders{ETag: etagPtr(state.ETag())}}, nil
}

func (a *API) DeleteManualBalance(ctx context.Context, request api.DeleteManualBalanceRequestObject) (api.DeleteManualBalanceResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Manual == nil {
		return nil, ErrNotImplemented
	}
	version, err := manualVersion(request.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	state, err := a.Manual.Delete(ctx, sc.portfolio.ID, request.IdentityId, manual.Input{Symbol: request.Symbol, Version: version, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx)})
	if err != nil {
		return nil, err
	}
	return api.DeleteManualBalance200JSONResponse{Body: toAPIManual(state), Headers: api.DeleteManualBalance200ResponseHeaders{ETag: etagPtr(state.ETag())}}, nil
}
