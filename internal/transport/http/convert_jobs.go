package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/jobs"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIJob(j *jobs.Job) api.Job {
	return api.Job{
		Id: j.ID, PortfolioId: j.PortfolioID, Kind: api.JobKind(j.Kind), Status: api.JobStatus(j.Status), Trigger: api.JobTrigger(j.Trigger),
		AttemptCount: int(j.AttemptCount), MaxAttempts: int(j.MaxAttempts), LastErrorClass: j.LastErrorClass, LastDetail: j.LastDetail,
		CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func toAPICycle(j *jobs.Job, point *api.ValuationPointSummary) api.CollectionCycle {
	return api.CollectionCycle{
		Id: j.ID, PortfolioId: j.PortfolioID, Status: api.JobStatus(j.Status), Trigger: api.CollectionCycleTrigger(j.Trigger),
		AttemptCount: int(j.AttemptCount), MaxAttempts: int(j.MaxAttempts), LastErrorClass: j.LastErrorClass, LastDetail: j.LastDetail,
		CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt, Point: point,
	}
}

func toAPICyclePoint(jobID uuid.UUID, p *jobs.CyclePoint) *api.ValuationPointSummary {
	if p == nil {
		return nil
	}
	completeness := api.Complete
	if p.IncompleteAccountCount > 0 {
		completeness = api.Incomplete
	}
	return &api.ValuationPointSummary{
		Id: p.ID, JobId: jobID, EvaluatedAt: p.EvaluatedAt, TotalUsd: p.TotalUSD, Completeness: completeness,
		AccountCount: p.AccountCount, StaleAccountCount: p.StaleAccountCount, IncompleteAccountCount: p.IncompleteAccountCount,
		BoundaryContributionUsd: p.BoundaryContributionUSD, BoundaryWithdrawalUsd: p.BoundaryWithdrawalUSD, RecalculatedAt: p.RecalculatedAt,
	}
}
