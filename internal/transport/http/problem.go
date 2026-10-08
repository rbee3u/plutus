package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/shared/apperr"
	"github.com/rbee3u/plutus/internal/transport/http/api"
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

func newProblemError(status int, code, title, detail string) error {
	return &problemError{status: status, code: code, title: title, detail: detail}
}

var (
	errUnauthenticated = newProblemError(http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
	errNotFound        = newProblemError(http.StatusNotFound, "not_found", "Not found", "The requested resource was not found.")
	errPrecondition    = newProblemError(http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")
)

func validationError(detail string) error {
	return newProblemError(http.StatusBadRequest, "validation_failed", "Request validation failed", detail)
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
		logHandlerError(c, err)
		writeProblem(c, http.StatusInternalServerError, "internal_error", "Internal server error", "The request failed due to an internal error.")
	}
}

// classify maps an application error kind to a status, code, and title.
func classify(err error) (int, string, string) {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return http.StatusNotFound, "not_found", "Not found"
	case apperr.KindPrecondition:
		return http.StatusPreconditionFailed, "precondition_failed", "Precondition failed"
	case apperr.KindUndeletable:
		return http.StatusConflict, "undeletable", "Portfolio is undeletable"
	case apperr.KindConflict:
		return http.StatusConflict, "conflict", "Conflict"
	case apperr.KindStateConflict:
		return http.StatusConflict, "state_conflict", "The identity state does not allow this action"
	case apperr.KindUnprocessable:
		return http.StatusUnprocessableEntity, "unprocessable", "The request could not be processed"
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable, "unavailable", "The platform connection service is unavailable"
	case apperr.KindRateLimited:
		return http.StatusTooManyRequests, "rate_limited", "Too many requests"
	case apperr.KindInvalid:
		return http.StatusBadRequest, "validation_failed", "Request validation failed"
	case apperr.KindInternal, apperr.KindUnauthenticated, apperr.KindForbidden:
		return 0, "", ""
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
