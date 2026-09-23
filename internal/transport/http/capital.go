package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/idempotency"
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
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Capital == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, sc.user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return nil, problemf(http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.CapitalFlow
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		return api.CreateCapitalFlow201JSONResponse{Body: body, Headers: api.CreateCapitalFlow201ResponseHeaders{ETag: &etag}}, nil
	}
	direction := string(request.Body.Direction)
	effectiveAt := request.Body.EffectiveAt.UTC()
	in := capital.Input{
		ActorUserID: sc.user.ID,
		RequestID:   requestTraceID(ctx),
		Direction:   &direction,
		AssetSymbol: &request.Body.AssetSymbol,
		Quantity:    &request.Body.Quantity,
		ValueUSD:    request.Body.ValueUsd,
		EffectiveAt: &effectiveAt,
		AccountID:   request.Body.AccountId,
		Note:        request.Body.Note,
	}
	item, err := a.Capital.Create(ctx, sc.portfolio.ID, &in)
	if err != nil {
		return nil, err
	}
	body := toAPIFlow(&item)
	etag := item.ETag()
	a.storeIdempotency(ctx, sc.user.ID, &sc.portfolio.ID, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreateCapitalFlow201JSONResponse{Body: body, Headers: api.CreateCapitalFlow201ResponseHeaders{ETag: etagPtr(etag)}}, nil
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
	return api.GetCapitalFlow200JSONResponse{Body: toAPIFlow(&item), Headers: api.GetCapitalFlow200ResponseHeaders{ETag: etagPtr(item.ETag())}}, nil
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
		ActorUserID:  sc.user.ID,
		RequestID:    requestTraceID(ctx),
		AssetSymbol:  request.Body.AssetSymbol,
		Quantity:     request.Body.Quantity,
		ValueUSD:     request.Body.ValueUsd,
		AccountID:    request.Body.AccountId,
		Note:         request.Body.Note,
		ClearAccount: nulls["accountId"],
		ClearValue:   nulls["valueUsd"],
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
	return api.PatchCapitalFlow200JSONResponse{Body: toAPIFlow(&item), Headers: api.PatchCapitalFlow200ResponseHeaders{ETag: etagPtr(item.ETag())}}, nil
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
