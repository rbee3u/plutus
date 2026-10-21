// Package manual owns immutable revisions and the inputs each cycle reports.
package manual

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/shared/apperr"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Balance stores only revision facts; labels and pricing mappings are projections.
type Balance struct {
	AssetID              uuid.UUID `json:"assetId"`
	Quantity             string    `json:"quantity"`
	ModifiedAt           time.Time `json:"changedAt"`
	AssetSymbol          string    `json:"-"`
	CanonicalAssetSymbol string    `json:"-"`
}

type State struct {
	AccountID      uuid.UUID `json:"accountId"`
	IdentityID     uuid.UUID `json:"identityId"`
	Version        uint64    `json:"version"`
	AppliedVersion *uint64   `json:"appliedVersion"`
	Items          []Balance `json:"items"`
	AppliedItems   []Balance `json:"appliedItems"`
	Pending        bool      `json:"pending"`
}

type Input struct {
	AssetSymbol string
	Quantity    string
	Version     uint64
	ActorUserID uuid.UUID
	RequestID   string
}

type Service struct {
	db      *sql.DB
	now     func() time.Time
	catalog *catalog.Service
}

func NewService(db *sql.DB, assets *catalog.Service) *Service {
	return &Service{db: db, now: time.Now, catalog: assets}
}

// Quantities excludes edit times and mappings from the pending-change comparison.
func Quantities(items []Balance) (sqljson.Column, error) {
	amounts := map[string]string{}
	for _, item := range items {
		amounts[item.AssetID.String()] = decimal.Compact(item.Quantity)
	}
	raw, err := json.Marshal(amounts)
	return sqljson.Column(raw), err
}

func sameQuantities(a, b []Balance) bool {
	left, lerr := Quantities(a)
	right, rerr := Quantities(b)
	return lerr == nil && rerr == nil && string(left) == string(right)
}

// Hydrate reads immutable quantities and current portfolio-local asset labels.
func Hydrate(ctx context.Context, q *sqlcgen.Queries, portfolioID, accountID []byte, revision uint64) ([]Balance, error) {
	rows, err := q.ListManualRevisionBalances(ctx, sqlcgen.ListManualRevisionBalancesParams{PortfolioID: portfolioID, AccountID: accountID, RevisionNumber: revision})
	if err != nil {
		return nil, err
	}
	items := make([]Balance, 0, len(rows))
	for _, row := range rows {
		id, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return nil, err
		}
		items = append(items, Balance{AssetID: id, Quantity: decimal.Compact(row.Quantity), ModifiedAt: row.ChangedAt, AssetSymbol: row.AssetSymbol, CanonicalAssetSymbol: row.CanonicalAssetSymbol})
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, portfolioID, identityID uuid.UUID) (State, error) {
	return mysql.WithReadTx(ctx, s.db, func(q *sqlcgen.Queries) (State, error) {
		account, err := q.GetManualAccount(ctx, sqlcgen.GetManualAccountParams{PortfolioID: portfolioID[:], IdentityID: identityID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return State{}, identity.ErrNotFound
		}
		if err != nil {
			return State{}, err
		}
		accountID, err := uuid.FromBytes(account.ID)
		if err != nil {
			return State{}, err
		}
		revision, err := q.GetCurrentManualRevision(ctx, sqlcgen.GetCurrentManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID})
		if err != nil {
			return State{}, err
		}
		items, err := Hydrate(ctx, q, portfolioID[:], account.ID, revision.RevisionNumber)
		if err != nil {
			return State{}, err
		}
		state := State{AccountID: accountID, IdentityID: identityID, Version: revision.RevisionNumber, Items: items, AppliedItems: []Balance{}}
		applied, err := q.GetAppliedManualRevision(ctx, sqlcgen.GetAppliedManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return State{}, err
		}
		if err == nil && applied.Valid {
			version := uint64(applied.Int64) // #nosec G115 -- revision numbers are positive BIGINT UNSIGNED values far below 2^63.
			state.AppliedVersion = &version
			if version == revision.RevisionNumber {
				state.AppliedItems = append([]Balance{}, state.Items...)
			} else {
				state.AppliedItems, err = Hydrate(ctx, q, portfolioID[:], account.ID, version)
				if err != nil {
					return State{}, err
				}
			}
		}
		state.Pending = !sameQuantities(state.Items, state.AppliedItems)
		return state, nil
	})
}

func (s *Service) Create(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	return s.mutate(ctx, portfolioID, identityID, in, "create")
}

func (s *Service) Update(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	return s.mutate(ctx, portfolioID, identityID, in, "update")
}

func (s *Service) Delete(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	in.Quantity = "0"
	return s.mutate(ctx, portfolioID, identityID, in, "delete")
}

func (s *Service) mutate(ctx context.Context, portfolioID, identityID uuid.UUID, in Input, action string) (State, error) {
	symbol, err := catalog.NormalizeSymbol(in.AssetSymbol)
	if err != nil {
		return State{}, err
	}
	amount, ok := decimal.Parse(in.Quantity)
	if !ok {
		return State{}, apperr.Invalid("Enter a valid quantity.")
	}
	if !decimal.Valid(in.Quantity) {
		return State{}, apperr.Invalid("Use a plain decimal quantity with up to 35 integer digits and 30 decimal places.")
	}
	quantity := decimal.Compact(decimal.Format(amount))
	err = mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		account, err := q.LockManualAccount(ctx, sqlcgen.LockManualAccountParams{PortfolioID: portfolioID[:], IdentityID: identityID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return identity.ErrNotFound
		}
		if err != nil {
			return err
		}
		if account.ArchivedAt.Valid {
			return identity.ErrArchived
		}
		revision, err := q.GetCurrentManualRevision(ctx, sqlcgen.GetCurrentManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID})
		if err != nil {
			return err
		}
		if revision.RevisionNumber != in.Version {
			return identity.ErrPrecondition
		}
		items, err := Hydrate(ctx, q, portfolioID[:], account.ID, revision.RevisionNumber)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(items, func(item Balance) bool { return item.AssetSymbol == symbol })
		if action == "create" && index >= 0 {
			return identity.ErrConflict
		}
		if action != "create" && index < 0 {
			return identity.ErrNotFound
		}
		if (index < 0 && amount.Sign() == 0) || (index >= 0 && items[index].Quantity == quantity) {
			return nil
		}
		var before, after map[string]any
		if index >= 0 {
			before = map[string]any{"assetId": items[index].AssetID, "assetSymbol": symbol, "quantity": items[index].Quantity}
		}
		at := s.now().UTC().Truncate(time.Microsecond)
		if amount.Sign() == 0 {
			items = slices.Delete(items, index, index+1)
		} else if index >= 0 {
			items[index].Quantity, items[index].ModifiedAt = quantity, at
		} else {
			id, _, err := s.catalog.EnsureAsset(ctx, q, portfolioID, adapter.Platform(account.Platform), symbol, at)
			if err != nil {
				return err
			}
			items = append(items, Balance{AssetID: id, AssetSymbol: symbol, Quantity: quantity, ModifiedAt: at})
		}
		if amount.Sign() != 0 {
			item := items[slices.IndexFunc(items, func(item Balance) bool { return item.AssetSymbol == symbol })]
			after = map[string]any{"assetId": item.AssetID, "assetSymbol": symbol, "quantity": quantity}
		}
		slices.SortFunc(items, func(a, b Balance) int { return strings.Compare(a.AssetID.String(), b.AssetID.String()) })
		if err := q.InsertManualRevision(ctx, sqlcgen.InsertManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID, RevisionNumber: revision.RevisionNumber + 1, ChangedAt: sql.NullTime{Time: at, Valid: true}}); err != nil {
			return err
		}
		for _, item := range items {
			if err := q.InsertManualRevisionBalance(ctx, sqlcgen.InsertManualRevisionBalanceParams{PortfolioID: portfolioID[:], AccountID: account.ID, RevisionNumber: revision.RevisionNumber + 1, AssetID: item.AssetID[:], Quantity: item.Quantity, ChangedAt: item.ModifiedAt}); err != nil {
				return err
			}
		}
		accountID, err := uuid.FromBytes(account.ID)
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, at, &audit.Entry{PortfolioID: portfolioID, ActorUserID: in.ActorUserID, RequestID: in.RequestID, Action: "manual.balance." + action, ResourceType: "account", ResourceID: accountID, Before: map[string]any{"revisionNumber": revision.RevisionNumber, "balance": before}, After: map[string]any{"revisionNumber": revision.RevisionNumber + 1, "balance": after}})
	})
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx, portfolioID, identityID)
}

type CycleAccount struct {
	AccountID      uuid.UUID
	Platform       adapter.Platform
	RevisionNumber uint64
	Items          []Balance
}

// CycleInputs reads the current revision of every boundary manual account.
// Revisions are immutable, so the caller may price them before reporting.
func (s *Service) CycleInputs(ctx context.Context, portfolioID uuid.UUID) ([]CycleAccount, error) {
	return mysql.WithReadTx(ctx, s.db, func(q *sqlcgen.Queries) ([]CycleAccount, error) {
		accounts, err := q.ListManualBoundary(ctx, portfolioID[:])
		if err != nil {
			return nil, err
		}
		out := make([]CycleAccount, 0, len(accounts))
		for _, account := range accounts {
			accountID, err := uuid.FromBytes(account.ID)
			if err != nil {
				return nil, err
			}
			revision, err := q.GetCurrentManualRevision(ctx, sqlcgen.GetCurrentManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID})
			if err != nil {
				return nil, err
			}
			items, err := Hydrate(ctx, q, portfolioID[:], account.ID, revision.RevisionNumber)
			if err != nil {
				return nil, err
			}
			out = append(out, CycleAccount{AccountID: accountID, Platform: adapter.Platform(account.Platform), RevisionNumber: revision.RevisionNumber, Items: items})
		}
		return out, nil
	})
}
