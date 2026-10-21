package financial

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// PortfolioContext is what reads need to know about the portfolio.
type PortfolioContext struct {
	ID       uuid.UUID
	Timezone string
}

func (p PortfolioContext) location() *time.Location {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (s *Service) latestContext(ctx context.Context, p PortfolioContext) (Context, *valuation.Point, []valuation.AccountValuation, error) {
	c := Context{PortfolioID: p.ID, Timezone: p.Timezone}
	point, accounts, err := s.valuation.LatestWithAccounts(ctx, p.ID)
	if errors.Is(err, valuation.ErrNoPoint) {
		return c, nil, nil, nil
	}
	if err != nil {
		return c, nil, nil, err
	}
	c.CycleID, c.EvaluatedAt = &point.CycleID, &point.EvaluatedAt
	return c, &point, accounts, nil
}

// indexAccounts shares the loaded accounts for one read operation.
func indexAccounts(accounts []valuation.AccountValuation) map[uuid.UUID]*valuation.AccountValuation {
	byID := make(map[uuid.UUID]*valuation.AccountValuation, len(accounts))
	for i := range accounts {
		byID[accounts[i].AccountID] = &accounts[i]
	}
	return byID
}

// contextAt fixes collected data while retaining current presentation overlays.
func (s *Service) contextAt(ctx context.Context, p PortfolioContext, cycleID *uuid.UUID) (Context, *valuation.Point, []valuation.AccountValuation, error) {
	if cycleID == nil {
		return s.latestContext(ctx, p)
	}
	detail, err := s.Detail(ctx, p, *cycleID)
	if errors.Is(err, valuation.ErrNotFound) {
		// Current holdings survive the public history cutoff. Pinning the point
		// returned by a latest read must retain that same access.
		latest, readErr := mysql.Queries(ctx, s.db).GetLatestValuationPoint(ctx, p.ID[:])
		if readErr != nil && !errors.Is(readErr, sql.ErrNoRows) {
			return Context{}, nil, nil, readErr
		}
		if readErr == nil {
			id, readErr := uuid.FromBytes(latest.CycleID)
			if readErr != nil {
				return Context{}, nil, nil, readErr
			}
			if id == *cycleID {
				return s.latestContext(ctx, p)
			}
		}
	}
	if err != nil {
		return Context{}, nil, nil, err
	}
	if detail.Point.DetailState == "expired" || detail.Point.DetailState == "pruning" {
		return Context{}, nil, nil, valuation.ErrDetailExpired
	}
	return Context{PortfolioID: p.ID, Timezone: p.Timezone, CycleID: cycleID, EvaluatedAt: &detail.Point.EvaluatedAt}, &detail.Point, detail.Accounts, nil
}
