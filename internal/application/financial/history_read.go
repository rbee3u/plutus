package financial

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// PointPage contains raw points sorted newest first and the range's exact row count.
type PointPage struct {
	Items                []valuation.Point
	PrecedingPoint       *valuation.Point
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
			rows, err := q.ListValuationPointPage(ctx, sqlcgen.ListValuationPointPageParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC(), Offset: int32(offset), Limit: int32(limit + 1)}) // #nosec G115 -- offset and limit are bounded above.
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
			var preceding *valuation.Point
			if len(items) > limit {
				preceding = &items[limit]
				items = items[:limit]
			}
			return PointPage{PrecedingPoint: preceding, Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
		})
	})
}

func (s *Service) pointSeries(ctx context.Context, p PortfolioContext, from, to time.Time, granularity string, maxPoints int) ([]valuation.Point, error) {
	if granularity == "raw" || granularity == "" {
		rows, err := mysql.Queries(ctx, s.db).ListValuationPointSummariesBetween(ctx, sqlcgen.ListValuationPointSummariesBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC(), Limit: int32(maxPoints + 1)}) // #nosec G115 -- PointsWithLimit bounds maxPoints to 1..10000.
		if err != nil {
			return nil, err
		}
		if len(rows) > maxPoints {
			return nil, ErrSeriesLimit
		}
		out := make([]valuation.Point, 0, len(rows))
		for _, row := range rows {
			point, err := pointSummaryFromRow(row)
			if err != nil {
				return nil, err
			}
			out = append(out, point)
		}
		return out, nil
	}
	selected := []sqlcgen.ListValuationPointHeadersBetweenRow{}
	params := sqlcgen.ListValuationPointHeaderPageParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC(), CursorID: make([]byte, 16)}
	for {
		page, err := mysql.Queries(ctx, s.db).ListValuationPointHeaderPage(ctx, params)
		if err != nil {
			return nil, err
		}
		batch := make([]sqlcgen.ListValuationPointHeadersBetweenRow, 0, len(page)+1)
		if len(selected) > 0 {
			batch = append(batch, selected[len(selected)-1])
			selected = selected[:len(selected)-1]
		}
		for _, row := range page {
			batch = append(batch, sqlcgen.ListValuationPointHeadersBetweenRow(row))
		}
		selected = append(selected, sample(batch, granularity, p.location())...)
		if len(selected) > maxPoints {
			return nil, ErrSeriesLimit
		}
		if len(page) < 1000 {
			break
		}
		last := page[len(page)-1]
		params.HasCursor, params.CursorAt, params.CursorID = 1, last.EvaluatedAt, last.CycleID
	}
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
	return out, nil
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

type BoundaryPage struct {
	Items                []valuation.BoundaryEvent
	Offset, Limit, Total int
}

func (s *Service) BoundaryEventsPage(ctx context.Context, p PortfolioContext, from, to time.Time, offset, limit int) (BoundaryPage, error) {
	if limit == 0 {
		limit = 50
	}
	if !to.After(from) || offset < 0 || offset > math.MaxInt32 || limit < 1 || limit > 200 {
		return BoundaryPage{}, ErrInvalid
	}
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (BoundaryPage, error) {
		q := mysql.Queries(ctx, s.db)
		total, err := q.CountBoundaryEvents(ctx, sqlcgen.CountBoundaryEventsParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
		if err != nil {
			return BoundaryPage{}, err
		}
		rows, err := q.ListBoundaryEventPage(ctx, sqlcgen.ListBoundaryEventPageParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC(), Offset: int32(offset), Limit: int32(limit), PlatformOrder: ordering.PlatformsCSV(), AccountOrder: ordering.AccountsCSV()}) // #nosec G115 -- Pagination is bounded above.
		if err != nil {
			return BoundaryPage{}, err
		}
		out := BoundaryPage{Items: make([]valuation.BoundaryEvent, 0, len(rows)), Offset: offset, Limit: limit, Total: int(total)}
		for _, row := range rows {
			cycle, err := uuid.FromBytes(row.CycleID)
			if err != nil {
				return BoundaryPage{}, err
			}
			account, err := uuid.FromBytes(row.AccountID)
			if err != nil {
				return BoundaryPage{}, err
			}
			identity, err := uuid.FromBytes(row.IdentityID)
			if err != nil {
				return BoundaryPage{}, err
			}
			out.Items = append(out.Items, valuation.BoundaryEvent{CycleID: cycle, EvaluatedAt: row.EvaluatedAt, AccountID: account, AccountName: adapters.AccountName(adapters.AccountType(row.AccountType)), IdentityID: identity, IdentityName: row.IdentityName, Platform: row.Platform, Direction: row.Direction, Method: row.ValuationMethod, ValueUSD: nullDecimal(row.ValueUsd), ValuationComplete: row.ValuationComplete})
		}
		return out, nil
	})
}
