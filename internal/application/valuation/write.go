package valuation

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/assetexclusion"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Service computes and reads valuations.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a valuation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// WritePoint values every Account Report of the job and upserts the job's Valuation Point.
// evaluatedAt is the instant recorded for the point; zero means now.
func (s *Service) WritePoint(ctx context.Context, portfolioID, jobID uuid.UUID, evaluatedAt time.Time) (Point, error) {
	if evaluatedAt.IsZero() {
		evaluatedAt = s.now()
	}
	evaluatedAt = evaluatedAt.UTC().Truncate(time.Microsecond)
	accounts, err := s.valueJob(ctx, portfolioID, jobID)
	if err != nil {
		return Point{}, err
	}
	previous, err := s.previousAccounts(ctx, portfolioID, evaluatedAt, jobID)
	if err != nil {
		return Point{}, err
	}
	point := buildPoint(portfolioID, jobID, evaluatedAt, accounts, previous)

	if err := s.persistPoint(ctx, point, accounts); err != nil {
		return Point{}, err
	}
	return s.PointByJob(ctx, portfolioID, jobID)
}

func (s *Service) valueJob(ctx context.Context, portfolioID, jobID uuid.UUID) ([]AccountValuation, error) {
	ignored, err := assetexclusion.ActiveForJob(ctx, s.db, portfolioID, jobID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if err != nil {
		return nil, err
	}
	book := NewPriceBook(s.q)
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		input, err := s.prepareReport(ctx, book, &rows[i], ignored, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, evaluateReport(input))
	}
	sortAccounts(out)
	return out, nil
}
