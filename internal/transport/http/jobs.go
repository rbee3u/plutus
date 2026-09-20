package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/idempotency"
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

func (a *API) CreateProjectionRebuild(ctx context.Context, request api.CreateProjectionRebuildRequestObject) (api.CreateProjectionRebuildResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.CreateProjectionRebuild401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.CreateProjectionRebuild404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	reason := "projection rebuild"
	if request.Body != nil && request.Body.Reason != nil {
		reason = *request.Body.Reason
		if len(reason) > 1024 {
			return api.CreateProjectionRebuild400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "reason is too long"))}, nil
		}
	}
	raw := []byte("{}")
	if request.Body != nil {
		encoded, err := json.Marshal(request.Body)
		if err != nil {
			return nil, err
		}
		raw = encoded
	}
	key := headerValue(request.Params.IdempotencyKey)
	rec, hash, err := a.lookupIdempotency(ctx, user.ID, key, http.MethodPost, requestPath(ctx), raw)
	if errors.Is(err, idempotency.ErrConflict) {
		return api.CreateProjectionRebuild409ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")), nil
	}
	if err != nil {
		return nil, err
	}
	if rec != nil {
		var body api.Job
		if err := json.Unmarshal(rec.Body, &body); err != nil {
			return nil, err
		}
		loc := rec.Headers["Location"]
		return api.CreateProjectionRebuild202JSONResponse{Body: body, Headers: api.CreateProjectionRebuild202ResponseHeaders{Location: strPtr(loc)}}, nil
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, reason)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			return api.CreateProjectionRebuild404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
		}
		return nil, err
	}
	body := toAPIJob(&job, false)
	location := jobLocation(request.PortfolioId, job.ID)
	a.storeIdempotency(ctx, user.ID, &request.PortfolioId, key, http.MethodPost, requestPath(ctx), hash, http.StatusAccepted, map[string]string{"Location": location}, body)
	return api.CreateProjectionRebuild202JSONResponse{
		Body:    body,
		Headers: api.CreateProjectionRebuild202ResponseHeaders{Location: &location},
	}, nil
}

func jobLocation(portfolioID, jobID uuid.UUID) string {
	return "/api/v1/portfolios/" + portfolioID.String() + "/jobs/" + jobID.String()
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
