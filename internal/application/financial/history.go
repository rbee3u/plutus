package financial

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Points returns the series between from and to, sampled by granularity (raw, hour, day).
func (s *Service) Points(ctx context.Context, p PortfolioContext, from, to time.Time, granularity string) ([]valuation.Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]valuation.Point, error) {
		if !to.After(from) {
			return nil, ErrInvalid
		}
		switch granularity {
		case "", "raw", "hour", "day":
		default:
			return nil, ErrInvalid
		}
		return s.pointSeries(ctx, p, from, to, granularity)
	})
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
func (s *Service) Detail(ctx context.Context, p PortfolioContext, cycleID uuid.UUID) (PointDetail, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (PointDetail, error) {
		point, err := s.valuation.Get(ctx, p.ID, cycleID)
		if err != nil {
			return PointDetail{}, err
		}
		if point.DetailState == "expired" {
			return PointDetail{Point: point, Accounts: []valuation.AccountValuation{}}, nil
		}
		accounts, err := s.valuation.Accounts(ctx, p.ID, point.CycleID)
		if err != nil {
			return PointDetail{}, err
		}
		return PointDetail{Point: point, Accounts: accounts}, nil
	})
}

// BoundaryEvents lists derived Boundary Events between two instants.
func (s *Service) BoundaryEvents(ctx context.Context, p PortfolioContext, from, to time.Time) ([]valuation.BoundaryEvent, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]valuation.BoundaryEvent, error) {
		if !to.After(from) {
			return nil, ErrInvalid
		}
		rows, err := mysql.Queries(ctx, s.db).ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
		if err != nil {
			return nil, err
		}
		points, err := s.pointsFromRows(ctx, rows)
		if err != nil {
			return nil, err
		}
		out := []valuation.BoundaryEvent{}
		for i := range points {
			out = append(out, points[i].BoundaryEvents...)
		}
		return out, nil
	})
}

func (s *Service) pointsFromRows(ctx context.Context, rows []sqlcgen.ValuationPoint) ([]valuation.Point, error) {
	return s.valuation.PointsWithEvents(ctx, rows)
}

// Snapshot returns an actual retained point before the requested day's end.
// It never combines one point's total with a different point's holdings.
func (s *Service) Snapshot(ctx context.Context, p PortfolioContext, day string) (PointDetail, error) {
	local, err := time.ParseInLocation("2006-01-02", day, p.location())
	if err != nil {
		return PointDetail{}, ErrInvalid
	}
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (PointDetail, error) {
		row, err := mysql.Queries(ctx, s.db).GetRetainedSnapshotBefore(ctx, sqlcgen.GetRetainedSnapshotBeforeParams{PortfolioID: p.ID[:], EvaluatedAt: local.AddDate(0, 0, 1).UTC()})
		if errors.Is(err, sql.ErrNoRows) {
			return PointDetail{}, valuation.ErrNotFound
		}
		if err != nil {
			return PointDetail{}, err
		}
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return PointDetail{}, err
		}
		return s.Detail(ctx, p, cycle)
	})
}
