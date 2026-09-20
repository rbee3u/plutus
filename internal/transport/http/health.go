package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
				"reason": "database_unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
