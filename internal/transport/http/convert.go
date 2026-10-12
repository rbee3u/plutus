package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func enumPtr[T ~string](value *string) *T {
	if value == nil {
		return nil
	}
	return new(T(*value))
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func nonEmptyStringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func nonNilUUIDPtr(v uuid.UUID) *uuid.UUID {
	if v == uuid.Nil {
		return nil
	}
	return &v
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func toCursorPage(limit int, next *string, hasMore bool) api.CursorPage {
	return api.CursorPage{Kind: api.Cursor, Limit: limit, NextCursor: next, HasMore: hasMore}
}

func toOffsetPage(offset, limit, total int) api.OffsetPage {
	return api.OffsetPage{Kind: api.Offset, Offset: offset, Limit: limit, Total: total}
}
