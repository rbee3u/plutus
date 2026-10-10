package financial

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
)

// PortfolioContext is what reads need to know about the Portfolio.
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
	point, err := s.valuation.Latest(ctx, p.ID)
	if errors.Is(err, valuation.ErrNoPoint) {
		return c, nil, nil, nil
	}
	if err != nil {
		return c, nil, nil, err
	}
	c.CycleID, c.EvaluatedAt = &point.CycleID, &point.EvaluatedAt
	c.ValuationRevision = new(point.Revision())
	accounts, err := s.valuation.Accounts(ctx, p.ID, point.CycleID)
	if err != nil {
		return c, nil, nil, err
	}
	return c, &point, accounts, nil
}

// indexAccounts shares the loaded Accounts for one read operation.
func indexAccounts(accounts []valuation.AccountValuation) map[uuid.UUID]*valuation.AccountValuation {
	byID := make(map[uuid.UUID]*valuation.AccountValuation, len(accounts))
	for i := range accounts {
		byID[accounts[i].AccountID] = &accounts[i]
	}
	return byID
}
