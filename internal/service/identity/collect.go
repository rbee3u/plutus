package identity

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
	"github.com/rbee3u/plutus/internal/service/ordering"
)

// Collectable is an enabled identity with decrypted credentials and its selected accounts.
type Collectable struct {
	Identity    Identity
	Credentials adapter.Credentials
	Accounts    []Account
	OpenError   error
}

// ListCollectable returns enabled identities with an enabled connection and their selected accounts.
// Credentials that cannot be opened are reported through OpenError instead of failing the cycle.
func (s *Service) ListCollectable(ctx context.Context, portfolioID uuid.UUID) ([]Collectable, error) {
	rows, err := mysql.Queries(ctx, s.db).ListCollectableIdentities(ctx, portfolioID[:])
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
			ID: id, PortfolioID: portfolioID, Platform: adapter.Platform(rows[i].Platform), DisplayName: rows[i].DisplayName,
			AccountStructure: adapter.AccountStructure(rows[i].AccountStructure), MarginMode: adapter.MarginMode(rows[i].LastObservedMarginMode.String),
			ExternalRef: nullStringPtr(rows[i].PlatformLoginRef), Version: rows[i].Version,
		}}
		accounts, err := mysql.Queries(ctx, s.db).ListAccountsByIdentity(ctx, sqlcgen.ListAccountsByIdentityParams{PortfolioID: portfolioID[:], IdentityID: id[:]})
		if err != nil {
			return nil, err
		}
		for j := range accounts {
			if !accounts[j].Selected || accounts[j].AccountType == string(adapter.TypeManual) {
				continue
			}
			account, err := accountFromFields(accounts[j].ID, accounts[j].IdentityID, accounts[j].AccountType, accounts[j].Selected)
			if err != nil {
				return nil, err
			}
			item.Accounts = append(item.Accounts, account)
		}
		sortIdentityAccounts(&item.Identity, item.Accounts)
		item.Credentials, item.OpenError = s.open(id, portfolioID, rows[i].CredentialsKeyVersion, rows[i].CredentialsCiphertext, rows[i].CredentialsNonce)
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b Collectable) int {
		return ordering.CompareIdentities(identityOrder(&a.Identity), identityOrder(&b.Identity))
	})
	return out, nil
}

// RecordCollection stores an identity's outcome in the report transaction.
func (s *Service) RecordCollection(ctx context.Context, q *sqlcgen.Queries, portfolioID, id uuid.UUID, version uint64, at time.Time, errorClass, errorDetail string, invalidate bool) error {
	params := sqlcgen.RecordIdentityCollectionParams{
		LastCollectionFinishedAt: sql.NullTime{Time: at.UTC(), Valid: true},
		Invalidate:               0,
		UpdatedAt:                s.now().UTC(),
		PortfolioID:              portfolioID[:],
		ID:                       id[:], Version: version,
	}
	if invalidate {
		params.Invalidate = 1
	}
	if errorClass != "" {
		params.LastErrorClass = sql.NullString{String: errorClass, Valid: true}
		params.LastErrorDetail = sql.NullString{String: errorDetail, Valid: true}
	}
	return q.RecordIdentityCollection(ctx, params)
}

// RecordMarginMode stores the margin mode a collection run observed. A change
// is written to the audit log with the system as the actor, because nobody
// changed it in Plutus: the owner switched it on the platform (ADR 0001). It uses
// the report transaction. An empty mode from structures without one is ignored.
func (s *Service) RecordMarginMode(ctx context.Context, q *sqlcgen.Queries, portfolioID, id uuid.UUID, version uint64, mode adapter.MarginMode) error {
	if mode == "" {
		return nil
	}
	stored, err := q.GetIdentityMarginModeForUpdate(ctx, sqlcgen.GetIdentityMarginModeForUpdateParams{PortfolioID: portfolioID[:], ID: id[:], Version: version})
	if errors.Is(err, sql.ErrNoRows) {
		return nil // A concurrent identity edit supersedes these diagnostics.
	}
	if err != nil {
		return err
	}
	previous := adapter.MarginMode(stored.String)
	if previous == mode {
		return nil
	}
	now := s.now().UTC()
	if err := q.SetIdentityMarginMode(ctx, sqlcgen.SetIdentityMarginModeParams{
		LastObservedMarginMode: sql.NullString{String: string(mode), Valid: true}, UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version,
	}); err != nil {
		return err
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, System: true, Action: "identity.margin_mode.change", ResourceType: "identity", ResourceID: id,
		Before: map[string]any{"marginMode": nullableMode(previous)}, After: map[string]any{"marginMode": string(mode)},
	}); err != nil {
		return err
	}
	return nil
}

func nullableMode(mode adapter.MarginMode) any {
	if mode == "" {
		return nil
	}
	return string(mode)
}

// BoundaryAccount is one selected account of an enabled identity.
type BoundaryAccount struct {
	Account
	Platform         adapter.Platform
	IdentityName     string
	ConnectionStatus string
	accountStructure adapter.AccountStructure
}

// ListBoundary returns every selected account of enabled identities.
func (s *Service) ListBoundary(ctx context.Context, portfolioID uuid.UUID) ([]BoundaryAccount, error) {
	rows, err := mysql.Queries(ctx, s.db).ListBoundaryAccounts(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	out := make([]BoundaryAccount, 0, len(rows))
	for i := range rows {
		account, err := accountFromFields(rows[i].ID, rows[i].IdentityID, rows[i].AccountType, true)
		if err != nil {
			return nil, err
		}
		out = append(out, BoundaryAccount{Account: account, Platform: adapter.Platform(rows[i].Platform), IdentityName: rows[i].IdentityName, ConnectionStatus: rows[i].ConnectionStatus, accountStructure: adapter.AccountStructure(rows[i].AccountStructure)})
	}
	slices.SortFunc(out, func(a, b BoundaryAccount) int {
		key := func(item BoundaryAccount) ordering.Account {
			return ordering.Account{Identity: ordering.Identity{Platform: string(item.Platform), Name: item.IdentityName, ID: item.IdentityID}, Structure: item.accountStructure, Type: item.AccountType, ID: item.ID}
		}
		return ordering.CompareAccounts(key(a), key(b))
	})
	return out, nil
}
