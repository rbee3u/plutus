// Package capital owns manual Capital Flows (ADR 0004).
package capital

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
	"github.com/rbee3u/plutus/internal/shared/etag"
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
	ID                   uuid.UUID
	PortfolioID          uuid.UUID
	Direction            string
	CanonicalAssetSymbol string
	Quantity             string
	ValueUSD             *string
	EffectiveAt          time.Time
	AccountID            *uuid.UUID
	AccountName          *string
	Note                 *string
	Version              uint64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ETag returns the flow validator.
func (f *Flow) ETag() string {
	return etag.Format("capital-flow", f.Version)
}

// ParseETag extracts the version from a flow ETag.
func ParseETag(value string) (uint64, bool) {
	return etag.Parse("capital-flow", value)
}

// Input is the accepted create or patch payload.
type Input struct {
	ActorUserID          uuid.UUID
	RequestID            string
	Direction            *string
	CanonicalAssetSymbol *string
	Quantity             *string
	ValueUSD             *string
	EffectiveAt          *time.Time
	AccountID            *uuid.UUID
	Note                 *string
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
	if in.Direction == nil || in.CanonicalAssetSymbol == nil || in.Quantity == nil || in.EffectiveAt == nil {
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
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		params := sqlcgen.InsertCapitalFlowParams{
			ID: id[:], PortfolioID: portfolioID[:], Direction: draft.Direction, CanonicalAssetSymbol: draft.CanonicalAssetSymbol, Quantity: draft.Quantity,
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
			return err
		}
		draft.ID, draft.Version = id, 1
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "capital_flow.create", ResourceType: "capital_flow", ResourceID: id,
			RequestID: in.RequestID, After: auditView(&draft),
		})
	}); err != nil {
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
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		params := sqlcgen.UpdateCapitalFlowParams{
			Direction: draft.Direction, CanonicalAssetSymbol: draft.CanonicalAssetSymbol, Quantity: draft.Quantity, EffectiveAt: draft.EffectiveAt,
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
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		draft.Version++
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: in.ActorUserID, Action: "capital_flow.update", ResourceType: "capital_flow", ResourceID: id,
			RequestID: in.RequestID, Before: auditView(&current), After: auditView(&draft),
		})
	}); err != nil {
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
	return mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		n, err := q.DeleteCapitalFlow(ctx, sqlcgen.DeleteCapitalFlowParams{PortfolioID: portfolioID[:], ID: id[:], Version: version})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPrecondition
		}
		return audit.Record(ctx, q, now, &audit.Entry{
			PortfolioID: portfolioID, ActorUserID: actor, Action: "capital_flow.delete", ResourceType: "capital_flow", ResourceID: id,
			RequestID: requestID, Before: auditView(&current),
		})
	})
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
	if in.CanonicalAssetSymbol != nil {
		canonicalAssetSymbol, err := catalog.NormalizeSymbol(*in.CanonicalAssetSymbol)
		if err != nil {
			return ErrInvalid
		}
		if canonicalAssetSymbol != draft.CanonicalAssetSymbol {
			revalue = true
		}
		draft.CanonicalAssetSymbol = canonicalAssetSymbol
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
		draft.ValueUSD = s.valueInUSD(ctx, draft.CanonicalAssetSymbol, draft.Quantity, draft.EffectiveAt)
	}
	return nil
}

// Cross-Platform lookup is an unresolved deviation from ADR 0003; see
// docs/architecture.md#known-implementation-discrepancy.
func (s *Service) valueInUSD(ctx context.Context, canonicalAssetSymbol, quantity string, at time.Time) *string {
	qty, ok := decimal.Parse(quantity)
	if !ok {
		return nil
	}
	switch canonicalAssetSymbol {
	case "USD", "USDT", "USDC":
		return decimal.FormatPtr(qty)
	}
	row, err := s.q.GetNearestPriceAt(ctx, sqlcgen.GetNearestPriceAtParams{CanonicalAssetSymbol: canonicalAssetSymbol, FilterPlatform: 0, At: at.UTC()})
	if err != nil || at.Sub(row.ObservedAt) > maxPriceAge {
		return nil
	}
	switch row.QuoteCanonicalAssetSymbol {
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
		ID: id, PortfolioID: portfolioID, Direction: row.Direction, CanonicalAssetSymbol: row.CanonicalAssetSymbol, Quantity: decimal.Compact(row.Quantity),
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
	Direction            string  `json:"direction"`
	CanonicalAssetSymbol string  `json:"canonicalAssetSymbol"`
	Quantity             string  `json:"quantity"`
	ValueUSD             *string `json:"valueUsd"`
	EffectiveAt          string  `json:"effectiveAt"`
	AccountID            *string `json:"accountId,omitempty"`
	Note                 *string `json:"note,omitempty"`
	Version              uint64  `json:"version"`
}

func auditView(f *Flow) flowAudit {
	view := flowAudit{Direction: f.Direction, CanonicalAssetSymbol: f.CanonicalAssetSymbol, Quantity: f.Quantity, ValueUSD: f.ValueUSD, EffectiveAt: f.EffectiveAt.UTC().Format(time.RFC3339Nano), Note: f.Note, Version: f.Version}
	if f.AccountID != nil {
		id := f.AccountID.String()
		view.AccountID = &id
	}
	return view
}
