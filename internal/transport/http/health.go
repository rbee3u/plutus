package httpserver

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func registerHealth(engine *gin.Engine, ready ReadyChecker) {
	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live"})
	})
	engine.GET("/health/ready", func(c *gin.Context) {
		if ready == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"reason": "readiness_unchecked",
			})
			return
		}
		if err := ready.Ready(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"reason": readyReason(err),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}

func readyReason(err error) string {
	switch {
	case errors.Is(err, mysql.ErrSchemaMismatch):
		return mysql.ErrSchemaMismatch.Error()
	case errors.Is(err, mysql.ErrMySQLIncompatible):
		return mysql.ErrMySQLIncompatible.Error()
	default:
		return mysql.ErrDatabaseUnavailable.Error()
	}
}
