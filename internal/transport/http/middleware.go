package httpserver

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/logging"
)

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("X-Frame-Options", "DENY")
		c.Next()
	}
}

func requestLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		traceID := c.GetHeader("X-Request-Id")
		if !validTraceID(traceID) {
			id, err := uuid.NewV7()
			if err != nil {
				id = uuid.Must(uuid.NewRandom())
			}
			traceID = id.String()
		}
		c.Set("trace_id", traceID)
		c.Request = c.Request.WithContext(logging.WithFields(c.Request.Context(), slog.String("trace_id", traceID)))
		c.Writer.Header().Set("X-Request-ID", traceID)
		c.Next()
		logger.InfoContext(c.Request.Context(), "http request",
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", time.Since(start)),
		)
	}
}

func validTraceID(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
