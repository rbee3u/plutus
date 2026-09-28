package httpserver

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/web"
)

func mountWeb(engine *gin.Engine) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(dist))
	engine.GET("/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/health") {
			writeProblem(c, http.StatusNotFound, "not_found", "Not found", "The requested resource was not found.")
			return
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			writeProblem(c, http.StatusInternalServerError, "internal_error", "Internal error", "The web shell could not be loaded.")
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}
