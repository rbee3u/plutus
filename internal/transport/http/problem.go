package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func writeProblem(c *gin.Context, status int, code, title, detail string) {
	traceID, _ := c.Get("trace_id")
	trace, _ := traceID.(string)
	body := api.Problem{
		Type:     "https://plutus.local/problems/" + code,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Code:     code,
		Instance: &c.Request.URL.Path,
	}
	if trace != "" {
		body.TraceId = &trace
	}
	c.Header("Content-Type", "application/problem+json")
	c.Status(status)
	if err := json.NewEncoder(c.Writer).Encode(body); err != nil {
		_ = c.Error(err)
	}
}

func requestError(c *gin.Context, err error) {
	writeProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error())
}

func handlerError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotImplemented) {
		writeProblem(c, http.StatusNotImplemented, "not_implemented", "Not implemented", "This endpoint is not implemented in the current phase.")
		return
	}
	writeProblem(c, http.StatusInternalServerError, "internal_error", "Internal server error", "The request failed due to an internal error.")
}
