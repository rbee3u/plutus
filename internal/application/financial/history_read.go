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
	selected := sample(headers, granularity, p.location())
	ids := make([][]byte, 0, len(selected))
	for _, header := range selected {
		ids = append(ids, header.CycleID)
	}
	rows, err := s.pointRowsByIDs(ctx, p.ID, ids)
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

// IDs arrive in timeline order; batches preserve that order and bound SQL parameters.
func (s *Service) pointRowsByIDs(ctx context.Context, portfolioID uuid.UUID, ids [][]byte) ([]sqlcgen.ValuationPoint, error) {
	out := make([]sqlcgen.ValuationPoint, 0, len(ids))
	for start := 0; start < len(ids); start += 500 {
		rows, err := mysql.Queries(ctx, s.db).ListValuationPointSummariesByIDs(ctx, sqlcgen.ListValuationPointSummariesByIDsParams{PortfolioID: portfolioID[:], Ids: ids[start:min(start+500, len(ids))]})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, sqlcgen.ValuationPoint(row))
		}
	}
	return out, nil
}

func pointSummaryFromRow(row sqlcgen.ValuationPoint) (valuation.Point, error) {
	return valuation.PointFromRow(&sqlcgen.ValuationPoint{
		PortfolioID: row.PortfolioID, CycleID: row.CycleID, EvaluatedAt: row.EvaluatedAt,
		TotalUsd: row.TotalUsd, AccountCount: row.AccountCount, StaleAccountCount: row.StaleAccountCount,
		IncompleteAccountCount: row.IncompleteAccountCount, DetailState: row.DetailState, CalculationVersion: row.CalculationVersion,
	})
}
