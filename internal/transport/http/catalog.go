package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) SearchCatalogAssetListings(ctx context.Context, request api.SearchCatalogAssetListingsRequestObject) (api.SearchCatalogAssetListingsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.SearchCatalogAssetListings401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.SearchCatalogAssetListings404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	in := catalog.ListingSearch{
		Query:  stringValue(request.Params.Q),
		Offset: intValue(request.Params.Offset),
		Limit:  intValue(request.Params.Limit),
	}
	if request.Params.PlatformId != nil {
		in.PlatformID = request.Params.PlatformId
	}
	if request.Params.MappingState != nil {
		in.MappingState = string(*request.Params.MappingState)
	}
	page, err := a.Catalog.SearchListings(ctx, request.PortfolioId, &in)
	if err != nil {
		return nil, err
	}
	items := make([]api.CatalogAssetListing, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPICatalogListing(&page.Items[i]))
	}
	return api.SearchCatalogAssetListings200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) SearchCatalogInstruments(ctx context.Context, request api.SearchCatalogInstrumentsRequestObject) (api.SearchCatalogInstrumentsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.SearchCatalogInstruments401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.SearchCatalogInstruments404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	in := catalog.InstrumentSearch{
		Query:  stringValue(request.Params.Q),
		Offset: intValue(request.Params.Offset),
		Limit:  intValue(request.Params.Limit),
		Pinned: request.Params.Pinned,
	}
	if request.Params.PlatformId != nil {
		in.PlatformID = request.Params.PlatformId
	}
	page, err := a.Catalog.SearchInstruments(ctx, request.PortfolioId, &in)
	if err != nil {
		return nil, err
	}
	items := make([]api.CatalogInstrument, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPICatalogInstrument(&page.Items[i]))
	}
	return api.SearchCatalogInstruments200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) PutCatalogListingMapping(ctx context.Context, request api.PutCatalogListingMappingRequestObject) (api.PutCatalogListingMappingResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PutCatalogListingMapping401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PutCatalogListingMapping404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Catalog == nil || a.Jobs == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	if !request.Body.State.Valid() {
		return api.PutCatalogListingMapping400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "Mapping state is invalid."))}, nil
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPut, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.PutCatalogListingMapping409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.CatalogMappingMutation
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		if rec.Status == http.StatusCreated {
			return api.PutCatalogListingMapping201JSONResponse{Body: body, Headers: api.PutCatalogListingMapping201ResponseHeaders{ETag: &etag}}, nil
		}
		return api.PutCatalogListingMapping200JSONResponse{Body: body, Headers: api.PutCatalogListingMapping200ResponseHeaders{ETag: &etag}}, nil
	}
	item, created, err := a.Catalog.PutMapping(ctx, request.PortfolioId, request.ListingId, &catalog.PutMappingInput{
		ActorUserID:         user.ID,
		RequestID:           requestTraceID(ctx),
		AssetID:             request.Body.AssetId,
		State:               string(request.Body.State),
		DisplayNameOverride: request.Body.DisplayNameOverride,
		Reason:              request.Body.Reason,
		IfMatch:             request.Params.IfMatch,
	})
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		return api.PutCatalogListingMapping404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, catalog.ErrExists):
		return api.PutCatalogListingMapping409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error())), nil
	case errors.Is(err, catalog.ErrPrecondition):
		return api.PutCatalogListingMapping412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, catalog.ErrInvalid):
		return api.PutCatalogListingMapping400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "catalog mapping change")
	if err != nil {
		return nil, err
	}
	body := api.CatalogMappingMutation{Mapping: toAPICatalogMapping(&item), Job: toAPIJob(&job, false)}
	etag := item.ETag()
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	a.storeIdempotency(ctx, user.ID, &request.PortfolioId, key, http.MethodPut, requestPath(ctx), hash, status, map[string]string{"ETag": etag}, body)
	if created {
		return api.PutCatalogListingMapping201JSONResponse{Body: body, Headers: api.PutCatalogListingMapping201ResponseHeaders{ETag: etagPtr(etag)}}, nil
	}
	return api.PutCatalogListingMapping200JSONResponse{Body: body, Headers: api.PutCatalogListingMapping200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) PutCatalogPin(ctx context.Context, request api.PutCatalogPinRequestObject) (api.PutCatalogPinResponseObject, error) { //nolint:gocritic // generated request object
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PutCatalogPin401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PutCatalogPin404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if !request.SubjectKind.Valid() {
		return api.PutCatalogPin400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", catalog.ErrInvalidSubject.Error()))}, nil
	}
	if a.Catalog == nil || a.Jobs == nil || request.Body == nil {
		return nil, ErrNotImplemented
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPut, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.PutCatalogPin409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.CatalogPinMutation
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		etag := rec.Headers["ETag"]
		if rec.Status == http.StatusCreated {
			return api.PutCatalogPin201JSONResponse{Body: body, Headers: api.PutCatalogPin201ResponseHeaders{ETag: &etag}}, nil
		}
		return api.PutCatalogPin200JSONResponse{Body: body, Headers: api.PutCatalogPin200ResponseHeaders{ETag: &etag}}, nil
	}
	item, created, err := a.Catalog.PutPin(ctx, request.PortfolioId, string(request.SubjectKind), request.SubjectId, &catalog.PutPinInput{
		ActorUserID: user.ID,
		RequestID:   requestTraceID(ctx),
		Reason:      request.Body.Reason,
		IfMatch:     request.Params.IfMatch,
	})
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		return api.PutCatalogPin404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, catalog.ErrExists):
		return api.PutCatalogPin409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "conflict", "Conflict", err.Error())), nil
	case errors.Is(err, catalog.ErrPrecondition):
		return api.PutCatalogPin412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, catalog.ErrInvalidSubject):
		return api.PutCatalogPin400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "catalog pin change")
	if err != nil {
		return nil, err
	}
	body := api.CatalogPinMutation{Pin: toAPICatalogPin(&item), Job: toAPIJob(&job, false)}
	etag := item.ETag()
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	a.storeIdempotency(ctx, user.ID, &request.PortfolioId, key, http.MethodPut, requestPath(ctx), hash, status, map[string]string{"ETag": etag}, body)
	if created {
		return api.PutCatalogPin201JSONResponse{Body: body, Headers: api.PutCatalogPin201ResponseHeaders{ETag: etagPtr(etag)}}, nil
	}
	return api.PutCatalogPin200JSONResponse{Body: body, Headers: api.PutCatalogPin200ResponseHeaders{ETag: etagPtr(etag)}}, nil
}

func (a *API) DeleteCatalogPin(ctx context.Context, request api.DeleteCatalogPinRequestObject) (api.DeleteCatalogPinResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.DeleteCatalogPin401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.DeleteCatalogPin404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if !request.SubjectKind.Valid() {
		return nil, catalog.ErrInvalidSubject
	}
	if a.Catalog == nil || a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Catalog.DisablePin(ctx, request.PortfolioId, string(request.SubjectKind), request.SubjectId, &catalog.DisablePinInput{
		ActorUserID: user.ID,
		RequestID:   requestTraceID(ctx),
		IfMatch:     request.Params.IfMatch,
	})
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		return api.DeleteCatalogPin404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, catalog.ErrPrecondition):
		return api.DeleteCatalogPin412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, catalog.ErrInvalidSubject):
		return nil, err
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "catalog pin change")
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.DeleteCatalogPin200JSONResponse{
		Body:    api.CatalogPinMutation{Pin: toAPICatalogPin(&item), Job: toAPIJob(&job, false)},
		Headers: api.DeleteCatalogPin200ResponseHeaders{ETag: etagPtr(etag)},
	}, nil
}

func toAPICatalogListing(item *catalog.Listing) api.CatalogAssetListing {
	out := api.CatalogAssetListing{
		ExternalRef:  item.ExternalRef,
		Id:           item.ID,
		Name:         item.Name,
		Pinned:       boolPtr(item.Pinned),
		PlatformCode: stringPtr(item.PlatformCode),
		PlatformId:   item.PlatformID,
		Status:       api.CatalogAssetListingStatus(item.Status),
		Symbol:       item.Symbol,
	}
	state := api.CatalogAssetListingMappingState(item.MappingState)
	out.MappingState = &state
	if item.MappedAssetID != nil {
		id := *item.MappedAssetID
		out.MappedAssetId = &id
	}
	if item.MappedAssetCode != nil {
		out.MappedAssetCode = item.MappedAssetCode
	}
	if item.MappingVersion != nil {
		version := int(*item.MappingVersion)
		out.MappingVersion = &version
	}
	return out
}

func toAPICatalogInstrument(item *catalog.Instrument) api.CatalogInstrument {
	return api.CatalogInstrument{
		ExpiryAt:     item.ExpiryAt,
		ExternalRef:  item.ExternalRef,
		Id:           item.ID,
		Kind:         api.CatalogInstrumentKind(item.Kind),
		Pinned:       boolPtr(item.Pinned),
		PlatformCode: stringPtr(item.PlatformCode),
		PlatformId:   item.PlatformID,
		Status:       api.CatalogInstrumentStatus(item.Status),
		Symbol:       item.Symbol,
	}
}

func toAPICatalogMapping(item *catalog.Mapping) api.CatalogMapping {
	revision := item.CurrentRevisionID
	return api.CatalogMapping{
		AssetId:             item.AssetID,
		CurrentRevisionId:   &revision,
		DisplayNameOverride: item.DisplayNameOverride,
		Id:                  item.ID,
		ListingId:           item.ListingID,
		PortfolioId:         item.PortfolioID,
		State:               api.CatalogMappingState(item.State),
		Version:             int(item.Version),
	}
}

func toAPICatalogPin(item *catalog.Pin) api.CatalogPin {
	revision := item.CurrentRevisionID
	return api.CatalogPin{
		CurrentRevisionId: &revision,
		Enabled:           item.Enabled,
		Id:                item.ID,
		PortfolioId:       item.PortfolioID,
		SubjectId:         item.SubjectID,
		SubjectKind:       api.CatalogPinSubjectKind(item.SubjectKind),
		Version:           int(item.Version),
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
