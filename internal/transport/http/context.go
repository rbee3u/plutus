package httpserver

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type ginContextKey struct{}

func bindGinContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), ginContextKey{}, c)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func ginFrom(ctx context.Context) *gin.Context {
	if c, ok := ctx.(*gin.Context); ok {
		return c
	}
	c, _ := ctx.Value(ginContextKey{}).(*gin.Context)
	return c
}

func limitBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func noStoreSensitive() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/auth/") || strings.Contains(path, "/identities") || strings.Contains(path, "/audit-events") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}

func rejectBadOrigin(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" || originAllowed(origin, c.Request.Host, allowed) {
			c.Next()
			return
		}
		writeProblem(c, http.StatusForbidden, "origin_forbidden", "Origin is not allowed", "The request origin is not allowed.")
		c.Abort()
	}
}

func originAllowed(origin, host string, extra []string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, host) {
		return true
	}
	for _, allowed := range extra {
		if origin == allowed {
			return true
		}
	}
	return false
}
