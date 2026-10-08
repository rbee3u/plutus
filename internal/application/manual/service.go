// Package manual manages current signed balances and immutable cycle inputs.
package manual

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
	"github.com/rbee3u/plutus/internal/shared/etag"
)

// Balance is a user-entered quantity, never an editable exchange observation.
type Balance struct {
	Symbol      string    `json:"symbol"`
	ListingID   uuid.UUID `json:"listingId"`
	AssetSymbol string    `json:"assetSymbol"`
	Quantity    string    `json:"quantity"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}

// State separates the editable ledger from the last completed valuation.
type State struct {
	AccountID      uuid.UUID `json:"accountId"`
	IdentityID     uuid.UUID `json:"identityId"`
	Version        uint64    `json:"version"`
	AppliedVersion *uint64   `json:"appliedVersion"`
	Items          []Balance `json:"items"`
	AppliedItems   []Balance `json:"appliedItems"`
	Pending        bool      `json:"pending"`
}

// ETag guards the entire Identity's manual ledger, including new symbols.
func (s State) ETag() string { return etag.Format("manual", s.Version) }

// ParseETag accepts only the exact validator emitted by this service.
func ParseETag(value string) (uint64, bool) {
	return etag.Parse("manual", value)
}

// Input carries a balance mutation and its authenticated audit actor.
type Input struct {
	Symbol      string
	Quantity    string
	Version     uint64
	ActorUserID uuid.UUID
	RequestID   string
}

// Service owns manual ledger and revision persistence.
type Service struct {
	db      *sql.DB
	now     func() time.Time
	catalog *catalog.Service
}

// NewService constructs a manual ledger service.
func NewService(db *sql.DB, listings *catalog.Service) *Service {
	return &Service{db: db, now: time.Now, catalog: listings}
}

func balances(ctx context.Context, q *sqlcgen.Queries, portfolioID, accountID uuid.UUID) ([]Balance, error) {
	rows, err := q.ListManualBalances(ctx, sqlcgen.ListManualBalancesParams{PortfolioID: portfolioID[:], AccountID: accountID[:]})
	if err != nil {
		return nil, err
	}
	out := make([]Balance, 0, len(rows))
	for _, row := range rows {
		id, err := uuid.FromBytes(row.ListingID)
		if err != nil {
			return nil, err
		}
		out = append(out, Balance{Symbol: row.Symbol, ListingID: id, AssetSymbol: row.AssetSymbol, Quantity: decimal.Compact(row.Quantity), ModifiedAt: row.ModifiedAt})
	}
	return out, nil
}

// Quantities encodes only financial inputs, excluding edit timestamps and mapping.
func Quantities(items []Balance) (sqljson.Column, error) {
	amounts := map[string]string{}
	for _, item := range items {
		amounts[item.Symbol] = decimal.Compact(item.Quantity)
	}
	raw, err := json.Marshal(amounts)
	return sqljson.Column(raw), err
}

func sameQuantities(a, b []Balance) bool {
	left, leftErr := Quantities(a)
	right, rightErr := Quantities(b)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

// Get returns a consistent current ledger and the inputs of its last completed point.
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
		items, err := balances(ctx, q, portfolioID, accountID)
		if err != nil {
			return State{}, err
		}
		state := State{AccountID: accountID, IdentityID: identityID, Version: account.Version, Items: items, AppliedItems: []Balance{}}
		applied, err := q.GetAppliedManualState(ctx, sqlcgen.GetAppliedManualStateParams{PortfolioID: portfolioID[:], AccountID: account.ID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return State{}, err
		}
		if err == nil {
			state.AppliedVersion = &applied.Version
			if err := json.Unmarshal(applied.BalancesJson, &state.AppliedItems); err != nil {
				return State{}, err
			}
		}
		state.Pending = !sameQuantities(state.Items, state.AppliedItems)
		return state, nil
	})
}

// Create adds a previously absent symbol without accumulating another quantity.
func (s *Service) Create(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	return s.mutate(ctx, portfolioID, identityID, in, "create")
}

// Update sets a current quantity; zero clears the symbol.
func (s *Service) Update(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	return s.mutate(ctx, portfolioID, identityID, in, "update")
}

// Delete removes the current symbol while preserving all earlier revisions.
func (s *Service) Delete(ctx context.Context, portfolioID, identityID uuid.UUID, in Input) (State, error) {
	in.Quantity = "0"
	return s.mutate(ctx, portfolioID, identityID, in, "delete")
}

func (s *Service) mutate(ctx context.Context, portfolioID, identityID uuid.UUID, in Input, action string) (State, error) {
	symbol, err := catalog.NormalizeSymbol(in.Symbol)
	if err != nil {
		return State{}, err
	}
	quantity := strings.TrimSpace(in.Quantity)
	if !decimal.Valid(quantity) {
		return State{}, catalog.ErrInvalid
	}
	amount, ok := decimal.Parse(quantity)
	if !ok {
		return State{}, catalog.ErrInvalid
	}
	quantity = decimal.Format(amount)
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		account, err := q.LockManualAccount(ctx, sqlcgen.LockManualAccountParams{PortfolioID: portfolioID[:], IdentityID: identityID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return identity.ErrNotFound
		}
		if err != nil {
			return err
		}
		if account.Status == identity.StatusArchived {
			return identity.ErrArchived
		}
		if account.Version != in.Version {
			return identity.ErrPrecondition
		}
		accountID, err := uuid.FromBytes(account.ID)
		if err != nil {
			return err
		}
		before, err := balances(ctx, q, portfolioID, accountID)
		if err != nil {
			return err
		}
		var previous *Balance
		for i := range before {
			if before[i].Symbol == symbol {
				previous = &before[i]
				break
			}
		}
		if action == "create" && previous != nil {
			return identity.ErrConflict
		}
		if action != "create" && previous == nil {
			return identity.ErrNotFound
		}
		unchanged := (previous == nil && amount.Sign() == 0) || (previous != nil && previous.Quantity == quantity)
		if unchanged {
			return nil
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if amount.Sign() == 0 {
			err = q.DeleteManualBalance(ctx, sqlcgen.DeleteManualBalanceParams{PortfolioID: portfolioID[:], AccountID: account.ID, Symbol: symbol})
		} else {
			listingID, _, listingErr := s.catalog.EnsureListing(ctx, q, adapters.Platform(account.Platform), symbol, "", now)
			if listingErr != nil {
				return listingErr
			}
			err = q.PutManualBalance(ctx, sqlcgen.PutManualBalanceParams{PortfolioID: portfolioID[:], AccountID: account.ID, Symbol: symbol, ListingID: listingID[:], Quantity: quantity, ModifiedAt: now})
		}
		if err != nil {
			return err
		}
		if err := q.AdvanceManualVersion(ctx, sqlcgen.AdvanceManualVersionParams{PortfolioID: portfolioID[:], AccountID: account.ID, At: now}); err != nil {
			return err
		}
		after, err := balances(ctx, q, portfolioID, accountID)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(after)
		if err != nil {
			return err
		}
		if err := q.InsertManualRevision(ctx, sqlcgen.InsertManualRevisionParams{PortfolioID: portfolioID[:], AccountID: account.ID, Version: account.Version + 1, BalancesJson: sqljson.Column(raw), ModifiedAt: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, RequestID: in.RequestID,
			Action: "manual.balance." + action, ResourceType: "account", ResourceID: accountID,
			Before: before, After: after,
		})
	}); err != nil {
		return State{}, err
	}
	return s.Get(ctx, portfolioID, identityID)
}

// CycleAccount is an immutable input chosen once before a cycle contacts Platforms.
type CycleAccount struct {
	AccountID  uuid.UUID
	IdentityID uuid.UUID
	Platform   adapters.Platform
	Version    uint64
	Items      []Balance
	ModifiedAt sql.NullTime
}

// Freeze persists every enabled Identity's Manual inputs once, including empty ledgers.
func (s *Service) Freeze(ctx context.Context, portfolioID, jobID uuid.UUID) ([]CycleAccount, error) {
	return mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) ([]CycleAccount, error) {
		if _, err := q.LockManualCycleJob(ctx, sqlcgen.LockManualCycleJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]}); err != nil {
			return nil, err
		}
		_, err := q.GetManualCycle(ctx, sqlcgen.GetManualCycleParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if errors.Is(err, sql.ErrNoRows) {
			accounts, err := q.LockManualBoundary(ctx, portfolioID[:])
			if err != nil {
				return nil, err
			}
			if err := q.InsertManualCycle(ctx, sqlcgen.InsertManualCycleParams{PortfolioID: portfolioID[:], JobID: jobID[:], CapturedAt: s.now().UTC()}); err != nil {
				return nil, err
			}
			for _, account := range accounts {
				id, err := uuid.FromBytes(account.ID)
				if err != nil {
					return nil, err
				}
				items, err := balances(ctx, q, portfolioID, id)
				if err != nil {
					return nil, err
				}
				raw, err := json.Marshal(items)
				if err != nil {
					return nil, err
				}
				modified, err := q.GetManualModifiedAt(ctx, sqlcgen.GetManualModifiedAtParams{PortfolioID: portfolioID[:], AccountID: account.ID})
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
				at := sql.NullTime{Time: modified, Valid: err == nil}
				if err := q.InsertManualCycleAccount(ctx, sqlcgen.InsertManualCycleAccountParams{PortfolioID: portfolioID[:], JobID: jobID[:], AccountID: account.ID, Version: account.Version, BalancesJson: sqljson.Column(raw), ModifiedAt: at}); err != nil {
					return nil, err
				}
			}
		}
		rows, err := q.ListManualCycleAccounts(ctx, sqlcgen.ListManualCycleAccountsParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
		if err != nil {
			return nil, err
		}
		out := make([]CycleAccount, 0, len(rows))
		for _, row := range rows {
			accountID, err := uuid.FromBytes(row.AccountID)
			if err != nil {
				return nil, err
			}
			identityID, err := uuid.FromBytes(row.IdentityID)
			if err != nil {
				return nil, err
			}
			item := CycleAccount{AccountID: accountID, IdentityID: identityID, Platform: adapters.Platform(row.Platform), Version: row.Version, ModifiedAt: row.ModifiedAt}
			if err := json.Unmarshal(row.BalancesJson, &item.Items); err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	})
}
