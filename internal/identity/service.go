// Package identity owns Identities, their Accounts, and the Connection embedded in each Identity.
package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/secret"
)

const (
	maxNameLength = 120

	// Identity statuses.
	StatusEnabled  = "enabled"
	StatusArchived = "archived"

	// Connection statuses.
	ConnectionUnconfigured = "unconfigured"
	ConnectionDisabled     = "disabled"
	ConnectionEnabled      = "enabled"
	ConnectionInvalid      = "invalid"
)

// Service owns Identity use cases.
type Service struct {
	db              *sql.DB
	q               *sqlcgen.Queries
	now             func() time.Time
	box             *secret.Box
	adapters        *adapters.Registry
	validateTimeout time.Duration
}

// Options configures the identity service.
type Options struct {
	MasterKey       []byte
	Adapters        *adapters.Registry
	ValidateTimeout time.Duration
}

// NewService constructs an identity service. A missing master key disables credential writes.
func NewService(db *sql.DB, opts Options) (*Service, error) {
	s := &Service{db: db, q: sqlcgen.New(db), now: time.Now, adapters: opts.Adapters, validateTimeout: opts.ValidateTimeout}
	if s.adapters == nil {
		s.adapters = adapters.NewRegistry()
	}
	if s.validateTimeout <= 0 {
		s.validateTimeout = 15 * time.Second
	}
	if len(opts.MasterKey) > 0 {
		box, err := secret.NewBox(opts.MasterKey)
		if err != nil {
			return nil, err
		}
		s.box = box
	}
	return s, nil
}

// Identity is one Platform login and its Accounts.
type Identity struct {
	ID          uuid.UUID
	PortfolioID uuid.UUID
	Platform    adapters.Platform
	DisplayName string
	// AccountStructure is declared at creation and never changes (ADR 0028).
	AccountStructure adapters.AccountStructure
	// MarginMode is the last Margin Mode the Platform reported; empty before the first successful
	// validation and for structures that have none.
	MarginMode  adapters.MarginMode
	ExternalRef *string
	Status      string
	ArchivedAt  *time.Time
	Connection  Connection
	Accounts    []Account
	Version     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ETag returns the identity validator.
func (i *Identity) ETag() string {
	return fmt.Sprintf(`"identity:%d"`, i.Version)
}

// ParseETag extracts the version from an identity ETag.
func ParseETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"identity:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

// Connection is the credential and scheduling state embedded in an Identity.
type Connection struct {
	Status                string
	CredentialsConfigured bool
	CredentialsUpdatedAt  *time.Time
	LastValidatedAt       *time.Time
	LastCollectedAt       *time.Time
	LastErrorClass        *string
	LastErrorDetail       *string
}

// Account is one collectable Account Type of an Identity.
type Account struct {
	ID          uuid.UUID
	IdentityID  uuid.UUID
	AccountType adapters.AccountType
	DisplayName string
	Selected    bool
	Version     uint64
}

// CreateInput creates an Identity with its Accounts.
type CreateInput struct {
	ActorUserID          uuid.UUID
	RequestID            string
	Platform             adapters.Platform
	DisplayName          string
	AccountStructure     adapters.AccountStructure
	SelectedAccountTypes []adapters.AccountType
}

// AccountPatch changes one Account.
type AccountPatch struct {
	ID          uuid.UUID
	Selected    *bool
	DisplayName *string
}

// PatchInput renames an Identity or toggles its Accounts.
type PatchInput struct {
	ActorUserID uuid.UUID
	RequestID   string
	DisplayName *string
	Accounts    []AccountPatch
}

// ActionInput identifies the actor of a connection action.
type ActionInput struct {
	ActorUserID uuid.UUID
	RequestID   string
}

// ValidationOutcome is the stored result of one validation call.
type ValidationOutcome struct {
	OK          bool
	ExternalRef string
	MarginMode  adapters.MarginMode
	ErrorClass  string
	ErrorDetail string
	ValidatedAt time.Time
}

// List returns every Identity of a Portfolio with its Accounts.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID) ([]Identity, error) {
	rows, err := s.q.ListIdentitiesByPortfolio(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	accounts, err := s.q.ListAccountsByPortfolio(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	byIdentity := map[uuid.UUID][]Account{}
	for i := range accounts {
		account, err := accountFromFields(accounts[i].ID, accounts[i].IdentityID, accounts[i].AccountType, accounts[i].DisplayName, accounts[i].Selected, accounts[i].Version)
		if err != nil {
			return nil, err
		}
		byIdentity[account.IdentityID] = append(byIdentity[account.IdentityID], account)
	}
	out := make([]Identity, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		item, err := fromFields(r.ID, r.PortfolioID, r.Platform, r.DisplayName, r.AccountStructure, r.MarginMode, r.ExternalRef, r.Status, r.ArchivedAt, r.ConnectionStatus, r.CredentialsConfigured, r.CredentialsUpdatedAt, r.LastValidatedAt, r.LastCollectedAt, r.LastErrorClass, r.LastErrorDetail, r.Version, r.CreatedAt, r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		item.Accounts = byIdentity[item.ID]
		if item.Accounts == nil {
			item.Accounts = []Account{}
		}
		out = append(out, item)
	}
	return out, nil
}

// Get returns one Identity with its Accounts.
func (s *Service) Get(ctx context.Context, portfolioID, id uuid.UUID) (Identity, error) {
	r, err := s.q.GetIdentity(ctx, sqlcgen.GetIdentityParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, err
	}
	item, err := fromFields(r.ID, r.PortfolioID, r.Platform, r.DisplayName, r.AccountStructure, r.MarginMode, r.ExternalRef, r.Status, r.ArchivedAt, r.ConnectionStatus, r.CredentialsConfigured, r.CredentialsUpdatedAt, r.LastValidatedAt, r.LastCollectedAt, r.LastErrorClass, r.LastErrorDetail, r.Version, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return Identity{}, err
	}
	accounts, err := s.q.ListAccountsByIdentity(ctx, sqlcgen.ListAccountsByIdentityParams{PortfolioID: portfolioID[:], IdentityID: id[:]})
	if err != nil {
		return Identity{}, err
	}
	item.Accounts = make([]Account, 0, len(accounts))
	for i := range accounts {
		account, err := accountFromFields(accounts[i].ID, accounts[i].IdentityID, accounts[i].AccountType, accounts[i].DisplayName, accounts[i].Selected, accounts[i].Version)
		if err != nil {
			return Identity{}, err
		}
		item.Accounts = append(item.Accounts, account)
	}
	return item, nil
}

// Create inserts an Identity and one Account per Account Type of its Account Structure.
func (s *Service) Create(ctx context.Context, portfolioID uuid.UUID, in *CreateInput) (Identity, error) {
	name, err := normalizeName(in.DisplayName)
	if err != nil {
		return Identity{}, err
	}
	if !adapters.ValidPlatform(in.Platform) || !adapters.ValidStructure(in.Platform, in.AccountStructure) {
		return Identity{}, ErrInvalid
	}
	selected := map[adapters.AccountType]bool{}
	for _, t := range in.SelectedAccountTypes {
		if !adapters.ValidAccountType(in.AccountStructure, t) {
			return Identity{}, ErrInvalid
		}
		selected[t] = true
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertIdentity(ctx, sqlcgen.InsertIdentityParams{
		ID:               id[:],
		PortfolioID:      portfolioID[:],
		Platform:         string(in.Platform),
		DisplayName:      name,
		AccountStructure: string(in.AccountStructure),
		Status:           StatusEnabled,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		if mysql.IsDuplicate(err) {
			return Identity{}, ErrConflict
		}
		return Identity{}, err
	}
	for _, info := range adapters.AccountTypes(in.AccountStructure) {
		accountID, err := uuid.NewV7()
		if err != nil {
			return Identity{}, err
		}
		if err := q.InsertAccount(ctx, sqlcgen.InsertAccountParams{
			ID:          accountID[:],
			PortfolioID: portfolioID[:],
			IdentityID:  id[:],
			AccountType: string(info.ID),
			DisplayName: info.Name,
			Selected:    selected[info.ID],
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			return Identity{}, err
		}
	}
	created := Identity{ID: id, PortfolioID: portfolioID, Platform: in.Platform, DisplayName: name, AccountStructure: in.AccountStructure, Status: StatusEnabled, Version: 1}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.create", ResourceType: "identity", ResourceID: id,
		RequestID: in.RequestID, After: auditView(&created), Metadata: map[string]any{"selectedAccountTypes": in.SelectedAccountTypes},
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Patch renames an Identity and updates Account selection when the version matches.
func (s *Service) Patch(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in PatchInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	name := current.DisplayName
	if in.DisplayName != nil {
		name, err = normalizeName(*in.DisplayName)
		if err != nil {
			return Identity{}, err
		}
	}
	known := map[uuid.UUID]Account{}
	for _, a := range current.Accounts {
		known[a.ID] = a
	}
	for _, patch := range in.Accounts {
		if _, ok := known[patch.ID]; !ok {
			return Identity{}, ErrNotFound
		}
		if patch.DisplayName != nil {
			if _, err := normalizeName(*patch.DisplayName); err != nil {
				return Identity{}, err
			}
		}
	}
	if current.Platform == adapters.PlatformKraken && current.Connection.Status == ConnectionEnabled {
		accounts := append([]Account(nil), current.Accounts...)
		for i := range accounts {
			for _, patch := range in.Accounts {
				if patch.ID == accounts[i].ID && patch.Selected != nil {
					accounts[i].Selected = *patch.Selected
				}
			}
		}
		creds, err := s.credentials(ctx, portfolioID, id)
		if err != nil {
			return Identity{}, err
		}
		if err := adapters.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(accounts), creds); err != nil {
			return Identity{}, fmt.Errorf("%w: %w Disable the Connection to change Accounts before replacing credentials.", ErrInvalid, err)
		}
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateIdentity(ctx, sqlcgen.UpdateIdentityParams{
		DisplayName: name, UpdatedAt: now,
		PortfolioID: portfolioID[:], ID: id[:], Version: version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Identity{}, ErrConflict
		}
		return Identity{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	for _, patch := range in.Accounts {
		account := known[patch.ID]
		if patch.Selected != nil {
			account.Selected = *patch.Selected
		}
		if patch.DisplayName != nil {
			account.DisplayName = strings.TrimSpace(*patch.DisplayName)
		}
		if _, err := q.UpdateAccount(ctx, sqlcgen.UpdateAccountParams{
			DisplayName: account.DisplayName, Selected: account.Selected, UpdatedAt: now,
			PortfolioID: portfolioID[:], ID: account.ID[:],
		}); err != nil {
			return Identity{}, err
		}
		known[patch.ID] = account
	}
	updated := current
	updated.DisplayName = name
	updated.Version++
	updated.Accounts = make([]Account, 0, len(current.Accounts))
	for _, a := range current.Accounts {
		updated.Accounts = append(updated.Accounts, known[a.ID])
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.update", ResourceType: "identity", ResourceID: id,
		RequestID: in.RequestID, Before: auditView(&current), After: auditView(&updated),
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Archive removes an Identity from the boundary. It is refused while the Connection is enabled.
func (s *Service) Archive(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	if current.Connection.Status == ConnectionEnabled {
		return current, ErrConnectionEnabled
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.ArchiveIdentity(ctx, sqlcgen.ArchiveIdentityParams{
		ArchivedAt: sql.NullTime{Time: now, Valid: true}, UpdatedAt: now,
		PortfolioID: portfolioID[:], ID: id[:], Version: version,
	})
	if err != nil {
		return Identity{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	updated := current
	updated.Status = StatusArchived
	updated.ArchivedAt = &now
	updated.Version++
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.archive", ResourceType: "identity", ResourceID: id,
		RequestID: in.RequestID, Before: auditView(&current), After: auditView(&updated),
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// PutCredentials encrypts and stores read-only API credentials. The Connection becomes disabled until enabled.
func (s *Service) PutCredentials(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput, creds adapters.Credentials) (Identity, error) {
	if s.box == nil {
		return Identity{}, ErrMasterKeyRequired
	}
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	if err := adapters.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(current.Accounts), creds); err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	keyVersion := secret.CurrentKeyVersion()
	ciphertext, nonce, err := s.box.Seal(id, portfolioID, keyVersion, secret.Credentials{APIKey: creds.APIKey, APISecret: creds.APISecret, Passphrase: creds.Passphrase, FuturesAPIKey: creds.FuturesKey(), FuturesAPISecret: creds.FuturesSecret()})
	if errors.Is(err, secret.ErrInvalidCredentials) {
		return Identity{}, ErrInvalid
	}
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.SetIdentityCredentials(ctx, sqlcgen.SetIdentityCredentialsParams{
		CredentialsCiphertext: ciphertext, CredentialsNonce: nonce, CredentialKeyVersion: keyVersion,
		CredentialsUpdatedAt: sql.NullTime{Time: now, Valid: true}, UpdatedAt: now,
		PortfolioID: portfolioID[:], ID: id[:], Version: version,
	})
	if err != nil {
		return Identity{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.credentials.set", ResourceType: "identity", ResourceID: id,
		RequestID: in.RequestID, Metadata: map[string]any{"keyVersion": keyVersion},
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// ClearCredentials removes stored credentials and returns the Connection to unconfigured.
func (s *Service) ClearCredentials(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.ClearIdentityCredentials(ctx, sqlcgen.ClearIdentityCredentialsParams{UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version})
	if err != nil {
		return Identity{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.credentials.clear", ResourceType: "identity", ResourceID: id, RequestID: in.RequestID,
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Validate runs one read-only Platform call and stores the outcome without changing enablement,
// except that an authentication or authorization failure marks the Connection invalid.
func (s *Service) Validate(ctx context.Context, portfolioID, id uuid.UUID, in ActionInput) (Identity, ValidationOutcome, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, ValidationOutcome{}, err
	}
	if current.Status == StatusArchived {
		return current, ValidationOutcome{}, ErrArchived
	}
	outcome, err := s.probe(ctx, &current)
	if err != nil {
		return current, ValidationOutcome{}, err
	}
	next := current.Connection.Status
	switch {
	case outcome.OK && next == ConnectionInvalid:
		next = ConnectionDisabled
	case !outcome.OK && adapters.StopsScheduling(outcome.ErrorClass):
		next = ConnectionInvalid
	}
	updated, err := s.storeOutcome(ctx, &current, current.Version, next, &outcome, in, "identity.connection.validate")
	if err != nil {
		return current, outcome, err
	}
	return updated, outcome, nil
}

// Enable validates first and then marks the Connection enabled for scheduling.
func (s *Service) Enable(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, ValidationOutcome, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, ValidationOutcome{}, err
	}
	if current.Version != version {
		return current, ValidationOutcome{}, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ValidationOutcome{}, ErrArchived
	}
	outcome, err := s.probe(ctx, &current)
	if err != nil {
		return current, ValidationOutcome{}, err
	}
	next := ConnectionEnabled
	if !outcome.OK {
		next = current.Connection.Status
		if adapters.StopsScheduling(outcome.ErrorClass) {
			next = ConnectionInvalid
		}
	}
	updated, err := s.storeOutcome(ctx, &current, version, next, &outcome, in, "identity.connection.enable")
	if err != nil {
		return current, outcome, err
	}
	if !outcome.OK {
		return updated, outcome, ErrValidationFailed
	}
	return updated, outcome, nil
}

// Disable stops scheduling the Connection.
func (s *Service) Disable(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in ActionInput) (Identity, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Identity{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	if current.Status == StatusArchived {
		return current, ErrArchived
	}
	if !current.Connection.CredentialsConfigured {
		return current, ErrCredentialsMissing
	}
	outcome := ValidationOutcome{OK: true, ValidatedAt: time.Time{}}
	updated, err := s.storeOutcome(ctx, &current, version, ConnectionDisabled, &outcome, in, "identity.connection.disable")
	if err != nil {
		return current, err
	}
	return updated, nil
}

// probe runs one read-only validation of the stored credentials. Besides the adapter's own
// checks it refuses a key that reaches another Platform account than the one the Identity is
// bound to (its external reference), and a Margin Mode that does not belong to the structure.
func (s *Service) probe(ctx context.Context, current *Identity) (ValidationOutcome, error) {
	creds, err := s.credentials(ctx, current.PortfolioID, current.ID)
	if err != nil {
		return ValidationOutcome{}, err
	}
	adapter, ok := s.adapters.Lookup(current.Platform)
	if !ok {
		return ValidationOutcome{}, ErrAdapterUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, s.validateTimeout)
	defer cancel()
	now := s.now().UTC()
	failed := func(err error) ValidationOutcome {
		class, detail := adapters.Classify(err)
		return ValidationOutcome{ErrorClass: class, ErrorDetail: detail, ValidatedAt: now}
	}
	if err := adapters.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(current.Accounts), creds); err != nil {
		return failed(err), nil
	}
	result, err := adapter.Validate(callCtx, creds, current.AccountStructure)
	if err != nil {
		return failed(err), nil
	}
	if bound := current.ExternalRef; bound != nil && result.ExternalRef != "" && result.ExternalRef != *bound {
		return failed(adapters.NewError(adapters.ClassUnsupported, fmt.Sprintf(
			"This key belongs to %s account %s, but this Identity is account %s; create a new Identity for the other account.",
			adapters.PlatformName(current.Platform), result.ExternalRef, *bound))), nil
	}
	if !adapters.ValidMarginMode(current.AccountStructure, result.MarginMode) {
		return failed(adapters.UnknownMarginMode(current.Platform, string(result.MarginMode))), nil
	}
	return ValidationOutcome{OK: true, ExternalRef: result.ExternalRef, MarginMode: result.MarginMode, ValidatedAt: now}, nil
}

func (s *Service) storeOutcome(ctx context.Context, current *Identity, version uint64, status string, outcome *ValidationOutcome, in ActionInput, action string) (Identity, error) {
	portfolioID, id := current.PortfolioID, current.ID
	now := s.now().UTC()
	params := sqlcgen.SetIdentityConnectionStatusParams{
		ConnectionStatus: status, UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version,
	}
	if outcome.ExternalRef != "" {
		params.ExternalRef = sql.NullString{String: outcome.ExternalRef, Valid: true}
	}
	metadata := map[string]any{"ok": outcome.OK, "status": status, "errorClass": outcome.ErrorClass}
	if outcome.OK && outcome.MarginMode != "" {
		params.MarginMode = sql.NullString{String: string(outcome.MarginMode), Valid: true}
		metadata["marginMode"] = string(outcome.MarginMode)
		if current.MarginMode != outcome.MarginMode {
			metadata["previousMarginMode"] = nullableMode(current.MarginMode)
		}
	}
	if outcome.OK && !outcome.ValidatedAt.IsZero() {
		params.LastValidatedAt = sql.NullTime{Time: outcome.ValidatedAt, Valid: true}
	}
	if !outcome.OK {
		params.LastErrorClass = sql.NullString{String: outcome.ErrorClass, Valid: true}
		params.LastErrorDetail = sql.NullString{String: outcome.ErrorDetail, Valid: true}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.SetIdentityConnectionStatus(ctx, params)
	if err != nil {
		return Identity{}, err
	}
	if n == 0 {
		return Identity{}, ErrPrecondition
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: action, ResourceType: "identity", ResourceID: id, RequestID: in.RequestID,
		Metadata: metadata,
	}); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

func (s *Service) credentials(ctx context.Context, portfolioID, id uuid.UUID) (adapters.Credentials, error) {
	row, err := s.q.GetIdentityCredentials(ctx, sqlcgen.GetIdentityCredentialsParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return adapters.Credentials{}, ErrNotFound
	}
	if err != nil {
		return adapters.Credentials{}, err
	}
	return s.open(id, portfolioID, row.CredentialKeyVersion, row.CredentialsCiphertext, row.CredentialsNonce)
}

func (s *Service) open(id, portfolioID uuid.UUID, keyVersion uint16, ciphertext, nonce []byte) (adapters.Credentials, error) {
	if len(ciphertext) == 0 {
		return adapters.Credentials{}, ErrCredentialsMissing
	}
	if s.box == nil {
		return adapters.Credentials{}, ErrMasterKeyRequired
	}
	creds, err := s.box.Open(id, portfolioID, keyVersion, ciphertext, nonce)
	if err != nil {
		return adapters.Credentials{}, err
	}
	return adapters.Credentials{APIKey: creds.APIKey, APISecret: creds.APISecret, Passphrase: creds.Passphrase, Futures: &adapters.KeyPair{APIKey: creds.FuturesAPIKey, APISecret: creds.FuturesAPISecret}}, nil
}

// Collectable is an enabled Identity with decrypted credentials and its selected Accounts.
type Collectable struct {
	Identity    Identity
	Credentials adapters.Credentials
	Accounts    []Account
	OpenError   error
}

// ListCollectable returns enabled Identities with an enabled Connection and their selected Accounts.
// Credentials that cannot be opened are reported through OpenError instead of failing the cycle.
func (s *Service) ListCollectable(ctx context.Context, portfolioID uuid.UUID) ([]Collectable, error) {
	rows, err := s.q.ListCollectableIdentities(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	out := make([]Collectable, 0, len(rows))
	for i := range rows {
		id, err := uuid.FromBytes(rows[i].ID)
		if err != nil {
			return nil, err
		}
		item := Collectable{Identity: Identity{
			ID: id, PortfolioID: portfolioID, Platform: adapters.Platform(rows[i].Platform), DisplayName: rows[i].DisplayName,
			AccountStructure: adapters.AccountStructure(rows[i].AccountStructure), MarginMode: adapters.MarginMode(rows[i].MarginMode.String),
			ExternalRef: nullStringPtr(rows[i].ExternalRef),
		}}
		accounts, err := s.q.ListAccountsByIdentity(ctx, sqlcgen.ListAccountsByIdentityParams{PortfolioID: portfolioID[:], IdentityID: id[:]})
		if err != nil {
			return nil, err
		}
		for j := range accounts {
			if !accounts[j].Selected {
				continue
			}
			account, err := accountFromFields(accounts[j].ID, accounts[j].IdentityID, accounts[j].AccountType, accounts[j].DisplayName, accounts[j].Selected, accounts[j].Version)
			if err != nil {
				return nil, err
			}
			item.Accounts = append(item.Accounts, account)
		}
		item.Credentials, item.OpenError = s.open(id, portfolioID, rows[i].CredentialKeyVersion, rows[i].CredentialsCiphertext, rows[i].CredentialsNonce)
		out = append(out, item)
	}
	return out, nil
}

// RecordCollection stores the outcome of one collection run for an Identity.
func (s *Service) RecordCollection(ctx context.Context, portfolioID, id uuid.UUID, at time.Time, errorClass, errorDetail string, invalidate bool) error {
	params := sqlcgen.RecordIdentityCollectionParams{
		LastCollectedAt: sql.NullTime{Time: at.UTC(), Valid: true},
		Invalidate:      0,
		UpdatedAt:       s.now().UTC(),
		PortfolioID:     portfolioID[:],
		ID:              id[:],
	}
	if invalidate {
		params.Invalidate = 1
	}
	if errorClass != "" {
		params.LastErrorClass = sql.NullString{String: errorClass, Valid: true}
		params.LastErrorDetail = sql.NullString{String: errorDetail, Valid: true}
	}
	return s.q.RecordIdentityCollection(ctx, params)
}

// RecordMarginMode stores the Margin Mode a Collection Run observed. A change is written to the
// audit log with the system as the actor, because nobody changed it in Plutus: the owner switched
// it on the Platform (ADR 0028). An empty mode, reported by structures without one, is ignored.
func (s *Service) RecordMarginMode(ctx context.Context, portfolioID, id uuid.UUID, mode adapters.MarginMode) error {
	if mode == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	stored, err := q.GetIdentityMarginModeForUpdate(ctx, sqlcgen.GetIdentityMarginModeForUpdateParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	previous := adapters.MarginMode(stored.String)
	if previous == mode {
		return nil
	}
	now := s.now().UTC()
	if err := q.SetIdentityMarginMode(ctx, sqlcgen.SetIdentityMarginModeParams{
		MarginMode: sql.NullString{String: string(mode), Valid: true}, UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:],
	}); err != nil {
		return err
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, System: true, Action: "identity.margin_mode.change", ResourceType: "identity", ResourceID: id,
		Before: map[string]any{"marginMode": nullableMode(previous)}, After: map[string]any{"marginMode": string(mode)},
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func nullableMode(mode adapters.MarginMode) any {
	if mode == "" {
		return nil
	}
	return string(mode)
}

// BoundaryAccount is one selected Account of an enabled Identity.
type BoundaryAccount struct {
	Account
	Platform         adapters.Platform
	IdentityName     string
	ConnectionStatus string
}

// ListBoundary returns every selected Account of enabled Identities.
func (s *Service) ListBoundary(ctx context.Context, portfolioID uuid.UUID) ([]BoundaryAccount, error) {
	rows, err := s.q.ListBoundaryAccounts(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	out := make([]BoundaryAccount, 0, len(rows))
	for i := range rows {
		account, err := accountFromFields(rows[i].ID, rows[i].IdentityID, rows[i].AccountType, rows[i].DisplayName, true, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, BoundaryAccount{Account: account, Platform: adapters.Platform(rows[i].Platform), IdentityName: rows[i].IdentityName, ConnectionStatus: rows[i].ConnectionStatus})
	}
	return out, nil
}

type auditIdentity struct {
	Platform         string         `json:"platform"`
	DisplayName      string         `json:"displayName"`
	AccountStructure string         `json:"accountStructure"`
	MarginMode       string         `json:"marginMode,omitempty"`
	Status           string         `json:"status"`
	Version          uint64         `json:"version"`
	Accounts         map[string]any `json:"accounts,omitempty"`
}

func auditView(i *Identity) auditIdentity {
	view := auditIdentity{
		Platform: string(i.Platform), DisplayName: i.DisplayName, AccountStructure: string(i.AccountStructure),
		MarginMode: string(i.MarginMode), Status: i.Status, Version: i.Version,
	}
	if len(i.Accounts) > 0 {
		view.Accounts = map[string]any{}
		for _, a := range i.Accounts {
			view.Accounts[string(a.AccountType)] = map[string]any{"selected": a.Selected, "displayName": a.DisplayName}
		}
	}
	return view
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > maxNameLength {
		return "", ErrInvalid
	}
	return name, nil
}

func accountFromFields(id, identityID []byte, accountType, displayName string, selected bool, version uint64) (Account, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Account{}, err
	}
	parsedIdentity, err := uuid.FromBytes(identityID)
	if err != nil {
		return Account{}, err
	}
	return Account{ID: parsedID, IdentityID: parsedIdentity, AccountType: adapters.AccountType(accountType), DisplayName: displayName, Selected: selected, Version: version}, nil
}

func fromFields(id, portfolioID []byte, platform, displayName, accountStructure string, marginMode, externalRef sql.NullString, status string, archivedAt sql.NullTime, connectionStatus string, credentialsConfigured bool, credentialsUpdatedAt, lastValidatedAt, lastCollectedAt sql.NullTime, lastErrorClass, lastErrorDetail sql.NullString, version uint64, createdAt, updatedAt time.Time) (Identity, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Identity{}, err
	}
	parsedPortfolio, err := uuid.FromBytes(portfolioID)
	if err != nil {
		return Identity{}, err
	}
	item := Identity{
		ID:               parsedID,
		PortfolioID:      parsedPortfolio,
		Platform:         adapters.Platform(platform),
		DisplayName:      displayName,
		AccountStructure: adapters.AccountStructure(accountStructure),
		MarginMode:       adapters.MarginMode(marginMode.String),
		ExternalRef:      nullStringPtr(externalRef),
		Status:           status,
		ArchivedAt:       nullTimePtr(archivedAt),
		Connection: Connection{
			Status:                connectionStatus,
			CredentialsConfigured: credentialsConfigured,
			CredentialsUpdatedAt:  nullTimePtr(credentialsUpdatedAt),
			LastValidatedAt:       nullTimePtr(lastValidatedAt),
			LastCollectedAt:       nullTimePtr(lastCollectedAt),
			LastErrorClass:        nullStringPtr(lastErrorClass),
			LastErrorDetail:       nullStringPtr(lastErrorDetail),
		},
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	return item, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func selectedTypes(accounts []Account) []adapters.AccountType {
	var types []adapters.AccountType
	for _, account := range accounts {
		if account.Selected {
			types = append(types, account.AccountType)
		}
	}
	return types
}
