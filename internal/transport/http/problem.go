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
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/projection"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

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

func (a *API) problem(ctx context.Context, status int, code, title, detail string) api.Problem {
	return newProblem(ginFrom(ctx), status, code, title, detail)
}

func (a *API) unauthenticated(ctx context.Context) api.Problem {
	return a.problem(ctx, http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
}

func requestError(c *gin.Context, err error) {
	writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error())
}

func handlerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotImplemented):
		writeProblem(c, http.StatusNotImplemented, "not_implemented", "Not implemented", "This endpoint is not implemented in the current phase.")
	case errors.Is(err, auth.ErrCSRF):
		writeProblem(c, http.StatusForbidden, "csrf_invalid", "CSRF token is invalid", "The request did not include a valid CSRF token.")
	case errors.Is(err, auth.ErrUnauthenticated):
		writeProblem(c, http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "A valid session is required.")
	case errors.Is(err, jobs.ErrInvalidCursor), errors.Is(err, jobs.ErrInvalidLimit):
		writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error())
	case errors.Is(err, catalog.ErrInvalidPage), errors.Is(err, catalog.ErrInvalid), errors.Is(err, catalog.ErrInvalidSubject), errors.Is(err, observation.ErrInvalidPage), errors.Is(err, observation.ErrInvalid), errors.Is(err, observation.ErrInvalidCursor), errors.Is(err, audit.ErrInvalidPage), errors.Is(err, audit.ErrInvalidCursor), errors.Is(err, capital.ErrInvalidPage), errors.Is(err, capital.ErrInvalidCursor), errors.Is(err, projection.ErrInvalidPage), errors.Is(err, projection.ErrInvalidGroup), errors.Is(err, projection.ErrInvalidCursor), errors.Is(err, projection.ErrInvalidQuery):
		writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error())
	default:
		writeProblem(c, http.StatusInternalServerError, "internal_error", "Internal server error", "The request failed due to an internal error.")
	}
}
