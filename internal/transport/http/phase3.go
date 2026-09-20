package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAuditEvents(context.Context, api.ListAuditEventsRequestObject) (api.ListAuditEventsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) SearchCatalogAssetListings(context.Context, api.SearchCatalogAssetListingsRequestObject) (api.SearchCatalogAssetListingsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PutCatalogListingMapping(context.Context, api.PutCatalogListingMappingRequestObject) (api.PutCatalogListingMappingResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) SearchCatalogInstruments(context.Context, api.SearchCatalogInstrumentsRequestObject) (api.SearchCatalogInstrumentsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) DeleteCatalogPin(context.Context, api.DeleteCatalogPinRequestObject) (api.DeleteCatalogPinResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PutCatalogPin(context.Context, api.PutCatalogPinRequestObject) (api.PutCatalogPinResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListExposures(context.Context, api.ListExposuresRequestObject) (api.ListExposuresResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListObservations(context.Context, api.ListObservationsRequestObject) (api.ListObservationsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetObservation(context.Context, api.GetObservationRequestObject) (api.GetObservationResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListPeriodSummaries(context.Context, api.ListPeriodSummariesRequestObject) (api.ListPeriodSummariesResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) CreateProjectionRebuild(context.Context, api.CreateProjectionRebuildRequestObject) (api.CreateProjectionRebuildResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListValuations(context.Context, api.ListValuationsRequestObject) (api.ListValuationsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListValueChanges(context.Context, api.ListValueChangesRequestObject) (api.ListValueChangesResponseObject, error) {
	return nil, ErrNotImplemented
}
