package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	maxDisplayNameLength = 191
	maxReasonLength      = 1024
	subjectListing       = "listing"
	subjectInstrument    = "instrument"
	stateProvisional     = "provisional"
	stateConfirmed       = "confirmed"
	stateUnmapped        = "unmapped"
)

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
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

func nullNote(note *string) sql.NullString {
	if note == nil {
		return sql.NullString{}
	}
	value := strings.TrimSpace(*note)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func ptrNote(note sql.NullString) *string {
	if !note.Valid {
		return nil
	}
	value := note.String
	return &value
}

func flagInt(on bool) int {
	if on {
		return 1
	}
	return 0
}

func validateReason(note *string) error {
	if note == nil {
		return nil
	}
	if len(strings.TrimSpace(*note)) > maxReasonLength {
		return ErrInvalid
	}
	return nil
}

func validateDisplayName(name *string) error {
	if name == nil {
		return nil
	}
	if len(strings.TrimSpace(*name)) > maxDisplayNameLength {
		return ErrInvalid
	}
	return nil
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func currentOrNil[T any](exists bool, current *T) any {
	if !exists {
		return nil
	}
	return current
}

func isDuplicate(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func parseRevisionETag(value, layout string) (uint64, bool, bool) {
	if value == "*" {
		return 0, true, true
	}
	var version uint64
	if _, err := fmt.Sscanf(value, layout, &version); err != nil {
		return 0, false, false
	}
	return version, false, true
}

func insertAudit(ctx context.Context, q *sqlcgen.Queries, portfolioID, actor uuid.UUID, requestID, action, resourceType string, resourceID uuid.UUID, before, after any, now time.Time) error {
	auditID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var beforeJSON, afterJSON json.RawMessage
	if before != nil {
		beforeJSON, err = json.Marshal(before)
		if err != nil {
			return err
		}
	}
	if after != nil {
		afterJSON, err = json.Marshal(after)
		if err != nil {
			return err
		}
	}
	return q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:           mustBin(auditID),
		PortfolioID:  mustBin(portfolioID),
		ActorUserID:  mustBin(actor),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   mustBin(resourceID),
		RequestID:    defaultString(requestID, "unknown"),
		BeforeJson:   beforeJSON,
		AfterJson:    afterJSON,
		OccurredAt:   now,
	})
}
