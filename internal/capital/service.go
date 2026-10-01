// Package capital owns manual Capital Flows (ADR 0004).
package capital

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	// DirectionContribution moves money into the boundary.
	DirectionContribution = "contribution"
	// DirectionWithdrawal moves money out of the boundary.
	DirectionWithdrawal = "withdrawal"

	maxNote = 500
	// maxPriceAge bounds how far back a price may be to value a flow.
	maxPriceAge = 24 * time.Hour
)

// Service owns Capital Flow use cases.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a capital service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Flow is one manual Capital Flow.
type Flow struct {
	ID          uuid.UUID
	PortfolioID uuid.UUID
	Direction   string
	AssetSymbol string
	Quantity    string
	ValueUSD    *string
	EffectiveAt time.Time
	AccountID   *uuid.UUID
	AccountName *string
	Note        *string
	Version     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ETag returns the flow validator.
func (f *Flow) ETag() string {
	return fmt.Sprintf(`"capital-flow:%d"`, f.Version)
}

// ParseETag extracts the version from a flow ETag.
func ParseETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"capital-flow:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

// Input is the accepted create or patch payload.
type Input struct {
	ActorUserID uuid.UUID
	RequestID   string
	Direction   *string
	AssetSymbol *string
	Quantity    *string
	ValueUSD    *string
	EffectiveAt *time.Time
	AccountID   *uuid.UUID
	Note        *string
	// ClearAccount detaches the flow from its Account; ClearValue drops a manual USD value.
	ClearAccount bool
	ClearValue   bool
}

// List returns flows effective within [from, to), newest first.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, from, to time.Time) ([]Flow, error) {
	if from.IsZero() {
		from = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if to.IsZero() {
		to = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	rows, err := s.q.ListCapitalFlows(ctx, sqlcgen.ListCapitalFlowsParams{PortfolioID: portfolioID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	out := make([]Flow, 0, len(rows))
	for i := range rows {
		item, err := s.fromRow(ctx, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// Get returns one flow.
func (s *Service) Get(ctx context.Context, portfolioID, id uuid.UUID) (Flow, error) {
	row, err := s.q.GetCapitalFlow(ctx, sqlcgen.GetCapitalFlowParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Flow{}, ErrNotFound
	}
	if err != nil {
		return Flow{}, err
	}
	return s.fromRow(ctx, &row)
}

// Create inserts a flow, valuing it in USD from the nearest Platform price when no value is given.
func (s *Service) Create(ctx context.Context, portfolioID uuid.UUID, in *Input) (Flow, error) {
	if in.Direction == nil || in.AssetSymbol == nil || in.Quantity == nil || in.EffectiveAt == nil {
		return Flow{}, ErrInvalid
	}
	draft := Flow{PortfolioID: portfolioID}
	if err := s.apply(ctx, &draft, in); err != nil {
		return Flow{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Flow{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Flow{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	params := sqlcgen.InsertCapitalFlowParams{
		ID: id[:], PortfolioID: portfolioID[:], Direction: draft.Direction, AssetSymbol: draft.AssetSymbol, Quantity: draft.Quantity,
		EffectiveAt: draft.EffectiveAt, CreatedByUserID: in.ActorUserID[:], Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if draft.ValueUSD != nil {
		params.ValueUsd = sql.NullString{String: *draft.ValueUSD, Valid: true}
	}
	if draft.AccountID != nil {
		params.AccountID = draft.AccountID[:]
	}
	if draft.Note != nil {
		params.Note = sql.NullString{String: *draft.Note, Valid: true}
	}
	if err := q.InsertCapitalFlow(ctx, params); err != nil {
		return Flow{}, err
	}
	draft.ID, draft.Version = id, 1
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "capital_flow.create", ResourceType: "capital_flow", ResourceID: id,
		RequestID: in.RequestID, After: auditView(&draft),
	}); err != nil {
		return Flow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Flow{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Patch updates a flow when the version matches.
func (s *Service) Patch(ctx context.Context, portfolioID, id uuid.UUID, version uint64, in *Input) (Flow, error) {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return Flow{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	draft := current
	if err := s.apply(ctx, &draft, in); err != nil {
		return Flow{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Flow{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	params := sqlcgen.UpdateCapitalFlowParams{
		Direction: draft.Direction, AssetSymbol: draft.AssetSymbol, Quantity: draft.Quantity, EffectiveAt: draft.EffectiveAt,
		UpdatedAt: now, PortfolioID: portfolioID[:], ID: id[:], Version: version,
	}
	if draft.ValueUSD != nil {
		params.ValueUsd = sql.NullString{String: *draft.ValueUSD, Valid: true}
	}
	if draft.AccountID != nil {
		params.AccountID = draft.AccountID[:]
	}
	if draft.Note != nil {
		params.Note = sql.NullString{String: *draft.Note, Valid: true}
	}
	n, err := q.UpdateCapitalFlow(ctx, params)
	if err != nil {
		return Flow{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	draft.Version++
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "capital_flow.update", ResourceType: "capital_flow", ResourceID: id,
		RequestID: in.RequestID, Before: auditView(&current), After: auditView(&draft),
	}); err != nil {
		return Flow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Flow{}, err
	}
	return s.Get(ctx, portfolioID, id)
}

// Delete removes a flow when the version matches.
func (s *Service) Delete(ctx context.Context, portfolioID, id uuid.UUID, version uint64, actor uuid.UUID, requestID string) error {
	current, err := s.Get(ctx, portfolioID, id)
	if err != nil {
		return err
	}
	if current.Version != version {
		return ErrPrecondition
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.DeleteCapitalFlow(ctx, sqlcgen.DeleteCapitalFlowParams{PortfolioID: portfolioID[:], ID: id[:], Version: version})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPrecondition
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: portfolioID, ActorUserID: actor, Action: "capital_flow.delete", ResourceType: "capital_flow", ResourceID: id,
		RequestID: requestID, Before: auditView(&current),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// apply validates the input onto draft and (re)values it in USD when needed.
func (s *Service) apply(ctx context.Context, draft *Flow, in *Input) error {
	revalue := draft.ID == uuid.Nil
	if in.Direction != nil {
		if *in.Direction != DirectionContribution && *in.Direction != DirectionWithdrawal {
			return ErrInvalid
		}
		draft.Direction = *in.Direction
	}
	if in.AssetSymbol != nil {
		symbol, err := catalog.NormalizeSymbol(*in.AssetSymbol)
		if err != nil {
			return ErrInvalid
		}
		if symbol != draft.AssetSymbol {
			revalue = true
		}
		draft.AssetSymbol = symbol
	}
	if in.Quantity != nil {
		quantity, ok := decimal.Parse(*in.Quantity)
		if !ok || !decimal.Valid(*in.Quantity) || quantity.Sign() <= 0 {
			return ErrInvalid
		}
		if *in.Quantity != draft.Quantity {
			revalue = true
		}
		draft.Quantity = *in.Quantity
	}
	if in.EffectiveAt != nil {
		at := in.EffectiveAt.UTC().Truncate(time.Microsecond)
		if !at.Equal(draft.EffectiveAt) {
			revalue = true
		}
		draft.EffectiveAt = at
	}
	if in.ClearAccount {
		draft.AccountID = nil
	} else if in.AccountID != nil {
		if _, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: draft.PortfolioID[:], ID: in.AccountID[:]}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalid
			}
			return err
		}
		accountID := *in.AccountID
		draft.AccountID = &accountID
	}
	if in.Note != nil {
		note := strings.TrimSpace(*in.Note)
		if len(note) > maxNote {
			return ErrInvalid
		}
		if note == "" {
			draft.Note = nil
		} else {
			draft.Note = &note
		}
	}
	switch {
	case in.ClearValue:
		revalue = true
	case in.ValueUSD != nil:
		value, ok := decimal.Parse(*in.ValueUSD)
		if !ok || !decimal.Valid(*in.ValueUSD) || value.Sign() < 0 {
			return ErrInvalid
		}
		draft.ValueUSD = in.ValueUSD
		revalue = false
	}
	if revalue {
		draft.ValueUSD = s.valueInUSD(ctx, draft.AssetSymbol, draft.Quantity, draft.EffectiveAt)
	}
	return nil
}

// valueInUSD converts a quantity at the nearest Platform price at or before
// effectiveAt (ADR 0003).
func (s *Service) valueInUSD(ctx context.Context, assetSymbol, quantity string, at time.Time) *string {
	qty, ok := decimal.Parse(quantity)
	if !ok {
		return nil
	}
	switch assetSymbol {
	case "USD", "USDT", "USDC":
		return decimal.FormatPtr(qty)
	}
	row, err := s.q.GetNearestPriceAt(ctx, sqlcgen.GetNearestPriceAtParams{AssetSymbol: assetSymbol, FilterPlatform: 0, At: at.UTC()})
	if err != nil || at.Sub(row.ObservedAt) > maxPriceAge {
		return nil
	}
	switch row.QuoteSymbol {
	case "USD", "USDT", "USDC":
	default:
		return nil
	}
	price, ok := decimal.Parse(row.Price)
	if !ok {
		return nil
	}
	return decimal.FormatPtr(decimal.Mul(qty, price))
}

func (s *Service) fromRow(ctx context.Context, row *sqlcgen.CapitalFlow) (Flow, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Flow{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Flow{}, err
	}
	item := Flow{
		ID: id, PortfolioID: portfolioID, Direction: row.Direction, AssetSymbol: row.AssetSymbol, Quantity: decimal.Compact(row.Quantity),
		EffectiveAt: row.EffectiveAt, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.ValueUsd.Valid {
		value := decimal.Compact(row.ValueUsd.String)
		item.ValueUSD = &value
	}
	if row.Note.Valid {
		note := row.Note.String
		item.Note = &note
	}
	if len(row.AccountID) == 16 {
		accountID, err := uuid.FromBytes(row.AccountID)
		if err != nil {
			return Flow{}, err
		}
		item.AccountID = &accountID
		if account, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: row.PortfolioID, ID: row.AccountID}); err == nil {
			name := account.IdentityName + " / " + account.DisplayName
			item.AccountName = &name
		}
	}
	return item, nil
}

type flowAudit struct {
	Direction   string  `json:"direction"`
	AssetSymbol string  `json:"assetSymbol"`
	Quantity    string  `json:"quantity"`
	ValueUSD    *string `json:"valueUsd"`
	EffectiveAt string  `json:"effectiveAt"`
	AccountID   *string `json:"accountId,omitempty"`
	Note        *string `json:"note,omitempty"`
	Version     uint64  `json:"version"`
}

func auditView(f *Flow) flowAudit {
	view := flowAudit{Direction: f.Direction, AssetSymbol: f.AssetSymbol, Quantity: f.Quantity, ValueUSD: f.ValueUSD, EffectiveAt: f.EffectiveAt.UTC().Format(time.RFC3339Nano), Note: f.Note, Version: f.Version}
	if f.AccountID != nil {
		id := f.AccountID.String()
		view.AccountID = &id
	}
	return view
}
