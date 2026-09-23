package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

// problemError carries an RFC 9457 problem from a handler to the response writer.
type problemError struct {
	status int
	code   string
	title  string
	detail string
}

func (e *problemError) Error() string {
	return e.code + ": " + e.detail
}

func problemf(status int, code, title, detail string) error {
	return &problemError{status: status, code: code, title: title, detail: detail}
}

var (
	errUnauthenticated = problemf(http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
	errNotFound        = problemf(http.StatusNotFound, "not_found", "Not found", "The requested resource was not found.")
	errPrecondition    = problemf(http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")
)

func validationError(detail string) error {
	return problemf(http.StatusBadRequest, "validation_failed", "Request validation failed", detail)
}

func writeProblem(c *gin.Context, status int, code, title, detail string) {
	body := newProblem(c, status, code, title, detail)
	c.Header("Content-Type", "application/problem+json")
	c.Status(status)
	if err := json.NewEncoder(c.Writer).Encode(body); err != nil {
		_ = c.Error(err)
	}
}

func newProblem(c *gin.Context, status int, code, title, detail string) api.Problem {
	body := api.Problem{
		Type:   "https://plutus.local/problems/" + code,
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
	}
	if c != nil {
		path := c.Request.URL.Path
		body.Instance = &path
		if traceID, ok := c.Get("trace_id"); ok {
			if trace, ok := traceID.(string); ok && trace != "" {
				body.TraceId = &trace
			}
		}
	}
	return body
}

func requestError(c *gin.Context, err error) {
	writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error())
}

// handlerError maps handler and service errors onto problem responses.
func handlerError(c *gin.Context, err error) {
	var pe *problemError
	if errors.As(err, &pe) {
		writeProblem(c, pe.status, pe.code, pe.title, pe.detail)
		return
	}
	if status, code, title := classify(err); status != 0 {
		writeProblem(c, status, code, title, err.Error())
		return
	}
	switch {
	case errors.Is(err, ErrNotImplemented):
		writeProblem(c, http.StatusNotImplemented, "not_implemented", "Not implemented", "This endpoint is not available in the current configuration.")
	case errors.Is(err, auth.ErrCSRF):
		writeProblem(c, http.StatusForbidden, "csrf_invalid", "CSRF token is invalid", "The request did not include a valid CSRF token.")
	case errors.Is(err, auth.ErrUnauthenticated):
		writeProblem(c, http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
	default:
		writeProblem(c, http.StatusInternalServerError, "internal_error", "Internal server error", "The request failed due to an internal error.")
	}
}

// classify maps well-known service errors to a status, code, and title.
func classify(err error) (int, string, string) {
	switch {
	case errors.Is(err, portfolio.ErrNotFound), errors.Is(err, identity.ErrNotFound), errors.Is(err, capital.ErrNotFound),
		errors.Is(err, catalog.ErrNotFound), errors.Is(err, jobs.ErrNotFound), errors.Is(err, valuation.ErrNotFound), errors.Is(err, valuation.ErrNoPoint):
		return http.StatusNotFound, "not_found", "Not found"
	case errors.Is(err, portfolio.ErrPrecondition), errors.Is(err, identity.ErrPrecondition), errors.Is(err, capital.ErrPrecondition), errors.Is(err, catalog.ErrPrecondition):
		return http.StatusPreconditionFailed, "precondition_failed", "Precondition failed"
	case errors.Is(err, portfolio.ErrUndeletable):
		return http.StatusConflict, "undeletable", "Portfolio is undeletable"
	case errors.Is(err, portfolio.ErrConflict), errors.Is(err, identity.ErrConflict), errors.Is(err, idempotency.ErrConflict):
		return http.StatusConflict, "conflict", "Conflict"
	case errors.Is(err, identity.ErrArchived), errors.Is(err, identity.ErrConnectionEnabled), errors.Is(err, identity.ErrCredentialsMissing):
		return http.StatusConflict, "state_conflict", "The identity state does not allow this action"
	case errors.Is(err, identity.ErrMasterKeyRequired), errors.Is(err, identity.ErrAdapterUnavailable):
		return http.StatusServiceUnavailable, "unavailable", "The platform connection service is unavailable"
	case errors.Is(err, portfolio.ErrInvalidName), errors.Is(err, portfolio.ErrInvalidTimezone), errors.Is(err, portfolio.ErrInvalidInterval),
		errors.Is(err, identity.ErrInvalid), errors.Is(err, capital.ErrInvalid), errors.Is(err, catalog.ErrInvalid), errors.Is(err, valuation.ErrInvalid),
		errors.Is(err, jobs.ErrInvalidCursor), errors.Is(err, jobs.ErrInvalidLimit), errors.Is(err, audit.ErrInvalidPage), errors.Is(err, audit.ErrInvalidCursor):
		return http.StatusBadRequest, "validation_failed", "Request validation failed"
	default:
		return 0, "", ""
	}
}

func (a *API) problem(ctx context.Context, status int, code, title, detail string) api.Problem {
	return newProblem(ginFrom(ctx), status, code, title, detail)
}

func (a *API) unauthenticated(ctx context.Context) api.Problem {
	return a.problem(ctx, http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
}
