package httpserver

import (
	"context"
	"errors"

	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListJobs(ctx context.Context, request api.ListJobsRequestObject) (api.ListJobsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListJobs401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListJobs404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	limit := 0
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	cursor := ""
	if request.Params.Cursor != nil {
		cursor = *request.Params.Cursor
	}
	page, err := a.Jobs.List(ctx, request.PortfolioId, limit, cursor)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			return api.ListJobs404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
		}
		return nil, err
	}
	items := make([]api.Job, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIJob(&page.Items[i], false))
	}
	return api.ListJobs200JSONResponse{Items: items, Page: api.CursorPage{
		HasMore:    page.HasMore,
		Kind:       api.Cursor,
		Limit:      page.Limit,
		NextCursor: page.NextCursor,
	}}, nil
}

func (a *API) GetJob(ctx context.Context, request api.GetJobRequestObject) (api.GetJobResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetJob401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetJob404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Jobs.Get(ctx, request.PortfolioId, request.JobId)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			return api.GetJob404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
		}
		return nil, err
	}
	return api.GetJob200JSONResponse(toAPIJob(&item, true)), nil
}

func toAPIJob(item *jobs.Job, includeAttempts bool) api.Job {
	out := api.Job{
		AttemptCount:    int(item.AttemptCount),
		CreatedAt:       item.CreatedAt,
		FinishedAt:      item.FinishedAt,
		Id:              item.ID,
		Kind:            api.JobKind(item.Kind),
		LastErrorCode:   item.LastErrorCode,
		LastErrorDetail: item.LastErrorDetail,
		MaxAttempts:     int(item.MaxAttempts),
		PortfolioId:     item.PortfolioID,
		StartedAt:       item.StartedAt,
		Status:          api.JobStatus(item.Status),
	}
	if includeAttempts {
		attempts := make([]api.JobAttempt, 0, len(item.Attempts))
		for i := range item.Attempts {
			attempts = append(attempts, toAPIJobAttempt(&item.Attempts[i]))
		}
		out.Attempts = &attempts
	}
	return out
}

func toAPIJobAttempt(item *jobs.Attempt) api.JobAttempt {
	return api.JobAttempt{
		AttemptNumber: int(item.AttemptNumber),
		ErrorClass:    item.ErrorClass,
		ErrorCode:     item.ErrorCode,
		FinishedAt:    item.FinishedAt,
		Outcome:       api.JobAttemptOutcome(item.Outcome),
		SafeDetail:    item.SafeDetail,
		StartedAt:     item.StartedAt,
		WorkerId:      item.WorkerID,
	}
}
