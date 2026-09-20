package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) GetCapitalBaseline(ctx context.Context, request api.GetCapitalBaselineRequestObject) (api.GetCapitalBaselineResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetCapitalBaseline401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetCapitalBaseline404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Capital.GetBaseline(ctx, request.PortfolioId)
	if errors.Is(err, capital.ErrNotFound) {
		return api.GetCapitalBaseline404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.GetCapitalBaseline200JSONResponse{Body: toAPIBaseline(&item), Headers: api.GetCapitalBaseline200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) PutCapitalBaseline(ctx context.Context, request api.PutCapitalBaselineRequestObject) (api.PutCapitalBaselineResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PutCapitalBaseline401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PutCapitalBaseline404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil || a.Jobs == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPut, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.PutCapitalBaseline409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.CapitalBaselineMutation
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		if rec.Status == http.StatusCreated {
			return api.PutCapitalBaseline201JSONResponse{Body: body, Headers: api.PutCapitalBaseline201ResponseHeaders{ETag: &etag}}, nil
		}
		return api.PutCapitalBaseline200JSONResponse{Body: body, Headers: api.PutCapitalBaseline200ResponseHeaders{ETag: &etag}}, nil
	}
	item, created, err := a.Capital.PutBaseline(ctx, request.PortfolioId, &capital.PutBaselineInput{
		ActorUserID: user.ID,
		RequestID:   requestTraceID(ctx),
		EffectiveAt: request.Body.EffectiveAt,
		AssetID:     request.Body.AssetId,
		Quantity:    request.Body.Quantity,
		Note:        request.Body.Note,
		IfMatch:     request.Params.IfMatch,
	})
	switch {
	case errors.Is(err, capital.ErrNotFound):
		return api.PutCapitalBaseline404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, capital.ErrExists):
		return api.PutCapitalBaseline409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error())), nil
	case errors.Is(err, capital.ErrPrecondition):
		return api.PutCapitalBaseline412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, capital.ErrInvalid):
		return api.PutCapitalBaseline400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "capital baseline change")
	if err != nil {
		return nil, err
	}
	body := api.CapitalBaselineMutation{Baseline: toAPIBaseline(&item), Job: toAPIJob(&job, false)}
	etag := item.ETag()
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	a.storeIdempotency(ctx, user.ID, &request.PortfolioId, key, http.MethodPut, requestPath(ctx), hash, status, map[string]string{"ETag": etag}, body)
	if created {
		return api.PutCapitalBaseline201JSONResponse{Body: body, Headers: api.PutCapitalBaseline201ResponseHeaders{ETag: etagPtr(etag)}}, nil
	}
	return api.PutCapitalBaseline200JSONResponse{Body: body, Headers: api.PutCapitalBaseline200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) ListCapitalBaselineRevisions(ctx context.Context, request api.ListCapitalBaselineRevisionsRequestObject) (api.ListCapitalBaselineRevisionsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListCapitalBaselineRevisions401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListCapitalBaselineRevisions404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Capital.ListBaselineRevisions(ctx, request.PortfolioId, intValue(request.Params.Limit), stringValue(request.Params.Cursor))
	if err != nil {
		if errors.Is(err, capital.ErrNotFound) {
			return api.ListCapitalBaselineRevisions404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
		}
		return nil, err
	}
	items := make([]api.CapitalBaselineRevision, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIBaselineRevision(&page.Items[i]))
	}
	return api.ListCapitalBaselineRevisions200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func (a *API) ListCapitalFlows(ctx context.Context, request api.ListCapitalFlowsRequestObject) (api.ListCapitalFlowsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListCapitalFlows401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListCapitalFlows404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Capital.ListFlows(ctx, request.PortfolioId, intValue(request.Params.Offset), intValue(request.Params.Limit))
	if err != nil {
		return nil, err
	}
	items := make([]api.CapitalFlow, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIFlow(&page.Items[i]))
	}
	return api.ListCapitalFlows200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) CreateCapitalFlow(ctx context.Context, request api.CreateCapitalFlowRequestObject) (api.CreateCapitalFlowResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.CreateCapitalFlow401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.CreateCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil || a.Jobs == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.CreateCapitalFlow409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.CapitalFlowMutation
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		return api.CreateCapitalFlow201JSONResponse{Body: body, Headers: api.CreateCapitalFlow201ResponseHeaders{ETag: &etag}}, nil
	}
	in := &capital.CreateFlowInput{
		ActorUserID: user.ID,
		RequestID:   requestTraceID(ctx),
		Direction:   string(request.Body.Direction),
		Quantity:    request.Body.Quantity,
		AssetID:     request.Body.AssetId,
		EffectiveAt: request.Body.EffectiveAt,
		AccountID:   request.Body.AccountId,
		Note:        request.Body.Note,
	}
	item, err := a.Capital.CreateFlow(ctx, request.PortfolioId, in)
	if errors.Is(err, capital.ErrInvalid) {
		return api.CreateCapitalFlow400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	}
	if err != nil {
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "capital flow create")
	if err != nil {
		return nil, err
	}
	body := api.CapitalFlowMutation{Flow: toAPIFlow(&item), Job: toAPIJob(&job, false)}
	etag := item.ETag()
	a.storeIdempotency(ctx, user.ID, &request.PortfolioId, key, http.MethodPost, requestPath(ctx), hash, http.StatusCreated, map[string]string{"ETag": etag}, body)
	return api.CreateCapitalFlow201JSONResponse{Body: body, Headers: api.CreateCapitalFlow201ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) GetCapitalFlow(ctx context.Context, request api.GetCapitalFlowRequestObject) (api.GetCapitalFlowResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetCapitalFlow401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Capital.GetFlow(ctx, request.PortfolioId, request.FlowId)
	if errors.Is(err, capital.ErrNotFound) {
		return api.GetCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.GetCapitalFlow200JSONResponse{Body: toAPIFlow(&item), Headers: api.GetCapitalFlow200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) PatchCapitalFlow(ctx context.Context, request api.PatchCapitalFlowRequestObject) (api.PatchCapitalFlowResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PatchCapitalFlow401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PatchCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil || a.Jobs == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	version, ok := capital.ParseFlowETag(request.Params.IfMatch)
	if !ok {
		return api.PatchCapitalFlow412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	in := &capital.PatchFlowInput{
		ActorUserID: user.ID,
		RequestID:   requestTraceID(ctx),
		Quantity:    request.Body.Quantity,
		AssetID:     request.Body.AssetId,
		EffectiveAt: request.Body.EffectiveAt,
		Note:        request.Body.Note,
		AccountID:   request.Body.AccountId,
	}
	if request.Body.Direction != nil {
		value := string(*request.Body.Direction)
		in.Direction = &value
	}
	item, err := a.Capital.PatchFlow(ctx, request.PortfolioId, request.FlowId, version, in)
	switch {
	case errors.Is(err, capital.ErrNotFound):
		return api.PatchCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, capital.ErrPrecondition):
		return api.PatchCapitalFlow412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, capital.ErrInvalid):
		return api.PatchCapitalFlow400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "capital flow edit")
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.PatchCapitalFlow200JSONResponse{Body: api.CapitalFlowMutation{Flow: toAPIFlow(&item), Job: toAPIJob(&job, false)}, Headers: api.PatchCapitalFlow200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) DeleteCapitalFlow(ctx context.Context, request api.DeleteCapitalFlowRequestObject) (api.DeleteCapitalFlowResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.DeleteCapitalFlow401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.DeleteCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil || a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	version, ok := capital.ParseFlowETag(request.Params.IfMatch)
	if !ok {
		return api.DeleteCapitalFlow412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	item, err := a.Capital.DeleteFlow(ctx, request.PortfolioId, request.FlowId, version, user.ID, requestTraceID(ctx))
	switch {
	case errors.Is(err, capital.ErrNotFound):
		return api.DeleteCapitalFlow404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, capital.ErrPrecondition):
		return api.DeleteCapitalFlow412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "capital flow delete")
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.DeleteCapitalFlow200JSONResponse{Body: api.CapitalFlowMutation{Flow: toAPIFlow(&item), Job: toAPIJob(&job, false)}, Headers: api.DeleteCapitalFlow200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) ListCapitalFlowRevisions(ctx context.Context, request api.ListCapitalFlowRevisionsRequestObject) (api.ListCapitalFlowRevisionsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListCapitalFlowRevisions401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListCapitalFlowRevisions404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Capital == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Capital.ListFlowRevisions(ctx, request.PortfolioId, request.FlowId, intValue(request.Params.Limit), stringValue(request.Params.Cursor))
	if errors.Is(err, capital.ErrNotFound) {
		return api.ListCapitalFlowRevisions404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]api.CapitalFlowRevision, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIFlowRevision(&page.Items[i]))
	}
	return api.ListCapitalFlowRevisions200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func toAPIBaseline(item *capital.Baseline) api.CapitalBaseline {
	return api.CapitalBaseline{
		AssetId:           item.AssetID,
		CurrentRevisionId: item.CurrentRevisionID,
		EffectiveAt:       item.EffectiveAt,
		Id:                item.ID,
		Note:              item.Note,
		PortfolioId:       item.PortfolioID,
		Quantity:          item.Quantity,
		Version:           int(item.Version),
	}
}

func toAPIBaselineRevision(item *capital.BaselineRevision) api.CapitalBaselineRevision {
	return api.CapitalBaselineRevision{
		AssetId:         item.AssetID,
		CreatedAt:       item.CreatedAt,
		CreatedByUserId: item.CreatedByUserID,
		EffectiveAt:     item.EffectiveAt,
		Id:              item.ID,
		Note:            item.Note,
		Quantity:        item.Quantity,
	}
}

func toAPIFlow(item *capital.Flow) api.CapitalFlow {
	out := api.CapitalFlow{
		AssetId:           item.AssetID,
		CurrentRevisionId: item.CurrentRevisionID,
		Direction:         api.CapitalFlowDirection(item.Direction),
		EffectiveAt:       item.EffectiveAt,
		Id:                item.ID,
		Note:              item.Note,
		PortfolioId:       item.PortfolioID,
		Quantity:          item.Quantity,
		State:             api.CapitalFlowState(item.State),
		Version:           int(item.Version),
	}
	if item.AccountID != nil {
		out.AccountId = item.AccountID
	}
	return out
}

func toAPIFlowRevision(item *capital.FlowRevision) api.CapitalFlowRevision {
	out := api.CapitalFlowRevision{
		AssetId:         item.AssetID,
		CreatedAt:       item.CreatedAt,
		CreatedByUserId: item.CreatedByUserID,
		Direction:       api.CapitalFlowRevisionDirection(item.Direction),
		EffectiveAt:     item.EffectiveAt,
		Id:              item.ID,
		Note:            item.Note,
		Quantity:        item.Quantity,
		State:           api.CapitalFlowRevisionState(item.State),
	}
	if item.AccountID != nil {
		out.AccountId = item.AccountID
	}
	return out
}

func toCursorPage(limit int, next *string, hasMore bool) api.CursorPage {
	return api.CursorPage{HasMore: hasMore, Kind: api.Cursor, Limit: limit, NextCursor: next}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
