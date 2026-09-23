package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListJobs(ctx context.Context, request api.ListJobsRequestObject) (api.ListJobsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	kind := ""
	if request.Params.Kind != nil {
		kind = string(*request.Params.Kind)
	}
	page, err := a.Jobs.List(ctx, sc.portfolio.ID, kind, intValue(request.Params.Limit), stringValue(request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]api.Job, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIJob(&page.Items[i]))
	}
	return api.ListJobs200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func (a *API) GetJob(ctx context.Context, request api.GetJobRequestObject) (api.GetJobResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	job, err := a.Jobs.Get(ctx, sc.portfolio.ID, request.JobId)
	if err != nil {
		return nil, err
	}
	return api.GetJob200JSONResponse(toAPIJob(&job)), nil
}
