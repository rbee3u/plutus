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

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/portfolio"
)

// scope is the authenticated user and the owned portfolio a request acts on.
type scope struct {
	user      auth.User
	session   auth.Session
	portfolio portfolio.Portfolio
}

func (sc *scope) portfolioContext() financial.PortfolioContext {
	return financial.PortfolioContext{ID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
}

// scoped authenticates the caller and loads one owned portfolio.
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
	if item.DeletionRequestedAt != nil {
		return scope{}, errNotFound
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
