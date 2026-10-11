package audit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// Service reads Portfolio audit events.
type Service struct {
	db *sql.DB
}

// NewService constructs an audit listing service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Event is a stored audit document.
type Event struct {
	ID           uuid.UUID
	PortfolioID  uuid.UUID
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
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (Page, error) {
		return s.list(ctx, portfolioID, in)
	})
}

func (s *Service) list(ctx context.Context, portfolioID uuid.UUID, in *ListFilter) (Page, error) {
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
		FilterResourceType: flagInt(in.ResourceType != ""),
		ResourceType:       in.ResourceType,
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
	rows, err := mysql.Queries(ctx, s.db).ListAuditEventsByPortfolio(ctx, params)
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
	if err := s.exclusionStates(ctx, portfolioID, items); err != nil {
		return Page{}, err
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

// Permanent rule versions remain the source of exclusion facts after later edits.
func (s *Service) exclusionStates(ctx context.Context, portfolioID uuid.UUID, items []Event) error {
	ids := make([][]byte, 0)
	for _, item := range items {
		if item.ResourceType == "asset" && (item.Action == "catalog.asset.exclude" || item.Action == "catalog.asset.restore") {
			ids = append(ids, item.ID[:])
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := mysql.Queries(ctx, s.db).ListAuditExclusionVersions(ctx, sqlcgen.ListAuditExclusionVersionsParams{PortfolioID: portfolioID[:], EventIds: ids})
	if err != nil {
		return err
	}
	type key struct {
		event   uuid.UUID
		version uint64
	}
	versions := make(map[key]sqlcgen.ListAuditExclusionVersionsRow, len(rows))
	for _, row := range rows {
		id, err := uuid.FromBytes(row.EventID)
		if err != nil {
			return err
		}
		versions[key{id, row.Version}] = row
	}
	for _, item := range items {
		if item.ResourceType != "asset" || (item.Action != "catalog.asset.exclude" && item.Action != "catalog.asset.restore") {
			continue
		}
		for _, state := range []*map[string]any{item.Before, item.After} {
			if state == nil {
				continue
			}
			number, ok := (*state)["version"].(json.Number)
			if !ok {
				continue
			}
			version, err := strconv.ParseUint(string(number), 10, 64)
			if err != nil {
				return fmt.Errorf("audit exclusion version: %w", err)
			}
			if version == 0 {
				(*state)["excluded"], (*state)["reason"], (*state)["effectiveAt"] = false, nil, nil
				continue
			}
			row, ok := versions[key{item.ID, version}]
			if !ok {
				return fmt.Errorf("audit exclusion version %d is missing", version)
			}
			(*state)["excluded"], (*state)["effectiveAt"] = row.Excluded, row.EffectiveAt.UTC()
			(*state)["reason"] = nil
			if row.Reason.Valid {
				(*state)["reason"] = row.Reason.String
			}
		}
	}
	return nil
}

func eventFromRow(row *sqlcgen.AuditEvent) (Event, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Event{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Event{}, err
	}
	resourceID, err := uuid.FromBytes(row.ResourceID)
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID:           id,
		PortfolioID:  portfolioID,
		ActorUserID:  optionalUUID(row.ActorUserID),
		Action:       row.Action,
		ResourceType: row.ResourceType,
		ResourceID:   resourceID,
		RequestID:    row.RequestID.String,
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
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
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
