package access

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const maxLabelLength = 120

// Service owns identity and connection configuration.
type Service struct {
	q               *sqlcgen.Queries
	now             func() time.Time
	defaultInterval uint32
	defaultFresh    uint32
}

// NewService constructs an access configuration service.
func NewService(db *sql.DB, defaultInterval, defaultFreshness time.Duration) *Service {
	interval := uint32(defaultInterval.Seconds())
	if interval == 0 {
		interval = 900
	}
	fresh := uint32(defaultFreshness.Seconds())
	if fresh == 0 {
		fresh = interval * 2
	}
	return &Service{q: sqlcgen.New(db), now: time.Now, defaultInterval: interval, defaultFresh: fresh}
}

// Identity is a configured platform identity.
type Identity struct {
	ID               uuid.UUID
	PortfolioID      uuid.UUID
	PlatformID       uuid.UUID
	ParentIdentityID *uuid.UUID
	ExternalRef      string
	DisplayName      string
	AccountMode      string
	Kind             string
	Status           string
	Version          uint64
}

// Connection is a credential-less access configuration.
type Connection struct {
	ID                        uuid.UUID
	PortfolioID               uuid.UUID
	IdentityID                uuid.UUID
	Label                     string
	Status                    string
	CredentialsConfigured     bool
	CollectionIntervalSeconds int
	FreshnessAfterSeconds     int
	LastValidatedAt           *time.Time
	Version                   uint64
}

// ETag returns the identity validator.
func (i *Identity) ETag() string {
	return fmt.Sprintf(`"identity:%d"`, i.Version)
}

// ETag returns the connection validator.
func (c *Connection) ETag() string {
	return fmt.Sprintf(`"connection:%d"`, c.Version)
}

// CreateIdentityInput is the accepted create payload.
type CreateIdentityInput struct {
	PlatformID       uuid.UUID
	ParentIdentityID *uuid.UUID
	ExternalRef      string
	DisplayName      string
	AccountMode      string
	Kind             string
}

// PatchIdentityInput is the accepted patch payload.
type PatchIdentityInput struct {
	DisplayName      *string
	Status           *string
	ParentIdentityID *uuid.UUID
	ClearParent      bool
	ExternalRef      *string
	AccountMode      *string
}

// CreateConnectionInput is the accepted create payload.
type CreateConnectionInput struct {
	IdentityID                uuid.UUID
	Label                     string
	Status                    string
	CollectionIntervalSeconds *int
	FreshnessAfterSeconds     *int
}

// PatchConnectionInput is the accepted patch payload.
type PatchConnectionInput struct {
	Label                     *string
	Status                    *string
	CollectionIntervalSeconds *int
	FreshnessAfterSeconds     *int
}

// ListIdentities returns identities in a portfolio.
func (s *Service) ListIdentities(ctx context.Context, portfolioID uuid.UUID) ([]Identity, error) {
	rows, err := s.q.ListIdentitiesByPortfolio(ctx, mustBin(portfolioID))
	if err != nil {
		return nil, err
	}
	out := make([]Identity, 0, len(rows))
	for i := range rows {
		item, err := identityFromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetIdentity returns one identity.
func (s *Service) GetIdentity(ctx context.Context, portfolioID, identityID uuid.UUID) (Identity, error) {
	row, err := s.q.GetIdentity(ctx, sqlcgen.GetIdentityParams{PortfolioID: mustBin(portfolioID), ID: mustBin(identityID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, err
	}
	return identityFromRow(&row)
}

// CreateIdentity inserts an identity.
func (s *Service) CreateIdentity(ctx context.Context, portfolioID uuid.UUID, in *CreateIdentityInput) (Identity, error) {
	if !catalog.AccountModeSupported(in.AccountMode) {
		return Identity{}, ErrInvalidAccountMode
	}
	if err := s.validatePlatform(ctx, in.PlatformID); err != nil {
		return Identity{}, err
	}
	kind := strings.TrimSpace(in.Kind)
	if kind != "main" && kind != "subordinate" {
		return Identity{}, ErrInvalidKind
	}
	if kind == "main" && in.ParentIdentityID != nil {
		return Identity{}, ErrInvalidKind
	}
	if kind == "subordinate" {
		if in.ParentIdentityID == nil {
			return Identity{}, ErrInvalidKind
		}
		if err := s.validateParent(ctx, portfolioID, *in.ParentIdentityID); err != nil {
			return Identity{}, err
		}
	}
	display := strings.TrimSpace(in.DisplayName)
	ref := strings.TrimSpace(in.ExternalRef)
	if display == "" || ref == "" || len(display) > maxLabelLength {
		return Identity{}, ErrInvalidLabel
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	var parent []byte
	if in.ParentIdentityID != nil {
		parent = mustBin(*in.ParentIdentityID)
	}
	if err := s.q.InsertIdentity(ctx, sqlcgen.InsertIdentityParams{
		ID:               mustBin(id),
		PortfolioID:      mustBin(portfolioID),
		PlatformID:       mustBin(in.PlatformID),
		ParentIdentityID: parent,
		ExternalRef:      ref,
		DisplayName:      display,
		AccountMode:      in.AccountMode,
		Kind:             kind,
		Status:           "enabled",
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		if mysql.IsDuplicate(err) {
			return Identity{}, ErrConflict
		}
		return Identity{}, err
	}
	return s.GetIdentity(ctx, portfolioID, id)
}

// PatchIdentity updates an identity when the version matches.
func (s *Service) PatchIdentity(ctx context.Context, portfolioID, identityID uuid.UUID, version uint64, in PatchIdentityInput) (Identity, error) {
	current, err := s.GetIdentity(ctx, portfolioID, identityID)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	display := current.DisplayName
	if in.DisplayName != nil {
		display = strings.TrimSpace(*in.DisplayName)
		if display == "" || len(display) > maxLabelLength {
			return Identity{}, ErrInvalidLabel
		}
	}
	status := current.Status
	if in.Status != nil {
		status = *in.Status
		if status != "enabled" && status != "disabled" {
			return Identity{}, ErrInvalidStatus
		}
	}
	ref := current.ExternalRef
	if in.ExternalRef != nil {
		ref = strings.TrimSpace(*in.ExternalRef)
		if ref == "" {
			return Identity{}, ErrInvalidLabel
		}
	}
	mode := current.AccountMode
	if in.AccountMode != nil {
		if !catalog.AccountModeSupported(*in.AccountMode) {
			return Identity{}, ErrInvalidAccountMode
		}
		mode = *in.AccountMode
	}
	parent := current.ParentIdentityID
	if in.ClearParent {
		parent = nil
	}
	if in.ParentIdentityID != nil {
		parent = in.ParentIdentityID
	}
	if current.Kind == "main" && parent != nil {
		return Identity{}, ErrInvalidKind
	}
	if current.Kind == "subordinate" && parent == nil {
		return Identity{}, ErrInvalidKind
	}
	if parent != nil {
		if err := s.validateParent(ctx, portfolioID, *parent); err != nil {
			return Identity{}, err
		}
	}
	var parentBytes []byte
	if parent != nil {
		parentBytes = mustBin(*parent)
	}
	n, err := s.q.UpdateIdentity(ctx, sqlcgen.UpdateIdentityParams{
		DisplayName:      display,
		Status:           status,
		ParentIdentityID: parentBytes,
		ExternalRef:      ref,
		AccountMode:      mode,
		UpdatedAt:        s.now().UTC(),
		PortfolioID:      mustBin(portfolioID),
		ID:               mustBin(identityID),
		Version:          version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Identity{}, ErrConflict
		}
		return Identity{}, err
	}
	if n == 0 {
		return s.staleIdentity(ctx, portfolioID, identityID)
	}
	return s.GetIdentity(ctx, portfolioID, identityID)
}

// ArchiveIdentity archives an identity.
func (s *Service) ArchiveIdentity(ctx context.Context, portfolioID, identityID uuid.UUID, version uint64) (Identity, error) {
	current, err := s.GetIdentity(ctx, portfolioID, identityID)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	now := s.now().UTC()
	n, err := s.q.ArchiveIdentity(ctx, sqlcgen.ArchiveIdentityParams{
		ArchivedAt:  sql.NullTime{Time: now, Valid: true},
		UpdatedAt:   now,
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(identityID),
		Version:     version,
	})
	if err != nil {
		return Identity{}, err
	}
	if n == 0 {
		return s.staleIdentity(ctx, portfolioID, identityID)
	}
	return s.GetIdentity(ctx, portfolioID, identityID)
}

// ListConnections returns connections without credential material.
func (s *Service) ListConnections(ctx context.Context, portfolioID uuid.UUID) ([]Connection, error) {
	rows, err := s.q.ListConnectionsByPortfolio(ctx, mustBin(portfolioID))
	if err != nil {
		return nil, err
	}
	out := make([]Connection, 0, len(rows))
	for i := range rows {
		item, err := connectionFromRow(rows[i].ID, rows[i].PortfolioID, rows[i].IdentityID, rows[i].Label, rows[i].Status, rows[i].CredentialsCiphertext, rows[i].CollectionIntervalSeconds, rows[i].FreshnessAfterSeconds, rows[i].LastValidatedAt, rows[i].Version)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetConnection returns one connection.
func (s *Service) GetConnection(ctx context.Context, portfolioID, connectionID uuid.UUID) (Connection, error) {
	row, err := s.q.GetConnection(ctx, sqlcgen.GetConnectionParams{PortfolioID: mustBin(portfolioID), ID: mustBin(connectionID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	if err != nil {
		return Connection{}, err
	}
	return connectionFromRow(row.ID, row.PortfolioID, row.IdentityID, row.Label, row.Status, row.CredentialsCiphertext, row.CollectionIntervalSeconds, row.FreshnessAfterSeconds, row.LastValidatedAt, row.Version)
}

// CreateConnection inserts a credential-less connection.
func (s *Service) CreateConnection(ctx context.Context, portfolioID uuid.UUID, in CreateConnectionInput) (Connection, error) {
	if _, err := s.GetIdentity(ctx, portfolioID, in.IdentityID); err != nil {
		return Connection{}, err
	}
	label := strings.TrimSpace(in.Label)
	if label == "" || len(label) > maxLabelLength {
		return Connection{}, ErrInvalidLabel
	}
	status := in.Status
	if status == "" {
		status = "disabled"
	}
	if status == "enabled" {
		return Connection{}, ErrCredentialsRequired
	}
	if status != "disabled" && status != "invalid" {
		return Connection{}, ErrInvalidStatus
	}
	interval := s.defaultInterval
	if in.CollectionIntervalSeconds != nil {
		if *in.CollectionIntervalSeconds < 1 {
			return Connection{}, ErrInvalidStatus
		}
		interval = uint32(*in.CollectionIntervalSeconds)
	}
	fresh := s.defaultFresh
	if in.FreshnessAfterSeconds != nil {
		if *in.FreshnessAfterSeconds < 1 {
			return Connection{}, ErrInvalidStatus
		}
		fresh = uint32(*in.FreshnessAfterSeconds)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Connection{}, err
	}
	now := s.now().UTC()
	if err := s.q.InsertConnection(ctx, sqlcgen.InsertConnectionParams{
		ID:                        mustBin(id),
		PortfolioID:               mustBin(portfolioID),
		IdentityID:                mustBin(in.IdentityID),
		Label:                     label,
		Status:                    status,
		CredentialKeyVersion:      0,
		ConfigJson:                json.RawMessage(`{}`),
		CollectionIntervalSeconds: interval,
		FreshnessAfterSeconds:     fresh,
		Version:                   1,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}); err != nil {
		if mysql.IsDuplicate(err) {
			return Connection{}, ErrConflict
		}
		return Connection{}, err
	}
	return s.GetConnection(ctx, portfolioID, id)
}

// PatchConnection updates a connection when the version matches.
func (s *Service) PatchConnection(ctx context.Context, portfolioID, connectionID uuid.UUID, version uint64, in PatchConnectionInput) (Connection, error) {
	current, err := s.GetConnection(ctx, portfolioID, connectionID)
	if err != nil {
		return Connection{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	label := current.Label
	if in.Label != nil {
		label = strings.TrimSpace(*in.Label)
		if label == "" || len(label) > maxLabelLength {
			return Connection{}, ErrInvalidLabel
		}
	}
	status := current.Status
	if in.Status != nil {
		status = *in.Status
		if status != "enabled" && status != "disabled" && status != "invalid" {
			return Connection{}, ErrInvalidStatus
		}
		if status == "enabled" && !current.CredentialsConfigured {
			return Connection{}, ErrCredentialsRequired
		}
	}
	interval := uint32(current.CollectionIntervalSeconds)
	if in.CollectionIntervalSeconds != nil {
		if *in.CollectionIntervalSeconds < 1 {
			return Connection{}, ErrInvalidStatus
		}
		interval = uint32(*in.CollectionIntervalSeconds)
	}
	fresh := uint32(current.FreshnessAfterSeconds)
	if in.FreshnessAfterSeconds != nil {
		if *in.FreshnessAfterSeconds < 1 {
			return Connection{}, ErrInvalidStatus
		}
		fresh = uint32(*in.FreshnessAfterSeconds)
	}
	n, err := s.q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{
		Label:                     label,
		Status:                    status,
		CollectionIntervalSeconds: interval,
		FreshnessAfterSeconds:     fresh,
		UpdatedAt:                 s.now().UTC(),
		PortfolioID:               mustBin(portfolioID),
		ID:                        mustBin(connectionID),
		Version:                   version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Connection{}, ErrConflict
		}
		return Connection{}, err
	}
	if n == 0 {
		return s.staleConnection(ctx, portfolioID, connectionID)
	}
	return s.GetConnection(ctx, portfolioID, connectionID)
}

// DeleteConnection removes a connection when the version matches.
func (s *Service) DeleteConnection(ctx context.Context, portfolioID, connectionID uuid.UUID, version uint64) error {
	current, err := s.GetConnection(ctx, portfolioID, connectionID)
	if err != nil {
		return err
	}
	if current.Version != version {
		return ErrPrecondition
	}
	n, err := s.q.DeleteConnection(ctx, sqlcgen.DeleteConnectionParams{
		PortfolioID: mustBin(portfolioID),
		ID:          mustBin(connectionID),
		Version:     version,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		_, err := s.staleConnection(ctx, portfolioID, connectionID)
		return err
	}
	return nil
}

func (s *Service) validatePlatform(ctx context.Context, platformID uuid.UUID) error {
	row, err := s.q.GetPlatformByID(ctx, mustBin(platformID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidPlatform
	}
	if err != nil {
		return err
	}
	if row.Status != "enabled" {
		return ErrInvalidPlatform
	}
	return nil
}

func (s *Service) validateParent(ctx context.Context, portfolioID, parentID uuid.UUID) error {
	parent, err := s.GetIdentity(ctx, portfolioID, parentID)
	if err != nil {
		return ErrInvalidParent
	}
	if parent.Status == "archived" || parent.Kind != "main" {
		return ErrInvalidParent
	}
	return nil
}

func (s *Service) staleIdentity(ctx context.Context, portfolioID, identityID uuid.UUID) (Identity, error) {
	current, err := s.GetIdentity(ctx, portfolioID, identityID)
	if err != nil {
		return Identity{}, err
	}
	return current, ErrPrecondition
}

func (s *Service) staleConnection(ctx context.Context, portfolioID, connectionID uuid.UUID) (Connection, error) {
	current, err := s.GetConnection(ctx, portfolioID, connectionID)
	if err != nil {
		return Connection{}, err
	}
	return current, ErrPrecondition
}

func identityFromRow(row *sqlcgen.Identity) (Identity, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Identity{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Identity{}, err
	}
	platformID, err := uuid.FromBytes(row.PlatformID)
	if err != nil {
		return Identity{}, err
	}
	item := Identity{
		ID:          id,
		PortfolioID: portfolioID,
		PlatformID:  platformID,
		ExternalRef: row.ExternalRef,
		DisplayName: row.DisplayName,
		AccountMode: row.AccountMode,
		Kind:        row.Kind,
		Status:      row.Status,
		Version:     row.Version,
	}
	if len(row.ParentIdentityID) == 16 {
		parent, err := uuid.FromBytes(row.ParentIdentityID)
		if err != nil {
			return Identity{}, err
		}
		item.ParentIdentityID = &parent
	}
	return item, nil
}

func connectionFromRow(id, portfolioID, identityID []byte, label, status string, ciphertext []byte, interval, fresh uint32, lastValidated sql.NullTime, version uint64) (Connection, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Connection{}, err
	}
	parsedPortfolio, err := uuid.FromBytes(portfolioID)
	if err != nil {
		return Connection{}, err
	}
	parsedIdentity, err := uuid.FromBytes(identityID)
	if err != nil {
		return Connection{}, err
	}
	item := Connection{
		ID:                        parsedID,
		PortfolioID:               parsedPortfolio,
		IdentityID:                parsedIdentity,
		Label:                     label,
		Status:                    status,
		CredentialsConfigured:     len(ciphertext) > 0,
		CollectionIntervalSeconds: int(interval),
		FreshnessAfterSeconds:     int(fresh),
		Version:                   version,
	}
	if lastValidated.Valid {
		t := lastValidated.Time
		item.LastValidatedAt = &t
	}
	return item, nil
}

func mustBin(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return nil
	}
	return b
}

// ParseIdentityETag extracts the version from an identity ETag.
func ParseIdentityETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"identity:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

// ParseConnectionETag extracts the version from a connection ETag.
func ParseConnectionETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"connection:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}
