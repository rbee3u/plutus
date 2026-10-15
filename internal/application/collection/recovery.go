package collection

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// cycleSummary includes reports committed by earlier attempts, even when their
// connections are no longer collectable. Skipped reports count as account
// failures but do not represent an attempted identity collection.
func (s *Service) cycleSummary(ctx context.Context, point valuation.Point) (Summary, error) {
	rows, err := s.q.ListReportsByCycle(ctx, sqlcgen.ListReportsByCycleParams{PortfolioID: point.PortfolioID[:], CycleID: point.CycleID[:]})
	if err != nil {
		return Summary{}, err
	}
	type outcome struct {
		reports, failed        int
		collected, invalidated bool
	}
	identities := map[string]*outcome{}
	summary := Summary{Point: point, ReportCount: len(rows), ObservationTime: point.EvaluatedAt}
	for i := range rows {
		r := &rows[i]
		if r.AccountType == string(adapters.TypeManual) {
			continue
		}
		key := string(r.IdentityID)
		if identities[key] == nil {
			identities[key] = &outcome{}
		}
		o := identities[key]
		o.reports++
		if r.ErrorClass.String != ClassSkipped {
			o.collected = true
		}
		if r.Status == adapters.StatusFailed {
			summary.ReportFailed++
			o.failed++
			if r.ConnectionStatus == identity.ConnectionInvalid && adapters.StopsScheduling(r.ErrorClass.String) {
				o.invalidated = true
			}
		}
	}
	for _, o := range identities {
		if o.collected {
			summary.IdentityCount++
			if o.failed == o.reports {
				summary.IdentityFailed++
			}
		}
		if o.invalidated {
			summary.Invalidated++
		}
	}
	return summary, nil
}
