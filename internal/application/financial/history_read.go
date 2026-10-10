package financial

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// PointPage contains raw points sorted newest first and the range's exact row count.
type PointPage struct {
	Items                []valuation.Point
	Offset, Limit, Total int
}

// PointsPage reads only the requested raw page. Count and page use one read snapshot.
func (s *Service) PointsPage(ctx context.Context, p PortfolioContext, from, to time.Time, offset, limit int) (PointPage, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (PointPage, error) {
		if limit == 0 {
			limit = 20
		}
		if !to.After(from) || offset < 0 || offset > math.MaxInt32 || (limit != 10 && limit != 20 && limit != 50) {
			return PointPage{}, ErrInvalid
		}
		return mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) (PointPage, error) {
			total, err := q.CountValuationPointsBetween(ctx, sqlcgen.CountValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
			if err != nil {
				return PointPage{}, err
			}
			rows, err := q.ListValuationPointPage(ctx, sqlcgen.ListValuationPointPageParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC(), Offset: int32(offset), Limit: int32(limit)}) // #nosec G115 -- offset and limit are bounded above.
			if err != nil {
				return PointPage{}, err
			}
			items := make([]valuation.Point, 0, len(rows))
			for _, row := range rows {
				point, err := pointSummaryFromRow(sqlcgen.ValuationPoint(row))
				if err != nil {
					return PointPage{}, err
				}
				items = append(items, point)
			}
			if err := s.valuation.WithBoundaryTotals(ctx, items); err != nil {
				return PointPage{}, err
			}
			return PointPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
		})
	})
}

func (s *Service) pointSeries(ctx context.Context, p PortfolioContext, from, to time.Time, granularity string) ([]valuation.Point, error) {
	if granularity == "raw" || granularity == "" {
		rows, err := mysql.Queries(ctx, s.db).ListValuationPointSummariesBetween(ctx, sqlcgen.ListValuationPointSummariesBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
		if err != nil {
			return nil, err
		}
		out := make([]valuation.Point, 0, len(rows))
		for _, row := range rows {
			point, err := pointSummaryFromRow(row)
			if err != nil {
				return nil, err
			}
			out = append(out, point)
		}
		return out, s.valuation.WithBoundaryTotals(ctx, out)
	}
	headers, err := mysql.Queries(ctx, s.db).ListValuationPointHeadersBetween(ctx, sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	points := make([]valuation.Point, 0, len(headers))
	for _, header := range headers {
		id, err := uuid.FromBytes(header.CycleID)
		if err != nil {
			return nil, err
		}
		points = append(points, valuation.Point{CycleID: id, EvaluatedAt: header.EvaluatedAt})
	}
	selected := sample(points, granularity, p.location())
	out := make([]valuation.Point, 0, len(selected))
	// Batches bound SQL placeholders without changing chronological ordering.
	for start := 0; start < len(selected); start += 500 {
		batch := selected[start:min(start+500, len(selected))]
		ids := make([][]byte, 0, len(batch))
		for _, point := range batch {
			ids = append(ids, point.CycleID[:])
		}
		rows, err := mysql.Queries(ctx, s.db).ListValuationPointSummariesByIDs(ctx, sqlcgen.ListValuationPointSummariesByIDsParams{PortfolioID: p.ID[:], Ids: ids})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			point, err := pointSummaryFromRow(sqlcgen.ValuationPoint(row))
			if err != nil {
				return nil, err
			}
			out = append(out, point)
		}
	}
	return out, s.valuation.WithBoundaryTotals(ctx, out)
}

func pointSummaryFromRow(row sqlcgen.ValuationPoint) (valuation.Point, error) {
	return valuation.PointFromRow(&sqlcgen.ValuationPoint{
		PortfolioID: row.PortfolioID, CycleID: row.CycleID, EvaluatedAt: row.EvaluatedAt,
		TotalUsd: row.TotalUsd, AccountCount: row.AccountCount, StaleAccountCount: row.StaleAccountCount,
		IncompleteAccountCount: row.IncompleteAccountCount, DetailState: row.DetailState, CalculationVersion: row.CalculationVersion,
	})
}
