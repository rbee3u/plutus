package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/auth"
	"github.com/rbee3u/plutus/internal/domain/portfolio"
	"github.com/rbee3u/plutus/internal/domain/valuation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

// scope is the authenticated user and the owned Portfolio a request acts on.
type scope struct {
	user      auth.User
	session   auth.Session
	portfolio portfolio.Portfolio
}

func (sc *scope) valuation() valuation.PortfolioContext {
	return valuation.PortfolioContext{ID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
}

// scoped authenticates the caller and loads one owned Portfolio.
func (a *API) scoped(ctx context.Context, portfolioID uuid.UUID) (scope, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return scope{}, errUnauthenticated
	}
	if a.Portfolios == nil {
		return scope{}, ErrNotImplemented
	}
	item, err := a.Portfolios.Get(ctx, user.ID, portfolioID)
	if errors.Is(err, portfolio.ErrNotFound) {
		return scope{}, errNotFound
	}
	if err != nil {
		return scope{}, err
	}
	return scope{user: user, session: session, portfolio: item}, nil
}

// mutating is scoped plus the CSRF check required by every state change.
func (a *API) mutating(ctx context.Context, portfolioID uuid.UUID, csrf *string) (scope, error) {
	sc, err := a.scoped(ctx, portfolioID)
	if err != nil {
		return scope{}, err
	}
	if err := a.requireCSRF(ctx, csrf, &sc.session); err != nil {
		return scope{}, err
	}
	return sc, nil
}

func requestTraceID(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	if value, ok := c.Get("trace_id"); ok {
		if trace, ok := value.(string); ok {
			return trace
		}
	}
	return ""
}

const rawBodyKey = "raw_body"

// captureBody keeps the raw PATCH body so handlers can tell an explicit null from an absent field.
func captureBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPatch && c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", "The request body could not be read.")
				c.Abort()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(data))
			c.Set(rawBodyKey, data)
		}
		c.Next()
	}
}

// explicitNulls returns the top-level JSON fields sent as null.
func explicitNulls(ctx context.Context) map[string]bool {
	c := ginFrom(ctx)
	if c == nil {
		return nil
	}
	raw, ok := c.Get(rawBodyKey)
	if !ok {
		return nil
	}
	data, _ := raw.([]byte)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil
	}
	out := make(map[string]bool, len(fields))
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			out[key] = true
		}
	}
	return out
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

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func uuidPtr(v uuid.UUID) *uuid.UUID {
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
