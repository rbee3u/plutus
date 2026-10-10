// Package identity owns Identities, their Accounts, and the Connection embedded in each Identity.
package identity

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/rbee3u/plutus/internal/shared/textkey"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/secret"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/etag"
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
	// AccountStructure is declared at creation and never changes (ADR 0001).
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
	return etag.Format("identity", i.Version)
}

// ParseETag extracts the version from an identity ETag.
func ParseETag(value string) (uint64, bool) {
	return etag.Parse("identity", value)
}

// Connection is the credential and scheduling state embedded in an Identity.
type Connection struct {
	Status                    string
	CredentialsConfigured     bool
	CredentialsSetAt          *time.Time
	LastValidationSucceededAt *time.Time
	LastCollectionFinishedAt  *time.Time
	LastErrorClass            *string
	LastErrorDetail           *string
}

// Account is one collectable Account Type of an Identity.
type Account struct {
	ID          uuid.UUID
	IdentityID  uuid.UUID
	AccountType adapters.AccountType
	DisplayName string
	Selected    bool
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
	ID       uuid.UUID
	Selected *bool
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
	rows, err := mysql.Queries(ctx, s.db).ListIdentitiesByPortfolio(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	accounts, err := mysql.Queries(ctx, s.db).ListAccountsByPortfolio(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	byIdentity := map[uuid.UUID][]Account{}
	for i := range accounts {
		account, err := accountFromFields(accounts[i].ID, accounts[i].IdentityID, accounts[i].AccountType, accounts[i].Selected)
		if err != nil {
			return nil, err
		}
		byIdentity[account.IdentityID] = append(byIdentity[account.IdentityID], account)
	}
	out := make([]Identity, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		item, err := fromRow((*sqlcgen.GetIdentityRow)(r))
		if err != nil {
			return nil, err
		}
		item.Accounts = byIdentity[item.ID]
		if item.Accounts == nil {
			item.Accounts = []Account{}
		}
		sortIdentityAccounts(&item, item.Accounts)
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b Identity) int { return ordering.CompareIdentities(identityOrder(&a), identityOrder(&b)) })
	return out, nil
}

// Get returns one Identity with its Accounts.
func (s *Service) Get(ctx context.Context, portfolioID, id uuid.UUID) (Identity, error) {
	r, err := mysql.Queries(ctx, s.db).GetIdentity(ctx, sqlcgen.GetIdentityParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, err
	}
	item, err := fromRow(&r)
	if err != nil {
		return Identity{}, err
	}
	accounts, err := mysql.Queries(ctx, s.db).ListAccountsByIdentity(ctx, sqlcgen.ListAccountsByIdentityParams{PortfolioID: portfolioID[:], IdentityID: id[:]})
	if err != nil {
		return Identity{}, err
	}
	item.Accounts = make([]Account, 0, len(accounts))
	for i := range accounts {
		account, err := accountFromFields(accounts[i].ID, accounts[i].IdentityID, accounts[i].AccountType, accounts[i].Selected)
		if err != nil {
			return Identity{}, err
		}
		item.Accounts = append(item.Accounts, account)
	}
	sortIdentityAccounts(&item, item.Accounts)
	return item, nil
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
	name, valid := textkey.Name(raw, maxNameLength)
	if !valid {
		return "", ErrInvalid
	}
	return name, nil
}

func accountFromFields(id, identityID []byte, accountType string, selected bool) (Account, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Account{}, err
	}
	parsedIdentity, err := uuid.FromBytes(identityID)
	if err != nil {
		return Account{}, err
	}
	return Account{ID: parsedID, IdentityID: parsedIdentity, AccountType: adapters.AccountType(accountType), DisplayName: adapters.AccountName(adapters.AccountType(accountType)), Selected: selected}, nil
}

func fromRow(row *sqlcgen.GetIdentityRow) (Identity, error) {
	parsedID, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Identity{}, err
	}
	parsedPortfolio, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Identity{}, err
	}
	item := Identity{
		ID:               parsedID,
		PortfolioID:      parsedPortfolio,
		Platform:         adapters.Platform(row.Platform),
		DisplayName:      row.DisplayName,
		AccountStructure: adapters.AccountStructure(row.AccountStructure),
		MarginMode:       adapters.MarginMode(row.LastObservedMarginMode.String),
		ExternalRef:      nullStringPtr(row.PlatformLoginRef),
		Status:           StatusEnabled,
		ArchivedAt:       nullTimePtr(row.ArchivedAt),
		Connection: Connection{
			Status:                    row.ConnectionStatus,
			CredentialsConfigured:     row.CredentialsConfigured,
			CredentialsSetAt:          nullTimePtr(row.CredentialsSetAt),
			LastValidationSucceededAt: nullTimePtr(row.LastValidationSucceededAt),
			LastCollectionFinishedAt:  nullTimePtr(row.LastCollectionFinishedAt),
			LastErrorClass:            nullStringPtr(row.LastErrorClass),
			LastErrorDetail:           nullStringPtr(row.LastErrorDetail),
		},
		Version:   row.Version,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if row.ArchivedAt.Valid {
		item.Status = StatusArchived
	}
	return item, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return new(v.Time)
}

func selectedTypes(accounts []Account) []adapters.AccountType {
	var types []adapters.AccountType
	for _, account := range accounts {
		if account.Selected && account.AccountType != adapters.TypeManual {
			types = append(types, account.AccountType)
		}
	}
	return types
}
