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
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/secret"
)

const maxLabelLength = 120

// Service owns identity and connection configuration.
type Service struct {
	db              *sql.DB
	q               *sqlcgen.Queries
	box             *secret.Box
	now             func() time.Time
	defaultInterval uint32
	defaultFresh    uint32
}

// NewService constructs an access configuration service.
func NewService(db *sql.DB, defaultInterval, defaultFreshness time.Duration, masterKey []byte) *Service {
	interval := uint32(defaultInterval.Seconds())
	if interval == 0 {
		interval = 900
	}
	fresh := uint32(defaultFreshness.Seconds())
	if fresh == 0 {
		fresh = interval * 2
	}
	svc := &Service{db: db, q: sqlcgen.New(db), now: time.Now, defaultInterval: interval, defaultFresh: fresh}
	if len(masterKey) > 0 {
		box, err := secret.NewBox(masterKey)
		if err == nil {
			svc.box = box
		}
	}
	return svc
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

// Connection is an access configuration without credential plaintext.
type Connection struct {
	ID                        uuid.UUID
	PortfolioID               uuid.UUID
	IdentityID                uuid.UUID
	Label                     string
	Status                    string
	CredentialsConfigured     bool
	CredentialsUpdatedAt      *time.Time
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
	Credentials               *secret.Credentials
	ActorUserID               uuid.UUID
	RequestID                 string
}

// PatchConnectionInput is the accepted patch payload.
type PatchConnectionInput struct {
	Label                     *string
	Status                    *string
	CollectionIntervalSeconds *int
	FreshnessAfterSeconds     *int
	Credentials               *secret.Credentials
	ActorUserID               uuid.UUID
	RequestID                 string
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
		if item.Status == "archived" {
			continue
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
	if current.Status == "archived" {
		return Identity{}, ErrIdentityArchived
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
	if identityFieldsChanged(&current, ref, mode, parent) {
		count, err := s.q.CountIdentityObservations(ctx, sqlcgen.CountIdentityObservationsParams{
			PortfolioID: mustBin(portfolioID),
			IdentityID:  mustBin(identityID),
		})
		if err != nil {
			return Identity{}, err
		}
		if count > 0 {
			return Identity{}, ErrIdentityFrozen
		}
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
	children, err := s.q.CountActiveSubordinates(ctx, sqlcgen.CountActiveSubordinatesParams{
		PortfolioID:      mustBin(portfolioID),
		ParentIdentityID: mustBin(identityID),
	})
	if err != nil {
		return Identity{}, err
	}
	if children > 0 {
		return Identity{}, ErrArchiveSubordinates
	}
	enabled, err := s.q.CountEnabledConnectionsByIdentity(ctx, sqlcgen.CountEnabledConnectionsByIdentityParams{
		PortfolioID: mustBin(portfolioID),
		IdentityID:  mustBin(identityID),
	})
	if err != nil {
		return Identity{}, err
	}
	if enabled > 0 {
		return Identity{}, ErrArchiveConnection
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
		item, err := connectionFromSQL(rows[i].ID, rows[i].PortfolioID, rows[i].IdentityID, rows[i].Label, rows[i].Status, rows[i].CredentialsCiphertext, rows[i].CollectionIntervalSeconds, rows[i].FreshnessAfterSeconds, rows[i].LastValidatedAt, rows[i].Version, rows[i].ConfigJson)
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
	return connectionFromSQL(row.ID, row.PortfolioID, row.IdentityID, row.Label, row.Status, row.CredentialsCiphertext, row.CollectionIntervalSeconds, row.FreshnessAfterSeconds, row.LastValidatedAt, row.Version, row.ConfigJson)
}

// CreateConnection inserts a connection, optionally encrypting credentials.
func (s *Service) CreateConnection(ctx context.Context, portfolioID uuid.UUID, in *CreateConnectionInput) (Connection, error) {
	if in == nil {
		return Connection{}, ErrInvalidStatus
	}
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
	if status != "enabled" && status != "disabled" && status != "invalid" {
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
	ciphertext, nonce, keyVersion, cfg, err := s.sealCredentials(id, portfolioID, in.Credentials, now, nil)
	if err != nil {
		return Connection{}, err
	}
	if status == "enabled" && len(ciphertext) == 0 {
		return Connection{}, ErrCredentialsRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Connection{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertConnection(ctx, sqlcgen.InsertConnectionParams{
		ID:                        mustBin(id),
		PortfolioID:               mustBin(portfolioID),
		IdentityID:                mustBin(in.IdentityID),
		Label:                     label,
		Status:                    status,
		CredentialsCiphertext:     ciphertext,
		CredentialsNonce:          nonce,
		CredentialKeyVersion:      keyVersion,
		ConfigJson:                sqljson.Column(cfg),
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
	created, err := connectionFromSQL(mustBin(id), mustBin(portfolioID), mustBin(in.IdentityID), label, status, ciphertext, interval, fresh, sql.NullTime{}, 1, cfg)
	if err != nil {
		return Connection{}, err
	}
	if err := insertConnectionAudit(ctx, q, in.ActorUserID, in.RequestID, "connection.create", nil, &created, now); err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(); err != nil {
		return Connection{}, err
	}
	return s.GetConnection(ctx, portfolioID, id)
}

// PatchConnection updates a connection when the version matches.
func (s *Service) PatchConnection(ctx context.Context, portfolioID, connectionID uuid.UUID, version uint64, in PatchConnectionInput) (Connection, error) {
	row, err := s.q.GetConnection(ctx, sqlcgen.GetConnectionParams{PortfolioID: mustBin(portfolioID), ID: mustBin(connectionID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	if err != nil {
		return Connection{}, err
	}
	current, err := connectionFromSQL(row.ID, row.PortfolioID, row.IdentityID, row.Label, row.Status, row.CredentialsCiphertext, row.CollectionIntervalSeconds, row.FreshnessAfterSeconds, row.LastValidatedAt, row.Version, row.ConfigJson)
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
	now := s.now().UTC()
	ciphertext, nonce, keyVersion, cfg, err := s.sealCredentials(connectionID, portfolioID, in.Credentials, now, &row)
	if err != nil {
		return Connection{}, err
	}
	if status == "enabled" && len(ciphertext) == 0 {
		return Connection{}, ErrCredentialsRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Connection{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{
		Label:                     label,
		Status:                    status,
		CollectionIntervalSeconds: interval,
		FreshnessAfterSeconds:     fresh,
		CredentialsCiphertext:     ciphertext,
		CredentialsNonce:          nonce,
		CredentialKeyVersion:      keyVersion,
		ConfigJson:                sqljson.Column(cfg),
		UpdatedAt:                 now,
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
	updated, err := connectionFromSQL(mustBin(connectionID), mustBin(portfolioID), mustBin(current.IdentityID), label, status, ciphertext, interval, fresh, row.LastValidatedAt, version+1, cfg)
	if err != nil {
		return Connection{}, err
	}
	if err := insertConnectionAudit(ctx, q, in.ActorUserID, in.RequestID, "connection.update", &current, &updated, now); err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(); err != nil {
		return Connection{}, err
	}
	return s.GetConnection(ctx, portfolioID, connectionID)
}

// DeleteConnection removes a connection when the version matches.
func (s *Service) DeleteConnection(ctx context.Context, portfolioID, connectionID uuid.UUID, version uint64, actor uuid.UUID, requestID string) error {
	current, err := s.GetConnection(ctx, portfolioID, connectionID)
	if err != nil {
		return err
	}
	if current.Version != version {
		return ErrPrecondition
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.DeleteConnection(ctx, sqlcgen.DeleteConnectionParams{
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
	if err := insertConnectionAudit(ctx, q, actor, requestID, "connection.delete", &current, nil, s.now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
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

func identityFieldsChanged(current *Identity, ref, mode string, parent *uuid.UUID) bool {
	if ref != current.ExternalRef || mode != current.AccountMode {
		return true
	}
	switch {
	case current.ParentIdentityID == nil && parent == nil:
		return false
	case current.ParentIdentityID == nil || parent == nil:
		return true
	default:
		return *current.ParentIdentityID != *parent
	}
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

func connectionFromSQL(id, portfolioID, identityID []byte, label, status string, ciphertext []byte, interval, fresh uint32, lastValidated sql.NullTime, version uint64, configJSON []byte) (Connection, error) {
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
		CredentialsUpdatedAt:      credentialsUpdatedAt(configJSON),
	}
	if lastValidated.Valid {
		t := lastValidated.Time
		item.LastValidatedAt = &t
	}
	return item, nil
}

type connectionConfig struct {
	CredentialsUpdatedAt *time.Time `json:"credentialsUpdatedAt,omitempty"`
}

type connectionAudit struct {
	Label                     string     `json:"label"`
	Status                    string     `json:"status"`
	CredentialsConfigured     bool       `json:"credentialsConfigured"`
	CredentialsUpdatedAt      *time.Time `json:"credentialsUpdatedAt,omitempty"`
	CollectionIntervalSeconds int        `json:"collectionIntervalSeconds"`
	FreshnessAfterSeconds     int        `json:"freshnessAfterSeconds"`
}

func (s *Service) sealCredentials(connectionID, portfolioID uuid.UUID, cred *secret.Credentials, now time.Time, current *sqlcgen.GetConnectionRow) ([]byte, []byte, uint16, json.RawMessage, error) {
	cfg := connectionConfig{}
	if current != nil {
		cfg.CredentialsUpdatedAt = credentialsUpdatedAt(current.ConfigJson)
	}
	if cred == nil {
		if current == nil {
			raw, err := json.Marshal(cfg)
			return nil, nil, 0, raw, err
		}
		raw, err := json.Marshal(cfg)
		return current.CredentialsCiphertext, current.CredentialsNonce, current.CredentialKeyVersion, raw, err
	}
	if s.box == nil {
		return nil, nil, 0, nil, ErrMasterKeyRequired
	}
	keyVersion := secret.CurrentKeyVersion()
	ciphertext, nonce, err := s.box.Seal(connectionID, portfolioID, keyVersion, *cred)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	updated := now.UTC()
	cfg.CredentialsUpdatedAt = &updated
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	return ciphertext, nonce, keyVersion, raw, nil
}

func credentialsUpdatedAt(raw []byte) *time.Time {
	if len(raw) == 0 {
		return nil
	}
	var cfg connectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	return cfg.CredentialsUpdatedAt
}

func insertConnectionAudit(ctx context.Context, q *sqlcgen.Queries, actor uuid.UUID, requestID, action string, before, after *Connection, now time.Time) error {
	if actor == uuid.Nil {
		return nil
	}
	item := after
	if item == nil {
		item = before
	}
	if item == nil {
		return nil
	}
	auditID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var beforeJSON, afterJSON json.RawMessage
	if before != nil {
		beforeJSON, err = json.Marshal(connectionAuditFrom(before))
		if err != nil {
			return err
		}
	}
	if after != nil {
		afterJSON, err = json.Marshal(connectionAuditFrom(after))
		if err != nil {
			return err
		}
	}
	return q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:           mustBin(auditID),
		PortfolioID:  mustBin(item.PortfolioID),
		ActorUserID:  mustBin(actor),
		Action:       action,
		ResourceType: "connection",
		ResourceID:   mustBin(item.ID),
		RequestID:    defaultRequestID(requestID),
		BeforeJson:   sqljson.Column(beforeJSON),
		AfterJson:    sqljson.Column(afterJSON),
		OccurredAt:   now,
	})
}

func connectionAuditFrom(item *Connection) connectionAudit {
	return connectionAudit{
		Label:                     item.Label,
		Status:                    item.Status,
		CredentialsConfigured:     item.CredentialsConfigured,
		CredentialsUpdatedAt:      item.CredentialsUpdatedAt,
		CollectionIntervalSeconds: item.CollectionIntervalSeconds,
		FreshnessAfterSeconds:     item.FreshnessAfterSeconds,
	}
}

func defaultRequestID(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
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
