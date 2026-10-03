package httpserver

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/worker"
	"github.com/rbee3u/plutus/web"
)

func TestLiveAndReady(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	engine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: errors.New("down")}})

	live := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/live", nil)
	engine.ServeHTTP(live, req)
	if live.Code != http.StatusOK {
		t.Fatalf("live status %d", live.Code)
	}
	if live.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff header")
	}

	ready := httptest.NewRecorder()
	engine.ServeHTTP(ready, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", nil))
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

	okEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}})
	okReady := httptest.NewRecorder()
	okEngine.ServeHTTP(okReady, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", nil))
	if okReady.Code != http.StatusOK {
		t.Fatalf("ready ok status %d", okReady.Code)
	}

	schemaEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: mysql.ErrSchemaMismatch}})
	schemaReady := httptest.NewRecorder()
	schemaEngine.ServeHTTP(schemaReady, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", nil))
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

	workerEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: worker.ErrUnavailable}})
	workerReady := httptest.NewRecorder()
	workerEngine.ServeHTTP(workerReady, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", nil))
	if workerReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("worker ready status %d", workerReady.Code)
	}
	var workerBody map[string]string
	if err := json.Unmarshal(workerReady.Body.Bytes(), &workerBody); err != nil {
		t.Fatal(err)
	}
	if workerBody["reason"] != "worker_unavailable" {
		t.Fatalf("worker body %+v", workerBody)
	}

	keyEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: mysql.ErrMasterKeyRequired}})
	keyReady := httptest.NewRecorder()
	keyEngine.ServeHTTP(keyReady, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", nil))
	if keyReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("master key ready status %d", keyReady.Code)
	}
	var keyBody map[string]string
	if err := json.Unmarshal(keyReady.Body.Bytes(), &keyBody); err != nil {
		t.Fatal(err)
	}
	if keyBody["reason"] != "master_key_required" {
		t.Fatalf("master key body %+v", keyBody)
	}

	csrf := httptest.NewRecorder()
	Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}}).ServeHTTP(csrf, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/csrf", nil))
	if csrf.Code != http.StatusOK {
		t.Fatalf("csrf %d %s", csrf.Code, csrf.Body.String())
	}

	me := httptest.NewRecorder()
	Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}}).ServeHTTP(me, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil))
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated me %d %s", me.Code, me.Body.String())
	}
}

func TestEmbeddedShellServesIndex(t *testing.T) {
	t.Parallel()

	engine := Engine(&EngineOptions{
		Logger: slog.New(slog.DiscardHandler),
		Ready:  StaticReady{},
	})

	login := httptest.NewRecorder()
	engine.ServeHTTP(login, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))
	if login.Code != http.StatusOK {
		t.Fatalf("login spa status %d", login.Code)
	}
	if !strings.Contains(strings.ToLower(login.Body.String()), "<!doctype html>") {
		t.Fatalf("login spa body %s", login.Body.String())
	}
	if got := login.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
		t.Fatalf("login spa Cache-Control = %q, want %q", got, "no-cache, no-store, must-revalidate")
	}

	account := httptest.NewRecorder()
	engine.ServeHTTP(account, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account/security", nil))
	if account.Code != http.StatusOK {
		t.Fatalf("account spa status %d", account.Code)
	}
	if !strings.Contains(strings.ToLower(account.Body.String()), "<!doctype html>") {
		t.Fatalf("account spa body %s", account.Body.String())
	}
	if got := account.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
		t.Fatalf("account spa Cache-Control = %q, want %q", got, "no-cache, no-store, must-revalidate")
	}

	entries, err := fs.ReadDir(web.Dist, "dist/assets")
	if err != nil || len(entries) == 0 {
		t.Fatalf("read dist/assets: %v (len %d)", err, len(entries))
	}
	assetPath := "/assets/" + entries[0].Name()
	asset := httptest.NewRecorder()
	engine.ServeHTTP(asset, httptest.NewRequestWithContext(t.Context(), http.MethodGet, assetPath, nil))
	if asset.Code != http.StatusOK {
		t.Fatalf("asset status %d, want 200", asset.Code)
	}
	if got := asset.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q, want %q", got, "public, max-age=31536000, immutable")
	}

	unknownAPI := httptest.NewRecorder()
	engine.ServeHTTP(unknownAPI, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/nope", nil))
	if unknownAPI.Code != http.StatusNotFound {
		t.Fatalf("unknown api status %d", unknownAPI.Code)
	}
	if strings.Contains(strings.ToLower(unknownAPI.Body.String()), "<!doctype html>") {
		t.Fatal("unknown API path should not serve the SPA")
	}
}
