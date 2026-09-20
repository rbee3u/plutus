package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	defaultTimezone = "Asia/Shanghai"
	maxNameLength   = 120
)

// Service owns portfolio use cases.
type Service struct {
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a portfolio service.
func NewService(db *sql.DB) *Service {
	return &Service{q: sqlcgen.New(db), now: time.Now}
}

// Portfolio is an owned workspace.
type Portfolio struct {
	ID             uuid.UUID
	Name           string
	Timezone       string
	DisplayAssetID uuid.UUID
	DisplayAsset   string
	Status         string
	Version        uint64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ArchivedAt     *time.Time
}

// ETag returns the strong validator for this revision.
func (p *Portfolio) ETag() string {
	return fmt.Sprintf(`"portfolio:%d"`, p.Version)
}

// CreateInput is the accepted create payload.
type CreateInput struct {
	Name           string
	Timezone       string
	DisplayAssetID *uuid.UUID
}

// PatchInput is the accepted patch payload.
type PatchInput struct {
	Name           *string
	Timezone       *string
	DisplayAssetID *uuid.UUID
}

// List returns owned portfolios.
func (s *Service) List(ctx context.Context, userID uuid.UUID, includeArchived bool) ([]Portfolio, error) {
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListPortfoliosByUser(ctx, sqlcgen.ListPortfoliosByUserParams{
		UserID:          userBytes,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Portfolio, 0, len(rows))
	for i := range rows {
		item, err := fromListRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// Get returns one owned portfolio.
func (s *Service) Get(ctx context.Context, userID, portfolioID uuid.UUID) (Portfolio, error) {
	row, err := s.getRow(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	return fromGetRow(&row)
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
	displayID := catalog.AssetUSD
	if in.DisplayAssetID != nil {
		displayID = *in.DisplayAssetID
	}
	if err := validateDisplayAsset(displayID); err != nil {
		return Portfolio{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Portfolio{}, err
	}
	now := s.now().UTC()
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	assetBytes, err := displayID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	if err := s.q.InsertPortfolio(ctx, sqlcgen.InsertPortfolioParams{
		ID:             idBytes,
		UserID:         userBytes,
		Name:           name,
		Timezone:       timezone,
		DisplayAssetID: assetBytes,
		Status:         "active",
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
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
	displayID := current.DisplayAssetID
	if in.DisplayAssetID != nil {
		displayID = *in.DisplayAssetID
		if err := validateDisplayAsset(displayID); err != nil {
			return Portfolio{}, err
		}
	}
	idBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	assetBytes, err := displayID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	n, err := s.q.UpdatePortfolio(ctx, sqlcgen.UpdatePortfolioParams{
		Name:           name,
		Timezone:       timezone,
		DisplayAssetID: assetBytes,
		UpdatedAt:      s.now().UTC(),
		ID:             idBytes,
		UserID:         userBytes,
		Version:        version,
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

// Archive marks an active portfolio archived.
func (s *Service) Archive(ctx context.Context, userID, portfolioID uuid.UUID, version uint64) (Portfolio, error) {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	idBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	now := s.now().UTC()
	n, err := s.q.ArchivePortfolio(ctx, sqlcgen.ArchivePortfolioParams{
		ArchivedAt: sql.NullTime{Time: now, Valid: true},
		UpdatedAt:  now,
		ID:         idBytes,
		UserID:     userBytes,
		Version:    version,
	})
	if err != nil {
		return Portfolio{}, err
	}
	if n == 0 {
		return s.staleOrMissing(ctx, userID, portfolioID)
	}
	return s.Get(ctx, userID, portfolioID)
}

// Restore reactivates an archived portfolio.
func (s *Service) Restore(ctx context.Context, userID, portfolioID uuid.UUID, version uint64) (Portfolio, error) {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	idBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return Portfolio{}, err
	}
	n, err := s.q.RestorePortfolio(ctx, sqlcgen.RestorePortfolioParams{
		UpdatedAt: s.now().UTC(),
		ID:        idBytes,
		UserID:    userBytes,
		Version:   version,
	})
	if err != nil {
		return Portfolio{}, err
	}
	if n == 0 {
		return s.staleOrMissing(ctx, userID, portfolioID)
	}
	return s.Get(ctx, userID, portfolioID)
}

// Delete removes an empty portfolio.
func (s *Service) Delete(ctx context.Context, userID, portfolioID uuid.UUID, version uint64) error {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return err
	}
	if current.Version != version {
		return ErrPrecondition
	}
	idBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return err
	}
	empty, err := s.empty(ctx, idBytes)
	if err != nil {
		return err
	}
	if !empty {
		return ErrNotEmpty
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return err
	}
	n, err := s.q.DeletePortfolio(ctx, sqlcgen.DeletePortfolioParams{
		ID:      idBytes,
		UserID:  userBytes,
		Version: version,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		_, err := s.staleOrMissing(ctx, userID, portfolioID)
		return err
	}
	return nil
}

func (s *Service) empty(ctx context.Context, portfolioID []byte) (bool, error) {
	n, err := s.q.CountPortfolioDependencies(ctx, sqlcgen.CountPortfolioDependenciesParams{PortfolioID: portfolioID})
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func (s *Service) getRow(ctx context.Context, userID, portfolioID uuid.UUID) (sqlcgen.GetPortfolioByUserAndIDRow, error) {
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return sqlcgen.GetPortfolioByUserAndIDRow{}, err
	}
	idBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return sqlcgen.GetPortfolioByUserAndIDRow{}, err
	}
	row, err := s.q.GetPortfolioByUserAndID(ctx, sqlcgen.GetPortfolioByUserAndIDParams{UserID: userBytes, ID: idBytes})
	if errors.Is(err, sql.ErrNoRows) {
		return sqlcgen.GetPortfolioByUserAndIDRow{}, ErrNotFound
	}
	return row, err
}

func (s *Service) staleOrMissing(ctx context.Context, userID, portfolioID uuid.UUID) (Portfolio, error) {
	current, err := s.Get(ctx, userID, portfolioID)
	if err != nil {
		return Portfolio{}, err
	}
	return current, ErrPrecondition
}

func fromListRow(row *sqlcgen.ListPortfoliosByUserRow) (Portfolio, error) {
	return fromFields(row.ID, row.Name, row.Timezone, row.DisplayAssetID, row.DisplayAssetCode, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.ArchivedAt)
}

func fromGetRow(row *sqlcgen.GetPortfolioByUserAndIDRow) (Portfolio, error) {
	return fromFields(row.ID, row.Name, row.Timezone, row.DisplayAssetID, row.DisplayAssetCode, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.ArchivedAt)
}

func fromFields(id []byte, name, timezone string, displayAssetID []byte, displayAssetCode, status string, version uint64, createdAt, updatedAt time.Time, archivedAt sql.NullTime) (Portfolio, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Portfolio{}, err
	}
	assetID, err := uuid.FromBytes(displayAssetID)
	if err != nil {
		return Portfolio{}, err
	}
	item := Portfolio{
		ID:             parsedID,
		Name:           name,
		Timezone:       timezone,
		DisplayAssetID: assetID,
		DisplayAsset:   displayAssetCode,
		Status:         status,
		Version:        version,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		item.ArchivedAt = &t
	}
	return item, nil
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
		return defaultTimezone, nil
	}
	if _, err := time.LoadLocation(raw); err != nil {
		return "", ErrInvalidTimezone
	}
	return raw, nil
}

func validateDisplayAsset(id uuid.UUID) error {
	if id == catalog.AssetUSD || id == catalog.AssetCNY {
		return nil
	}
	return ErrInvalidDisplayAsset
}

// ParseETag extracts the version from a portfolio ETag.
func ParseETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"portfolio:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}
