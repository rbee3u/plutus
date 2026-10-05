package identity

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/audit"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

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
		if t != adapters.TypeManual && !adapters.ValidAccountType(in.AccountStructure, t) {
			return Identity{}, ErrInvalid
		}
		selected[t] = true
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	if _, err := mysql.WithTx(ctx, s.db, func(tx *sql.Tx, q *sqlcgen.Queries) (Identity, error) {
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
		accountTypes := append(adapters.AccountTypes(in.AccountStructure), adapters.AccountTypeInfo{ID: adapters.TypeManual, Name: "Manual"})
		selected[adapters.TypeManual] = true
		for _, info := range accountTypes {
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
		return Identity{}, nil
	}); err != nil {
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
		if account, ok := known[patch.ID]; ok && account.AccountType == adapters.TypeManual && patch.Selected != nil && !*patch.Selected {
			return Identity{}, ErrInvalid
		}
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
			return Identity{}, fmt.Errorf("%w: %w Disable the Connection to change Accounts before replacing credentials", ErrInvalid, err)
		}
	}
	now := s.now().UTC()
	if _, err := mysql.WithTx(ctx, s.db, func(tx *sql.Tx, q *sqlcgen.Queries) (Identity, error) {
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
		return Identity{}, nil
	}); err != nil {
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
	if _, err := mysql.WithTx(ctx, s.db, func(tx *sql.Tx, q *sqlcgen.Queries) (Identity, error) {
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
		return Identity{}, nil
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}
