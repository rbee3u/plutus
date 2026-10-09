package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// Service reads Portfolio audit events.
type Service struct {
	q *sqlcgen.Queries
}

// NewService constructs an audit listing service.
func NewService(db *sql.DB) *Service {
	return &Service{q: sqlcgen.New(db)}
}

// Event is a stored audit document.
type Event struct {
	ID           uuid.UUID
	PortfolioID  *uuid.UUID
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   uuid.UUID
	RequestID    string
	Before       *map[string]any
	After        *map[string]any
	Metadata     *map[string]any
	OccurredAt   time.Time
}

// Page is a cursor page of audit events.
type Page struct {
	Items      []Event
	Limit      int
	NextCursor *string
	HasMore    bool
}

// ListFilter selects audit events.
type ListFilter struct {
	Action       string
	ResourceType string
	Limit        int
	Cursor       string
}

// List returns audit events for a Portfolio using authenticated cursor pagination.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, in *ListFilter) (Page, error) {
	if in == nil {
		in = &ListFilter{}
	}
	limit, err := normalizeLimit(in.Limit)
	if err != nil {
		return Page{}, err
	}
	params := sqlcgen.ListAuditEventsByPortfolioParams{
		PortfolioID:        portfolioID[:],
		FilterAction:       flagInt(in.Action != ""),
		Action:             in.Action,
		LegacyAction:       legacyAction(in.Action),
		FilterResourceType: flagInt(in.ResourceType != ""),
		ResourceType:       in.ResourceType,
		LegacyResourceType: legacyResourceType(in.ResourceType),
		HasCursor:          0,
		CursorOccurredAt:   time.Time{},
		CursorID:           make([]byte, 16),
		Limit:              int32(limit + 1), // #nosec G115 -- normalizeLimit bounds the page size before adding the lookahead row.
	}
	if in.Cursor != "" {
		decoded, err := decodeCursor(in.Cursor, portfolioID)
		if err != nil {
			return Page{}, err
		}
		params.HasCursor = 1
		params.CursorOccurredAt = time.UnixMicro(decoded.TimeMicro).UTC()
		params.CursorID = decoded.ID[:]
	}
	rows, err := s.q.ListAuditEventsByPortfolio(ctx, params)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]Event, 0, len(rows))
	for i := range rows {
		item, err := eventFromRow(&rows[i])
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	page := Page{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := cursorCodec.Encode(cursor.Cursor{
			PortfolioID: portfolioID,
			TimeMicro:   last.OccurredAt.UTC().UnixMicro(),
			ID:          last.ID,
		})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

func eventFromRow(row *sqlcgen.AuditEvent) (Event, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Event{}, err
	}
	resourceID, err := uuid.FromBytes(row.ResourceID)
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID:           id,
		PortfolioID:  optionalUUID(row.PortfolioID),
		ActorUserID:  optionalUUID(row.ActorUserID),
		Action:       row.Action,
		ResourceType: row.ResourceType,
		ResourceID:   resourceID,
		RequestID:    row.RequestID,
		Before:       asObject(row.BeforeJson),
		After:        asObject(row.AfterJson),
		Metadata:     asObject(row.MetadataJson),
		OccurredAt:   row.OccurredAt,
	}, nil
}

func asObject(raw []byte) *map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}

func optionalUUID(raw []byte) *uuid.UUID {
	if len(raw) != 16 {
		return nil
	}
	id, err := uuid.FromBytes(raw)
	if err != nil {
		return nil
	}
	return &id
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

// Historical audit documents retain their original vocabulary.
func legacyAction(action string) string {
	switch action {
	case "catalog.platform_asset.map":
		return "catalog.listing.map"
	case "catalog.platform_asset.exclude":
		return "catalog.listing.exclude"
	case "catalog.platform_asset.restore":
		return "catalog.listing.restore"
	default:
		return action
	}
}

func legacyResourceType(resource string) string {
	if resource == "platform_asset" {
		return "asset_listing"
	}
	return resource
}
