package financial

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Points returns the series between from and to, sampled by granularity (raw, hour, day).
func (s *Service) Points(ctx context.Context, p PortfolioContext, from, to time.Time, granularity string) ([]valuation.Point, error) {
	if !to.After(from) {
		return nil, ErrInvalid
	}
	switch granularity {
	case "", "raw", "hour", "day":
	default:
		return nil, ErrInvalid
	}
	return s.pointSeries(ctx, p, from, to, granularity)
}

func sample(points []valuation.Point, granularity string, loc *time.Location) []valuation.Point {
	if granularity == "" || granularity == "raw" || len(points) == 0 {
		return points
	}
	out := make([]valuation.Point, 0, len(points))
	var lastBucket time.Time
	for i := range points {
		local := points[i].EvaluatedAt.In(loc)
		var bucket time.Time
		if granularity == "hour" {
			bucket = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
		} else {
			bucket = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		}
		if len(out) > 0 && bucket.Equal(lastBucket) {
			out[len(out)-1] = points[i]
			continue
		}
		out = append(out, points[i])
		lastBucket = bucket
	}
	return out
}

// Detail returns one point with its Account Valuations.
func (s *Service) Detail(ctx context.Context, p PortfolioContext, pointID uuid.UUID) (PointDetail, error) {
	point, err := s.valuation.Get(ctx, p.ID, pointID)
	if err != nil {
		return PointDetail{}, err
	}
	accounts, err := s.valuation.Accounts(ctx, p.ID, point.JobID)
	if err != nil {
		return PointDetail{}, err
	}
	return PointDetail{Point: point, Accounts: accounts}, nil
}

// BoundaryEvents lists derived Boundary Events between two instants.
func (s *Service) BoundaryEvents(ctx context.Context, p PortfolioContext, from, to time.Time) ([]valuation.BoundaryEvent, error) {
	if !to.After(from) {
		return nil, ErrInvalid
	}
	rows, err := s.q.ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	points, err := pointsFromRows(rows)
	if err != nil {
		return nil, err
	}
	out := []valuation.BoundaryEvent{}
	for i := range points {
		out = append(out, points[i].BoundaryEvents...)
	}
	return out, nil
}

func pointsFromRows(rows []sqlcgen.ValuationPoint) ([]valuation.Point, error) {
	out := make([]valuation.Point, 0, len(rows))
	for i := range rows {
		point, err := valuation.PointFromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, nil
}
