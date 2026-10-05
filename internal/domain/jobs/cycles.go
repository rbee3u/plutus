package jobs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// CyclePoint is the Valuation Point written by a succeeded Collection Cycle.
type CyclePoint struct {
	ID                      uuid.UUID
	EvaluatedAt             time.Time
	TotalUSD                string
	AccountCount            int
	StaleAccountCount       int
	IncompleteAccountCount  int
	BoundaryContributionUSD string
	BoundaryWithdrawalUSD   string
	RecalculatedAt          *time.Time
}

// Cycle is one Collection Cycle job with its point when one exists.
type Cycle struct {
	Job
	Point *CyclePoint
}

// CyclePage is a cursor page of cycles.
type CyclePage struct {
	Items      []Cycle
	Limit      int
	NextCursor *string
	HasMore    bool
}

// ListCycles returns Collection Cycles newest first with their Valuation Points.
func (s *Service) ListCycles(ctx context.Context, portfolioID uuid.UUID, limit int, cursor string) (CyclePage, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return CyclePage{}, ErrInvalidLimit
	}
	params := sqlcgen.ListCycleJobsWithPointsParams{
		PortfolioID:     portfolioID[:],
		HasCursor:       0,
		CursorCreatedAt: time.Time{},
		CursorID:        make([]byte, 16),
		Limit:           int32(limit + 1),
	}
	if cursor != "" {
		decoded, err := decodeCursor(cursor, portfolioID)
		if err != nil {
			return CyclePage{}, err
		}
		params.HasCursor = 1
		params.CursorCreatedAt = time.UnixMicro(decoded.CreatedAt).UTC()
		params.CursorID = decoded.ID[:]
	}
	rows, err := s.q.ListCycleJobsWithPoints(ctx, params)
	if err != nil {
		return CyclePage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]Cycle, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		job, err := fromFields(r.ID, r.PortfolioID, KindCollectionCycle, r.Status, r.Trigger, nil, r.AttemptCount, r.MaxAttempts, r.LastErrorClass, r.LastDetail, r.CreatedAt, r.CreatedAt, r.StartedAt, r.FinishedAt)
		if err != nil {
			return CyclePage{}, err
		}
		item := Cycle{Job: job}
		if len(r.PointID) == 16 && r.EvaluatedAt.Valid {
			pointID, err := uuid.FromBytes(r.PointID)
			if err != nil {
				return CyclePage{}, err
			}
			point := &CyclePoint{
				ID:                      pointID,
				EvaluatedAt:             r.EvaluatedAt.Time,
				TotalUSD:                r.TotalUsd.String,
				AccountCount:            int(r.AccountCount.Int32),
				StaleAccountCount:       int(r.StaleAccountCount.Int32),
				IncompleteAccountCount:  int(r.IncompleteAccountCount.Int32),
				BoundaryContributionUSD: r.BoundaryContributionUsd.String,
				BoundaryWithdrawalUSD:   r.BoundaryWithdrawalUsd.String,
			}
			if r.RecalculatedAt.Valid {
				t := r.RecalculatedAt.Time
				point.RecalculatedAt = &t
			}
			item.Point = point
		}
		items = append(items, item)
	}
	page := CyclePage{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{PortfolioID: portfolioID, CreatedAt: last.CreatedAt.UTC().UnixMicro(), ID: last.ID})
		if err != nil {
			return CyclePage{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}
