package valuation

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/service/assetexclusion"
	"github.com/rbee3u/plutus/internal/service/cycle"
)

// Service computes and reads valuations.
type Service struct {
	db  *sql.DB
	now func() time.Time
}

// NewService constructs a valuation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
}

// WritePoint values every account report of a cycle and commits its immutable valuation point.
// evaluatedAt is the instant recorded for the point; zero means now.
func (s *Service) WritePoint(ctx context.Context, portfolioID, cycleID uuid.UUID, evaluatedAt time.Time, claim ...*cycle.Claim) (Point, error) {
	return mysql.WithWriteContext(ctx, s.db, func(ctx context.Context) (Point, error) {
		if _, err := mysql.Queries(ctx, s.db).LockPortfolio(ctx, portfolioID[:]); err != nil {
			return Point{}, err
		}
		stored, err := mysql.Queries(ctx, s.db).GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
		if err == nil {
			return s.PointWithEvents(ctx, &stored)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Point{}, err
		}
		if evaluatedAt.IsZero() {
			evaluatedAt = s.now()
		}
		evaluatedAt = evaluatedAt.UTC().Truncate(time.Microsecond)
		latest, err := mysql.Queries(ctx, s.db).GetLatestValuationPoint(ctx, portfolioID[:])
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Point{}, err
		}
		if err == nil && (evaluatedAt.Before(latest.EvaluatedAt) || (evaluatedAt.Equal(latest.EvaluatedAt) && bytes.Compare(cycleID[:], latest.CycleID) < 0)) {
			return Point{}, errors.New("cannot insert a valuation point before committed history")
		}

		accounts, err := s.valueCycle(ctx, portfolioID, cycleID)
		if err != nil {
			return Point{}, err
		}
		previous, err := s.previousAccounts(ctx, portfolioID, evaluatedAt, cycleID)
		if err != nil {
			return Point{}, err
		}
		point := buildPoint(portfolioID, cycleID, evaluatedAt, accounts, previous)

		if err := s.persistPoint(ctx, point, accounts, claim...); err != nil {
			return Point{}, err
		}
		return s.PointByCycle(ctx, portfolioID, cycleID)
	})
}

func (s *Service) valueCycle(ctx context.Context, portfolioID, cycleID uuid.UUID) ([]AccountValuation, error) {
	exclusions, err := assetexclusion.ActiveForCycle(ctx, s.db, portfolioID, cycleID)
	if err != nil {
		return nil, err
	}
	rows, err := mysql.Queries(ctx, s.db).ListReportsByCycle(ctx, sqlcgen.ListReportsByCycleParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
	if err != nil {
		return nil, err
	}
	book := NewPriceBook(mysql.Queries(ctx, s.db), portfolioID)
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		input, err := s.prepareReport(ctx, book, &rows[i], exclusions, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, evaluateReport(input))
	}
	sortAccounts(out)
	return out, nil
}
