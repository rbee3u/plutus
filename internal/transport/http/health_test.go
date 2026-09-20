package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func TestLiveAndReady(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := Engine(logger, StaticReady{Err: errors.New("down")})

	live := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	engine.ServeHTTP(live, req)
	if live.Code != http.StatusOK {
		t.Fatalf("live status %d", live.Code)
	}
	if live.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff header")
	}

	ready := httptest.NewRecorder()
	engine.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status %d", ready.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(ready.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["reason"] != "database_unavailable" {
		t.Fatalf("body %+v", body)
	}

	okEngine := Engine(logger, StaticReady{})
	okReady := httptest.NewRecorder()
	okEngine.ServeHTTP(okReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if okReady.Code != http.StatusOK {
		t.Fatalf("ready ok status %d", okReady.Code)
	}

	schemaEngine := Engine(logger, StaticReady{Err: mysql.ErrSchemaMismatch})
	schemaReady := httptest.NewRecorder()
	schemaEngine.ServeHTTP(schemaReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if schemaReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("schema ready status %d", schemaReady.Code)
	}
	var schemaBody map[string]string
	if err := json.Unmarshal(schemaReady.Body.Bytes(), &schemaBody); err != nil {
		t.Fatal(err)
	}
	if schemaBody["reason"] != "schema_mismatch" {
		t.Fatalf("schema body %+v", schemaBody)
	}
}
