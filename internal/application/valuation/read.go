package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// PointByJob returns the point written by one Collection Cycle job.
func (s *Service) PointByJob(ctx context.Context, portfolioID, jobID uuid.UUID) (Point, error) {
	row, err := s.q.GetValuationPointByJob(ctx, sqlcgen.GetValuationPointByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNotFound
	}
	if err != nil {
		return Point{}, err
	}
	return PointFromRow(&row)
}

// Latest returns the newest point of a Portfolio.
func (s *Service) Latest(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	row, err := s.q.GetLatestValuationPoint(ctx, portfolioID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNoPoint
	}
	if err != nil {
		return Point{}, err
	}
	return PointFromRow(&row)
}

// First returns the earliest point of the Portfolio.
func (s *Service) First(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	row, err := s.q.GetFirstValuationPoint(ctx, portfolioID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNoPoint
	}
	if err != nil {
		return Point{}, err
	}
	return PointFromRow(&row)
}

// Get returns one point by ID.
func (s *Service) Get(ctx context.Context, portfolioID, pointID uuid.UUID) (Point, error) {
	row, err := s.q.GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: portfolioID[:], ID: pointID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNotFound
	}
	if err != nil {
		return Point{}, err
	}
	return PointFromRow(&row)
}

// Accounts returns the stored Account Valuations of a point.
func (s *Service) Accounts(ctx context.Context, portfolioID, jobID uuid.UUID) ([]AccountValuation, error) {
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if err != nil {
		return nil, err
	}
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		item, err := accountFromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	sortAccounts(out)
	return out, nil
}

func PointFromRow(row *sqlcgen.ValuationPoint) (Point, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Point{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Point{}, err
	}
	jobID, err := uuid.FromBytes(row.JobID)
	if err != nil {
		return Point{}, err
	}
	point := Point{
		ID:                      id,
		PortfolioID:             portfolioID,
		JobID:                   jobID,
		EvaluatedAt:             row.EvaluatedAt,
		TotalUSD:                decimal.Compact(row.TotalUsd),
		AccountCount:            int(row.AccountCount),
		StaleAccountCount:       int(row.StaleAccountCount),
		IncompleteAccountCount:  int(row.IncompleteAccountCount),
		BoundaryContributionUSD: decimal.Compact(row.BoundaryContributionUsd),
		BoundaryWithdrawalUSD:   decimal.Compact(row.BoundaryWithdrawalUsd),
		BoundaryEvents:          []BoundaryEvent{},
		Issues:                  []Issue{},
		CreatedAt:               row.CreatedAt,
	}
	if row.RecalculatedAt.Valid {
		t := row.RecalculatedAt.Time
		point.RecalculatedAt = &t
	}
	if len(row.BoundaryJson) > 0 {
		if err := json.Unmarshal(row.BoundaryJson, &point.BoundaryEvents); err != nil {
			return Point{}, err
		}
		for i := range point.BoundaryEvents {
			point.BoundaryEvents[i].PointID = id
			point.BoundaryEvents[i].EvaluatedAt = row.EvaluatedAt
			if point.BoundaryEvents[i].ValueUSD != nil {
				point.BoundaryEvents[i].valueRat = decimal.ParseOrZero(*point.BoundaryEvents[i].ValueUSD)
			}
		}
	}
	if len(row.IssuesJson) > 0 {
		if err := json.Unmarshal(row.IssuesJson, &point.Issues); err != nil {
			return Point{}, err
		}
	}
	return point, nil
}
