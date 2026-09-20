package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/worker"
)

func TestLiveAndReady(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: errors.New("down")}, RegistrationEnabled: false})

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

	okEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}, RegistrationEnabled: false})
	okReady := httptest.NewRecorder()
	okEngine.ServeHTTP(okReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if okReady.Code != http.StatusOK {
		t.Fatalf("ready ok status %d", okReady.Code)
	}

	schemaEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: mysql.ErrSchemaMismatch}, RegistrationEnabled: false})
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

	workerEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: worker.ErrUnavailable}, RegistrationEnabled: false})
	workerReady := httptest.NewRecorder()
	workerEngine.ServeHTTP(workerReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
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

	keyEngine := Engine(&EngineOptions{Logger: logger, Ready: StaticReady{Err: mysql.ErrMasterKeyRequired}, RegistrationEnabled: false})
	keyReady := httptest.NewRecorder()
	keyEngine.ServeHTTP(keyReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
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

	reg := httptest.NewRecorder()
	Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}, RegistrationEnabled: true}).ServeHTTP(reg, httptest.NewRequest(http.MethodGet, "/api/v1/auth/registration-status", nil))
	if reg.Code != http.StatusOK {
		t.Fatalf("registration status %d %s", reg.Code, reg.Body.String())
	}

	me := httptest.NewRecorder()
	Engine(&EngineOptions{Logger: logger, Ready: StaticReady{}, RegistrationEnabled: false}).ServeHTTP(me, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated me %d %s", me.Code, me.Body.String())
	}
}

func TestEmbeddedShellServesIndex(t *testing.T) {
	t.Parallel()

	engine := Engine(&EngineOptions{
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:               StaticReady{},
		RegistrationEnabled: false,
	})

	login := httptest.NewRecorder()
	engine.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
	if login.Code != http.StatusOK {
		t.Fatalf("login spa status %d", login.Code)
	}
	if !strings.Contains(strings.ToLower(login.Body.String()), "<!doctype html>") {
		t.Fatalf("login spa body %s", login.Body.String())
	}

	unknownAPI := httptest.NewRecorder()
	engine.ServeHTTP(unknownAPI, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if unknownAPI.Code != http.StatusNotFound {
		t.Fatalf("unknown api status %d", unknownAPI.Code)
	}
	if strings.Contains(strings.ToLower(unknownAPI.Body.String()), "<!doctype html>") {
		t.Fatal("unknown API path should not serve the SPA")
	}
}
