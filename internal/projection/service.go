package projection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Service rebuilds projection generations from source facts.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a projection service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Request is the rebuild envelope retained for targeted rebuilds.
type Request struct {
	PortfolioID uuid.UUID
	JobID       uuid.UUID
	Kinds       []string
}

// Rebuild writes a new generation and activates it when validation succeeds.
func (s *Service) Rebuild(ctx context.Context, in Request) error {
	if in.PortfolioID == uuid.Nil || in.JobID == uuid.Nil {
		return fmt.Errorf("%w: missing portfolio or job", ErrFailed)
	}
	_ = in.Kinds
	facts, err := s.loadFacts(ctx, in.PortfolioID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: portfolio not found", ErrFailed)
		}
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	asOf := s.now().UTC()
	generationID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	if err := s.q.InsertProjectionGeneration(ctx, sqlcgen.InsertProjectionGenerationParams{
		ID:                mustBin(generationID),
		PortfolioID:       mustBin(in.PortfolioID),
		JobID:             mustBin(in.JobID),
		Status:            "building",
		Timezone:          facts.timezone,
		DisplayAssetID:    mustBin(facts.displayAsset),
		SourceWatermarkAt: asOf,
		StartedAt:         asOf,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	built := s.build(facts, generationID, asOf)
	if err := s.persist(ctx, built); err != nil {
		s.failGeneration(ctx, in.PortfolioID, generationID, "persist_failed", err.Error())
		if mysql.IsDuplicate(err) {
			return fmt.Errorf("%w: %w", ErrFailed, err)
		}
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	if err := s.activate(ctx, in.PortfolioID, generationID); err != nil {
		if errors.Is(err, ErrFailed) {
			s.failGeneration(ctx, in.PortfolioID, generationID, "activate_failed", err.Error())
		}
		return err
	}
	return nil
}

func (s *Service) failGeneration(ctx context.Context, portfolioID, generationID uuid.UUID, code, detail string) {
	if len(detail) > 1024 {
		detail = detail[:1024]
	}
	_, _ = s.q.MarkProjectionGenerationFailed(ctx, sqlcgen.MarkProjectionGenerationFailedParams{
		CompletedAt:   sql.NullTime{Time: s.now().UTC(), Valid: true},
		FailureCode:   sql.NullString{String: code, Valid: true},
		FailureDetail: sql.NullString{String: detail, Valid: true},
		PortfolioID:   mustBin(portfolioID),
		ID:            mustBin(generationID),
	})
}

func (s *Service) activate(ctx context.Context, portfolioID, generationID uuid.UUID) error {
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockPortfolioActivation(ctx, mustBin(portfolioID)); err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	gen, err := q.LockProjectionGeneration(ctx, sqlcgen.LockProjectionGenerationParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(generationID),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	if gen.Status != "ready" {
		return fmt.Errorf("%w: generation is not ready", ErrFailed)
	}
	if err := q.RetireActiveProjectionGeneration(ctx, sqlcgen.RetireActiveProjectionGenerationParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(generationID),
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	if _, err := q.SetActiveProjectionGeneration(ctx, sqlcgen.SetActiveProjectionGenerationParams{
		ActiveProjectionGenerationID: mustBin(generationID),
		UpdatedAt:                    now,
		ID:                           mustBin(portfolioID),
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	n, err := q.MarkProjectionGenerationActive(ctx, sqlcgen.MarkProjectionGenerationActiveParams{
		ActivatedAt: sql.NullTime{Time: now, Valid: true},
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(generationID),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: generation could not be activated", ErrFailed)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %w", ErrTemporary, err)
	}
	return nil
}
