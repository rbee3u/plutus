package observation

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// SummaryPage is a cursor page of observation headers.
type SummaryPage struct {
	Items      []Observation
	Limit      int
	NextCursor *string
	HasMore    bool
}

// ListFilter selects observation headers.
type ListFilter struct {
	Kind       string
	IdentityID *uuid.UUID
	AccountID  *uuid.UUID
	From       *time.Time
	To         *time.Time
	Limit      int
	Cursor     string
}

// List returns observation headers using authenticated cursor pagination.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, in *ListFilter) (SummaryPage, error) {
	if in == nil {
		in = &ListFilter{}
	}
	if in.Kind != "" && !validObservationKind(in.Kind) {
		return SummaryPage{}, ErrInvalid
	}
	if in.From != nil && in.To != nil && in.From.After(*in.To) {
		return SummaryPage{}, ErrInvalid
	}
	limit, err := normalizeLimit(in.Limit)
	if err != nil {
		return SummaryPage{}, err
	}
	params := sqlcgen.ListObservationsParams{
		PortfolioID:      mustBin(portfolioID),
		FilterKind:       flagInt(in.Kind != ""),
		Kind:             in.Kind,
		FilterIdentity:   flagInt(in.IdentityID != nil),
		IdentityID:       make([]byte, 16),
		FilterAccount:    flagInt(in.AccountID != nil),
		AccountID:        make([]byte, 16),
		FilterFrom:       flagInt(in.From != nil),
		FilterTo:         flagInt(in.To != nil),
		HasCursor:        0,
		CursorObservedAt: time.Time{},
		CursorID:         make([]byte, 16),
		Limit:            int32(limit + 1),
	}
	if in.IdentityID != nil {
		params.IdentityID = mustBin(*in.IdentityID)
	}
	if in.AccountID != nil {
		params.AccountID = mustBin(*in.AccountID)
	}
	if in.From != nil {
		params.FromAt = in.From.UTC()
	}
	if in.To != nil {
		params.ToAt = in.To.UTC()
	}
	if in.Cursor != "" {
		decoded, err := decodeCursor(in.Cursor, portfolioID)
		if err != nil {
			return SummaryPage{}, err
		}
		params.HasCursor = 1
		params.CursorObservedAt = time.UnixMicro(decoded.ObservedAt).UTC()
		params.CursorID = mustBin(decoded.ID)
	}
	rows, err := s.q.ListObservations(ctx, params)
	if err != nil {
		return SummaryPage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]Observation, 0, len(rows))
	for i := range rows {
		item, err := observationFromHeader(&rows[i])
		if err != nil {
			return SummaryPage{}, err
		}
		items = append(items, item)
	}
	page := SummaryPage{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{
			PortfolioID: portfolioID,
			ObservedAt:  last.SourceObservedAt.UTC().UnixMicro(),
			ID:          last.ID,
		})
		if err != nil {
			return SummaryPage{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

// Summary returns a short header description.
func (o *Observation) Summary() string {
	switch o.Kind {
	case "balance":
		return "Balance"
	case "position":
		return "Position"
	case "reported_equity":
		return "Reported equity"
	case "price":
		return "Price"
	case "account_metric":
		return "Account metric"
	default:
		return o.Kind
	}
}

func validObservationKind(kind string) bool {
	switch kind {
	case "balance", "position", "reported_equity", "price", "account_metric":
		return true
	default:
		return false
	}
}

func normalizeLimit(limit int) (int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return 0, ErrInvalidPage
	}
	return limit, nil
}

func flagInt(on bool) int {
	if on {
		return 1
	}
	return 0
}
