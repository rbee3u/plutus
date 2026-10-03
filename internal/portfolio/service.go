// Package portfolio owns Portfolio ownership, settings, and cascade deletion.
package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	// DefaultName is the display name of the portfolio created with a user.
	DefaultName = "default"
	// DefaultTimezone is the IANA zone used when none is given.
	DefaultTimezone = "Asia/Shanghai"
	// DefaultCollectionInterval is the initial Collection Cycle interval for new Portfolios.
	DefaultCollectionInterval = 15 * time.Minute
	maxNameLength             = 120
	minInterval               = 60
	maxInterval               = 86400
)

// Service owns portfolio use cases.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a portfolio service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Portfolio is one person's ledger.
type Portfolio struct {
	ID                        uuid.UUID
	Name                      string
	Timezone                  string
	CollectionIntervalSeconds int
	Undeletable               bool
	Version                   uint64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// ETag returns the strong validator for this revision.
func (p *Portfolio) ETag() string {
	return fmt.Sprintf(`"portfolio:%d"`, p.Version)
}

// Location returns the Portfolio's time zone, falling back to UTC.
func (p *Portfolio) Location() *time.Location {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// CreateInput is the accepted create payload.
type CreateInput struct {
	Name                      string
	Timezone                  string
	CollectionIntervalSeconds *int
}

// PatchInput is the accepted patch payload.
type PatchInput struct {
	Name                      *string
	Timezone                  *string
	CollectionIntervalSeconds *int
}

// List returns owned portfolios.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Portfolio, error) {
	rows, err := s.q.ListPortfoliosByUser(ctx, bin(userID))
	if err != nil {
		return nil, err
	}
	out := make([]Portfolio, 0, len(rows))
	for i := range rows {
		item, err := fromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// Get returns one owned portfolio.
func (s *Service) Get(ctx context.Context, userID, portfolioID uuid.UUID) (Portfolio, error) {
	row, err := s.q.GetPortfolioByUserAndID(ctx, sqlcgen.GetPortfolioByUserAndIDParams{UserID: bin(userID), ID: bin(portfolioID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Portfolio{}, ErrNotFound
	}
	if err != nil {
		return Portfolio{}, err
	}
	return fromRow(&row)
}

// GetByID returns a portfolio regardless of owner (worker and CLI paths).
func (s *Service) GetByID(ctx context.Context, portfolioID uuid.UUID) (Portfolio, error) {
	row, err := s.q.GetPortfolioByID(ctx, bin(portfolioID))
	if errors.Is(err, sql.ErrNoRows) {
		return Portfolio{}, ErrNotFound
	}
	if err != nil {
		return Portfolio{}, err
	}
	return fromRow(&row)
}

// Create inserts a portfolio.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Portfolio, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return Portfolio{}, err
	}
	timezone, err := normalizeTimezone(in.Timezone)
	if err != nil {
		return Portfolio{}, err
	}
	interval := int(DefaultCollectionInterval / time.Second)
	if in.CollectionIntervalSeconds != nil {
		interval = *in.CollectionIntervalSeconds
	}
	if err := validateInterval(interval); err != nil {
		return Portfolio{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Portfolio{}, err
	}
	now := s.now().UTC()
	if err := s.q.InsertPortfolio(ctx, sqlcgen.InsertPortfolioParams{
		ID:                        bin(id),
		UserID:                    bin(userID),
		Name:                      name,
		Timezone:                  timezone,
		CollectionIntervalSeconds: uint32(interval), // #nosec G115 -- The stored interval and validateInterval are bounded to 60..86400 seconds.
		Undeletable:               false,
		Version:                   1,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}); err != nil {
		if mysql.IsDuplicate(err) {
			return Portfolio{}, ErrConflict
		}
		return Portfolio{}, err
	}
	return s.Get(ctx, userID, id)
}

// Patch updates mutable fields when the version matches.
func (s *Service) Patch(ctx context.Context, userID, portfolioID uuid.UUID, version uint64, in PatchInput) (Portfolio, error) {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	name := current.Name
	if in.Name != nil {
		name, err = normalizeName(*in.Name)
		if err != nil {
			return Portfolio{}, err
		}
	}
	timezone := current.Timezone
	if in.Timezone != nil {
		timezone, err = normalizeTimezone(*in.Timezone)
		if err != nil {
			return Portfolio{}, err
		}
	}
	interval := current.CollectionIntervalSeconds
	if in.CollectionIntervalSeconds != nil {
		interval = *in.CollectionIntervalSeconds
		if err := validateInterval(interval); err != nil {
			return Portfolio{}, err
		}
	}
	n, err := s.q.UpdatePortfolio(ctx, sqlcgen.UpdatePortfolioParams{
		Name:                      name,
		Timezone:                  timezone,
		CollectionIntervalSeconds: uint32(interval), // #nosec G115 -- The stored interval and validateInterval are bounded to 60..86400 seconds.
		UpdatedAt:                 s.now().UTC(),
		ID:                        bin(portfolioID),
		UserID:                    bin(userID),
		Version:                   version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Portfolio{}, ErrConflict
		}
		return Portfolio{}, err
	}
	if n == 0 {
		return s.staleOrMissing(ctx, userID, portfolioID)
	}
	return s.Get(ctx, userID, portfolioID)
}

// Delete removes an additional portfolio and every row scoped to it.
func (s *Service) Delete(ctx context.Context, userID, portfolioID uuid.UUID, version uint64) error {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return err
	}
	if current.Version != version {
		return ErrPrecondition
	}
	if current.Undeletable {
		return ErrUndeletable
	}
	err = s.deleteCascade(ctx, bin(userID), bin(portfolioID), version)
	if errors.Is(err, ErrPrecondition) {
		_, stale := s.staleOrMissing(ctx, userID, portfolioID)
		return stale
	}
	return err
}

func (s *Service) staleOrMissing(ctx context.Context, userID, portfolioID uuid.UUID) (Portfolio, error) {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	return current, ErrPrecondition
}

func fromRow(row *sqlcgen.Portfolio) (Portfolio, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Portfolio{}, err
	}
	return Portfolio{
		ID:                        id,
		Name:                      row.Name,
		Timezone:                  row.Timezone,
		CollectionIntervalSeconds: int(row.CollectionIntervalSeconds),
		Undeletable:               row.Undeletable,
		Version:                   row.Version,
		CreatedAt:                 row.CreatedAt,
		UpdatedAt:                 row.UpdatedAt,
	}, nil
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > maxNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}

func normalizeTimezone(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultTimezone, nil
	}
	if _, err := time.LoadLocation(raw); err != nil {
		return "", ErrInvalidTimezone
	}
	return raw, nil
}

func validateInterval(seconds int) error {
	if seconds < minInterval || seconds > maxInterval {
		return ErrInvalidInterval
	}
	return nil
}

// ParseETag extracts the version from a portfolio ETag.
func ParseETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"portfolio:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

func bin(id uuid.UUID) []byte {
	return id[:]
}
