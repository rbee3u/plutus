package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPICycle(c *cycle.Cycle, point *api.ValuationPointSummary) api.CollectionCycle {
	return api.CollectionCycle{
		Id:           c.ID,
		PortfolioId:  c.PortfolioID,
		Status:       api.CollectionCycleStatus(c.Status),
		Trigger:      api.CollectionCycleTrigger(c.Trigger),
		AttemptCount: int(c.AttemptCount),
		ErrorClass:   c.ErrorClass,
		ErrorDetail:  c.ErrorDetail,
		CreatedAt:    c.CreatedAt,
		StartedAt:    c.StartedAt,
		FinishedAt:   c.FinishedAt,
		Point:        point,
	}
}

func toAPICyclePoint(cycleID uuid.UUID, p *cycle.Point, boundaryContributionUSD, boundaryWithdrawalUSD string) *api.ValuationPointSummary {
	if p == nil {
		return nil
	}
	completeness := api.Complete
	if p.IncompleteAccountCount > 0 {
		completeness = api.Incomplete
	}
	return &api.ValuationPointSummary{
		CycleId: cycleID, CalculationVersion: int64(p.CalculationVersion), // #nosec G115 -- Application algorithm versions are positive, code-owned constants far below MaxInt64.
		EvaluatedAt:             p.EvaluatedAt,
		TotalUsd:                p.TotalUSD,
		Completeness:            completeness,
		AccountCount:            p.AccountCount,
		StaleAccountCount:       p.StaleAccountCount,
		IncompleteAccountCount:  p.IncompleteAccountCount,
		BoundaryContributionUsd: boundaryContributionUSD,
		BoundaryWithdrawalUsd:   boundaryWithdrawalUSD,
		DetailState:             api.ValuationPointSummaryDetailState(p.DetailState),
	}
}
