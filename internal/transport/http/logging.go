package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/infra/logging"
)

// BridgeGinLogs is process-scoped: install before any engines start and restore
// only after they stop. Engine deliberately never mutates Gin's global writers.
func BridgeGinLogs(app *slog.Logger) func() {
	previousWriter, previousError := gin.DefaultWriter, gin.DefaultErrorWriter
	previousDebug, previousRoutes := gin.DebugPrintFunc, gin.DebugPrintRouteFunc
	logger := app.With("source", "gin")
	gin.DefaultWriter = logging.Writer(logger, slog.LevelInfo)
	gin.DefaultErrorWriter = logging.Writer(logger, slog.LevelError)
	gin.DebugPrintFunc = func(format string, values ...any) { logger.Debug(strings.TrimSpace(fmt.Sprintf(format, values...))) }
	gin.DebugPrintRouteFunc = func(method, path, handler string, count int) {
		logger.Debug("route registered", slog.String("method", method), slog.String("path", path), slog.String("handler", handler), slog.Int("handlers", count))
	}
	return func() {
		gin.DefaultWriter, gin.DefaultErrorWriter = previousWriter, previousError
		gin.DebugPrintFunc, gin.DebugPrintRouteFunc = previousDebug, previousRoutes
	}
}

const appLoggerKey = "app_logger"

func recoveryLog(app *slog.Logger) gin.HandlerFunc {
	if app == nil {
		app = slog.Default()
	}
	logger := app.With("source", "gin")
	return func(c *gin.Context) {
		c.Set(appLoggerKey, app)
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(c.Request.Context(), "http request panic", slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
				if err, ok := recovered.(error); ok && (errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, http.ErrAbortHandler)) {
					_ = c.Error(err)
					c.Abort()
					return
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func logHandlerError(c *gin.Context, err error) {
	logger := slog.Default()
	if value, ok := c.Get(appLoggerKey); ok {
		if app, ok := value.(*slog.Logger); ok {
			logger = app
		}
	}
	logger.ErrorContext(c.Request.Context(), "http handler failed", slog.Any("err", err))
}
