package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListExposures(context.Context, api.ListExposuresRequestObject) (api.ListExposuresResponseObject, error) {
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
