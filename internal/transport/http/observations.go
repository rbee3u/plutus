package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListObservations(ctx context.Context, request api.ListObservationsRequestObject) (api.ListObservationsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListObservations401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListObservations404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Observations == nil {
		return nil, ErrNotImplemented
	}
	if request.Params.Kind != nil && !request.Params.Kind.Valid() {
		return api.ListObservations400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "Observation kind is invalid."))}, nil
	}
	in := observation.ListFilter{
		Limit:  intValue(request.Params.Limit),
		Cursor: stringValue(request.Params.Cursor),
		From:   request.Params.From,
		To:     request.Params.To,
	}
	if request.Params.Kind != nil {
		in.Kind = string(*request.Params.Kind)
	}
	if request.Params.IdentityId != nil {
		in.IdentityID = request.Params.IdentityId
	}
	if request.Params.AccountId != nil {
		in.AccountID = request.Params.AccountId
	}
	page, err := a.Observations.List(ctx, request.PortfolioId, &in)
	if err != nil {
		if errors.Is(err, observation.ErrInvalid) || errors.Is(err, observation.ErrInvalidPage) || errors.Is(err, observation.ErrInvalidCursor) {
			return api.ListObservations400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
		}
		return nil, err
	}
	items := make([]api.ObservationSummary, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIObservationSummary(&page.Items[i]))
	}
	return api.ListObservations200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func (a *API) GetObservation(ctx context.Context, request api.GetObservationRequestObject) (api.GetObservationResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetObservation401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetObservation404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Observations == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Observations.Get(ctx, request.PortfolioId, request.ObservationId)
	if errors.Is(err, observation.ErrNotFound) {
		return api.GetObservation404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.GetObservation200JSONResponse{
		Body:    toAPIObservation(&item),
		Headers: api.GetObservation200ResponseHeaders{ETag: etagPtr(etag)},
	}, nil
}

func toAPIObservationSummary(item *observation.Observation) api.ObservationSummary {
	summary := item.Summary()
	out := api.ObservationSummary{
		CollectedAt:        item.CollectedAt,
		Completeness:       api.ObservationSummaryCompleteness(item.Completeness),
		Id:                 item.ID,
		IdentityId:         item.IdentityID,
		IdentitySnapshotId: item.IdentitySnapshotID,
		Kind:               api.ObservationKind(item.Kind),
		PortfolioId:        item.PortfolioID,
		SourceObservedAt:   item.SourceObservedAt,
		Summary:            &summary,
		Version:            int(item.Version),
	}
	if item.AccountID != nil {
		out.AccountId = item.AccountID
	}
	return out
}
