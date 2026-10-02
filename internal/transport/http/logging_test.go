package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/logging"
)

func TestFailedListenDoesNotLogStarted(t *testing.T) {
	t.Parallel()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("expected TCP listener")
	}
	cfg := &config.Config{Server: config.Server{ListenAddress: "127.0.0.1", Port: addr.Port}}
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	if err := Run(context.Background(), cfg, logger, http.NotFoundHandler()); err == nil {
		t.Fatal("expected occupied port to fail")
	}
	if strings.Contains(output.String(), "http server started") {
		t.Fatalf("failed server logged successful startup: %s", output.String())
	}
}

func TestAccessRecoveryAndContextLogging(t *testing.T) {
	t.Parallel()
	if err := os.MkdirAll("/tmp/plutus", 0o750); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("/tmp/plutus", "http-logging-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	logs, err := logging.New(logging.Options{Directory: dir, MaxSizeMB: 1, SQLEnabled: true}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logs.Close() }()
	engine := Engine(&EngineOptions{Logger: logs.App, AccessLogger: logs.Access, Ready: FuncReady(func(ctx context.Context) error {
		logs.SQL.InfoContext(ctx, "ready correlation")
		return nil
	})})
	engine.GET("/panic", func(*gin.Context) { panic("test panic") })
	engine.GET("/failure", func(c *gin.Context) { handlerError(c, errors.New("unexpected handler failure")) })
	engine.GET("/broken", func(*gin.Context) { panic(&net.OpError{Op: "write", Err: os.NewSyscallError("write", syscall.EPIPE)}) })
	engine.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
	engine.GET("/context", func(c *gin.Context) {
		logs.SQL.InfoContext(c.Request.Context(), "correlated SQL")
		logs.App.InfoContext(c.Request.Context(), "correlated application")
		c.Status(http.StatusOK)
	})
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/health/live", 200},
		{http.MethodGet, "/health/ready", 200},
		{http.MethodGet, "/api/v1/unknown", 404},
		{http.MethodPost, "/api/v1/auth/login", 403},
		{http.MethodGet, "/panic", 500},
		{http.MethodGet, "/failure", 500},
		{http.MethodGet, "/broken", 200},
		{http.MethodGet, "/abort", 200},
		{http.MethodGet, "/context", 200},
		{http.MethodGet, "/", 200},
	} {
		req := httptest.NewRequest(tc.method, tc.path+"?hidden-query", nil)
		req.Header.Set("X-Request-Id", "test-request")
		req.Header.Set("Cookie", "sensitive-cookie-value")
		if tc.method == http.MethodPost {
			req.Header.Set("Origin", "https://untrusted.example")
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Fatalf("%s: status %d, want %d", tc.path, response.Code, tc.status)
		}
		if response.Header().Get("X-Request-Id") != "test-request" {
			t.Fatal("request id not echoed")
		}
	}
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, name+".log"))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	app, access, sql := read("app"), read("access"), read("sql")
	if strings.Count(access, "http request") != 10 || strings.Count(app, "http request panic") != 3 || strings.Contains(app, "http request\"") {
		t.Fatalf("wrong request/panic counts: %s %s", app, access)
	}
	for _, text := range []string{app, access, sql} {
		if strings.Contains(text, "sensitive-cookie-value") || strings.Contains(text, "hidden-query") {
			t.Fatal("request metadata leaked")
		}
	}
	if !strings.Contains(app, "http handler failed") || !strings.Contains(app, "trace_id=test-request") || !strings.Contains(sql, "trace_id=test-request") {
		t.Fatal("diagnostic/correlation fields missing")
	}
	for _, line := range strings.Split(sql, "\n") {
		if strings.Contains(line, "ready correlation") && !strings.Contains(line, "trace_id=test-request") {
			t.Fatal("generated API lost request context")
		}
	}
}

func TestGeneratedHandlersCarryRequestCancellation(t *testing.T) {
	t.Parallel()
	var observed error
	engine := Engine(&EngineOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: FuncReady(func(ctx context.Context) error {
		observed = ctx.Err()
		return nil
	})})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil).WithContext(ctx)
	engine.ServeHTTP(httptest.NewRecorder(), request)
	if !errors.Is(observed, context.Canceled) {
		t.Fatal("generated API lost request cancellation")
	}
}

func TestRequestIDValidation(t *testing.T) {
	t.Parallel()
	engine := Engine(&EngineOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, value := range []string{"", "has spaces", "非ASCII", strings.Repeat("x", 129), "contains\nnewline", "valid.ID_42-foo", strings.Repeat("x", 128)} {
		request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		request.Header.Set("X-Request-Id", value)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		id := response.Header().Get("X-Request-Id")
		if validTraceID(value) {
			if id != value {
				t.Fatalf("valid id changed: %q", id)
			}
		} else if _, err := uuid.Parse(id); err != nil {
			t.Fatalf("invalid id not replaced: %q", id)
		}
	}
}

func TestGinFrameworkBridge(t *testing.T) {
	// Gin globals are deliberately tested serially; parallel engines never alter them.
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previousWriter, previousError := gin.DefaultWriter, gin.DefaultErrorWriter
	restore := BridgeGinLogs(logger)
	_, _ = fmt.Fprintln(gin.DefaultWriter, "framework info")
	_, _ = fmt.Fprintln(gin.DefaultErrorWriter, "framework error")
	gin.DebugPrintFunc("debug %s\n", "message")
	gin.DebugPrintRouteFunc("GET", "/example", "handler", 2)
	restore()
	if gin.DefaultWriter != previousWriter || gin.DefaultErrorWriter != previousError {
		t.Fatal("Gin writers not restored")
	}
	text := output.String()
	if strings.Count(text, "source=gin") != 4 || !strings.Contains(text, "level=ERROR") || !strings.Contains(text, "level=DEBUG") || !strings.Contains(text, "route registered") {
		t.Fatalf("framework log missing: %s", text)
	}
	if gin.Mode() != gin.ReleaseMode {
		t.Fatal("bridge changed Gin mode")
	}
}
