package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
)

// Entry is one audit document to append.
type Entry struct {
	PortfolioID  uuid.UUID
	ActorUserID  uuid.UUID
	Action       string
	ResourceType string
	ResourceID   uuid.UUID
	RequestID    string
	Before       any
	After        any
	Metadata     any
}

// Record appends one audit event through q, which may be transactional.
// Entries without an actor (system and CLI paths) are skipped.
func Record(ctx context.Context, q *sqlcgen.Queries, now time.Time, entry *Entry) error {
	if entry.ActorUserID == uuid.Nil {
		return nil
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	before, err := marshalOptional(entry.Before)
	if err != nil {
		return err
	}
	after, err := marshalOptional(entry.After)
	if err != nil {
		return err
	}
	metadata, err := marshalOptional(entry.Metadata)
	if err != nil {
		return err
	}
	params := sqlcgen.InsertAuditEventParams{
		ID:           id[:],
		ActorUserID:  entry.ActorUserID[:],
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID[:],
		RequestID:    entry.RequestID,
		BeforeJson:   before,
		AfterJson:    after,
		MetadataJson: metadata,
		OccurredAt:   now.UTC(),
	}
	if entry.PortfolioID != uuid.Nil {
		params.PortfolioID = entry.PortfolioID[:]
	}
	return q.InsertAuditEvent(ctx, params)
}

func marshalOptional(value any) (sqljson.Column, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return sqljson.Column(raw), nil
}
