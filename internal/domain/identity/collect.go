package identity

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/audit"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/ordering"
)

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
			if !accounts[j].Selected || accounts[j].AccountType == string(adapters.TypeManual) {
				continue
			}
			account, err := accountFromFields(accounts[j].ID, accounts[j].IdentityID, accounts[j].AccountType, accounts[j].DisplayName, accounts[j].Selected, accounts[j].Version)
			if err != nil {
				return nil, err
			}
			item.Accounts = append(item.Accounts, account)
		}
		sortIdentityAccounts(&item.Identity, item.Accounts)
		item.Credentials, item.OpenError = s.open(id, portfolioID, rows[i].CredentialKeyVersion, rows[i].CredentialsCiphertext, rows[i].CredentialsNonce)
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b Collectable) int {
		return ordering.CompareIdentities(identityOrder(&a.Identity), identityOrder(&b.Identity))
	})
	return out, nil
}

// RecordCollection stores an Identity's outcome in the report transaction.
func (s *Service) RecordCollection(ctx context.Context, q *sqlcgen.Queries, portfolioID, id uuid.UUID, at time.Time, errorClass, errorDetail string, invalidate bool) error {
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
	return q.RecordIdentityCollection(ctx, params)
}

// RecordMarginMode stores the Margin Mode a Collection Run observed. A change
// is written to the audit log with the system as the actor, because nobody
// changed it in Plutus: the owner switched it on the Platform (ADR 0001). It uses
// the report transaction. An empty mode from structures without one is ignored.
func (s *Service) RecordMarginMode(ctx context.Context, q *sqlcgen.Queries, portfolioID, id uuid.UUID, mode adapters.MarginMode) error {
	if mode == "" {
		return nil
	}
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
	return nil
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
	accountStructure adapters.AccountStructure
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
		out = append(out, BoundaryAccount{Account: account, Platform: adapters.Platform(rows[i].Platform), IdentityName: rows[i].IdentityName, ConnectionStatus: rows[i].ConnectionStatus, accountStructure: adapters.AccountStructure(rows[i].AccountStructure)})
	}
	slices.SortFunc(out, func(a, b BoundaryAccount) int {
		key := func(item BoundaryAccount) ordering.Account {
			return ordering.Account{Identity: ordering.Identity{Platform: string(item.Platform), Name: item.IdentityName, ID: item.IdentityID}, Structure: item.accountStructure, Type: item.AccountType, ID: item.ID}
		}
		ka, kb := key(a), key(b)
		return ordering.CompareAccounts(&ka, &kb)
	})
	return out, nil
}
