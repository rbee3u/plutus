package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/rbee3u/plutus/internal/application/auth"

	"github.com/rbee3u/plutus/internal/application/capital"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListCapitalFlows(ctx context.Context, request api.ListCapitalFlowsRequestObject) (api.ListCapitalFlowsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	var from, to time.Time
	if request.Params.From != nil {
		from = request.Params.From.UTC()
	}
	if request.Params.To != nil {
		to = request.Params.To.UTC()
	}
	flows, err := a.Capital.List(ctx, sc.portfolio.ID, from, to)
	if err != nil {
		return nil, err
	}
	items := make([]api.CapitalFlow, 0, len(flows))
	for i := range flows {
		items = append(items, toAPIFlow(&flows[i]))
	}
	return api.ListCapitalFlows200JSONResponse{Items: items}, nil
}

func (a *API) CreateCapitalFlow(ctx context.Context, request api.CreateCapitalFlowRequestObject) (api.CreateCapitalFlowResponseObject, error) {
	if a.Capital == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	body, etag, err := createIdempotent(ctx, a, request.Params.XCSRFToken, stringValue(request.Params.IdempotencyKey), request.Body, http.StatusCreated, func(ctx context.Context, user auth.User) (api.CapitalFlow, string, error) {
		sc, err := a.scoped(ctx, request.PortfolioId)
		if err != nil {
			return api.CapitalFlow{}, "", err
		}
		direction := string(request.Body.Direction)
		at := request.Body.EffectiveAt.UTC()
		input := capital.Input{ActorUserID: user.ID, RequestID: requestTraceID(ctx), Direction: &direction, CanonicalAssetSymbol: &request.Body.CanonicalAssetSymbol, Quantity: &request.Body.Quantity, ValueUSD: request.Body.ValueUsd, EffectiveAt: &at, AccountID: request.Body.AccountId, Note: request.Body.Note, PricingPlatform: request.Body.PricingPlatform}
		item, err := a.Capital.Create(ctx, sc.portfolio.ID, &input)
		if err != nil {
			return api.CapitalFlow{}, "", err
		}
		return toAPIFlow(&item), item.ETag(), nil
	})
	if err != nil {
		return nil, err
	}
	return api.CreateCapitalFlow201JSONResponse{Body: body, Headers: api.CreateCapitalFlow201ResponseHeaders{ETag: &etag}}, nil
}

func (a *API) GetCapitalFlow(ctx context.Context, request api.GetCapitalFlowRequestObject) (api.GetCapitalFlowResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Capital.Get(ctx, sc.portfolio.ID, request.FlowId)
	if err != nil {
		return nil, err
	}
	return api.GetCapitalFlow200JSONResponse{Body: toAPIFlow(&item), Headers: api.GetCapitalFlow200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) PatchCapitalFlow(ctx context.Context, request api.PatchCapitalFlowRequestObject) (api.PatchCapitalFlowResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	version, ok := capital.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a capital flow ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	nulls := explicitNulls(ctx)
	in := capital.Input{
		ActorUserID:          sc.user.ID,
		RequestID:            requestTraceID(ctx),
		CanonicalAssetSymbol: request.Body.CanonicalAssetSymbol,
		Quantity:             request.Body.Quantity,
		ValueUSD:             request.Body.ValueUsd,
		AccountID:            request.Body.AccountId,
		Note:                 request.Body.Note,
		ClearAccount:         nulls["accountId"],
		PricingPlatform:      request.Body.PricingPlatform,
		ClearValue:           nulls["valueUsd"],
		ClearPricingPlatform: nulls["pricingPlatform"],
		ClearNote:            nulls["note"],
	}
	if request.Body.Direction != nil {
		direction := string(*request.Body.Direction)
		in.Direction = &direction
	}
	if request.Body.EffectiveAt != nil {
		at := request.Body.EffectiveAt.UTC()
		in.EffectiveAt = &at
	}
	item, err := a.Capital.Patch(ctx, sc.portfolio.ID, request.FlowId, version, &in)
	if err != nil {
		return nil, err
	}
	return api.PatchCapitalFlow200JSONResponse{Body: toAPIFlow(&item), Headers: api.PatchCapitalFlow200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) DeleteCapitalFlow(ctx context.Context, request api.DeleteCapitalFlowRequestObject) (api.DeleteCapitalFlowResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	version, ok := capital.ParseETag(request.Params.IfMatch)
	if !ok {
		return nil, errPrecondition
	}
	if err := a.Capital.Delete(ctx, sc.portfolio.ID, request.FlowId, version, sc.user.ID, requestTraceID(ctx)); err != nil {
		return nil, err
	}
	return api.DeleteCapitalFlow204Response{}, nil
}
