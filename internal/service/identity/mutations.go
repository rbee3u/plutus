package identity

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rbee3u/plutus/internal/shared/textkey"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
)

// Create inserts an identity and one account per account type of its account structure.
func (s *Service) Create(ctx context.Context, portfolioID uuid.UUID, in *CreateInput) (Identity, error) {
	name, err := normalizeName(in.DisplayName)
	if err != nil {
		return Identity{}, err
	}
	if !adapter.ValidPlatform(in.Platform) || !adapter.ValidStructure(in.Platform, in.AccountStructure) {
		return Identity{}, ErrInvalid
	}
	selected := map[adapter.AccountType]bool{}
	for _, t := range in.SelectedAccountTypes {
		if t != adapter.TypeManual && !adapter.ValidAccountType(in.AccountStructure, t) {
			return Identity{}, ErrInvalid
		}
		selected[t] = true
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		if err := q.InsertIdentity(ctx, sqlcgen.InsertIdentityParams{
			ID:               id[:],
			PortfolioID:      portfolioID[:],
			Platform:         string(in.Platform),
			DisplayName:      name,
			DisplayNameKey:   textkey.Key(name),
			AccountStructure: string(in.AccountStructure),
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			if mysql.IsDuplicate(err) {
				return ErrConflict
			}
			return err
		}
		accountTypes := append(adapter.AccountTypes(in.AccountStructure), adapter.AccountTypeInfo{ID: adapter.TypeManual, Name: "Manual"})
		selected[adapter.TypeManual] = true
		for _, info := range accountTypes {
			accountID, err := uuid.NewV7()
			if err != nil {
				return err
			}
			if err := q.InsertAccount(ctx, sqlcgen.InsertAccountParams{
				ID:          accountID[:],
				PortfolioID: portfolioID[:],
				IdentityID:  id[:],
				AccountType: string(info.ID),
				Selected:    selected[info.ID],
			}); err != nil {
				return err
			}
			if info.ID == adapter.TypeManual {
				if err := q.InsertManualRevision(ctx, sqlcgen.InsertManualRevisionParams{PortfolioID: portfolioID[:], AccountID: accountID[:], RevisionNumber: 1}); err != nil {
					return err
				}
			}
		}
		created := Identity{ID: id, PortfolioID: portfolioID, Platform: in.Platform, DisplayName: name, AccountStructure: in.AccountStructure, Status: StatusEnabled, Version: 1}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.create", ResourceType: "identity", ResourceID: id,
			RequestID: in.RequestID, After: auditView(&created), Metadata: map[string]any{"selectedAccountTypes": in.SelectedAccountTypes},
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Patch renames an identity and updates account selection when the version matches.
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
		if account, ok := known[patch.ID]; ok && account.AccountType == adapter.TypeManual && patch.Selected != nil && !*patch.Selected {
			return Identity{}, ErrInvalid
		}
		if _, ok := known[patch.ID]; !ok {
			return Identity{}, ErrNotFound
		}
	}
	if current.Platform == adapter.PlatformKraken && current.Connection.Status == ConnectionEnabled {
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
		if err := adapter.CheckCredentials(current.Platform, current.AccountStructure, selectedTypes(accounts), creds); err != nil {
			return Identity{}, fmt.Errorf("%w: %w Disable the connection to change accounts before replacing credentials", ErrInvalid, err)
		}
	}
	now := s.now().UTC()
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		n, err := q.UpdateIdentity(ctx, sqlcgen.UpdateIdentityParams{
			DisplayName: name, DisplayNameKey: textkey.Key(name), UpdatedAt: now,
			PortfolioID: portfolioID[:], ID: id[:], Version: version,
		})
		if err != nil {
			if mysql.IsDuplicate(err) {
				return ErrConflict
			}
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		for _, patch := range in.Accounts {
			account := known[patch.ID]
			if patch.Selected != nil {
				account.Selected = *patch.Selected
			}
			if _, err := q.UpdateAccount(ctx, sqlcgen.UpdateAccountParams{
				Selected:    account.Selected,
				PortfolioID: portfolioID[:], ID: account.ID[:],
			}); err != nil {
				return err
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
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.update", ResourceType: "identity", ResourceID: id,
			RequestID: in.RequestID, Before: auditView(&current), After: auditView(&updated),
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Archive removes an identity from the boundary. It is refused while the connection is enabled.
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
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		n, err := q.ArchiveIdentity(ctx, sqlcgen.ArchiveIdentityParams{
			ArchivedAt: sql.NullTime{Time: now, Valid: true}, UpdatedAt: now,
			PortfolioID: portfolioID[:], ID: id[:], Version: version,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		updated := current
		updated.Status = StatusArchived
		updated.ArchivedAt = &now
		updated.Version++
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "identity.archive", ResourceType: "identity", ResourceID: id,
			RequestID: in.RequestID, Before: auditView(&current), After: auditView(&updated),
		})
	}); err != nil {
		return Identity{}, err
	}
	return s.Get(ctx, portfolioID, id)
}
