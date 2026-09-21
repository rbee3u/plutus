package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/portfolio"
)

func TestPortfolioOwnershipAndIdentity(t *testing.T) {
	db := mysql.OpenTestDB(t)
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	portfolios := portfolio.NewService(db)
	accessSvc := access.NewService(db, 15*time.Minute, 30*time.Minute, nil)
	engine := Engine(&EngineOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:       StaticReady{},
		Auth:        authSvc,
		Portfolios:  portfolios,
		Access:      accessSvc,
		Idempotency: idempotency.NewStore(db),
	})

	password := "correct horse battery staple"
	userA := "pa-" + uuid.NewString()[:8]
	userB := "pb-" + uuid.NewString()[:8]
	if _, err := authSvc.CreateUser(context.Background(), userA, password); err != nil {
		t.Fatal(err)
	}
	if _, err := authSvc.CreateUser(context.Background(), userB, password); err != nil {
		t.Fatal(err)
	}
	sessionA := loginSession(t, engine, userA, password)
	sessionB := loginSession(t, engine, userB, password)

	list := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios", nil, sessionA, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Undeletable bool   `json:"undeletable"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "default" || !listed.Items[0].Undeletable {
		t.Fatalf("listed %+v", listed)
	}
	mainA := listed.Items[0].ID

	created := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios", map[string]any{"name": "trading"}, sessionA, "idem-1")
	if created.Code != http.StatusCreated {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	etag := created.Header().Get("ETag")
	var createdBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}

	replay := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios", map[string]any{"name": "trading"}, sessionA, "idem-1")
	if replay.Code != http.StatusCreated {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	conflict := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios", map[string]any{"name": "Other"}, sessionA, "idem-1")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict %d %s", conflict.Code, conflict.Body.String())
	}

	foreign := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+mainA, nil, sessionB, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-user %d %s", foreign.Code, foreign.Body.String())
	}

	stale := patchWithMatch(t, engine, "/api/v1/portfolios/"+createdBody.ID, map[string]any{"name": "Renamed"}, sessionA, `"portfolio:999"`)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale etag %d %s", stale.Code, stale.Body.String())
	}
	okPatch := patchWithMatch(t, engine, "/api/v1/portfolios/"+createdBody.ID, map[string]any{"name": "Renamed"}, sessionA, etag)
	if okPatch.Code != http.StatusOK {
		t.Fatalf("patch %d %s", okPatch.Code, okPatch.Body.String())
	}

	badMode := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+mainA+"/identities", map[string]any{
		"platformId":  catalog.PlatformBinance.String(),
		"externalRef": "acct-1",
		"displayName": "Binance",
		"accountMode": "spot",
		"kind":        "main",
	}, sessionA, "")
	if badMode.Code != http.StatusBadRequest {
		t.Fatalf("bad mode %d %s", badMode.Code, badMode.Body.String())
	}

	ident := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+mainA+"/identities", map[string]any{
		"platformId":  catalog.PlatformBinance.String(),
		"externalRef": "acct-1",
		"displayName": "Binance",
		"accountMode": "binance_pm",
		"kind":        "main",
	}, sessionA, "")
	if ident.Code != http.StatusCreated {
		t.Fatalf("identity %d %s", ident.Code, ident.Body.String())
	}
	var identBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ident.Body.Bytes(), &identBody); err != nil {
		t.Fatal(err)
	}

	blocked := deleteWithMatch(t, engine, "/api/v1/portfolios/"+mainA, sessionA, listETag(t, engine, "/api/v1/portfolios/"+mainA, sessionA))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("delete undeletable %d %s", blocked.Code, blocked.Body.String())
	}

	extraIdent := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+createdBody.ID+"/identities", map[string]any{
		"platformId":  catalog.PlatformBinance.String(),
		"externalRef": "acct-extra",
		"displayName": "Binance extra",
		"accountMode": "binance_pm",
		"kind":        "main",
	}, sessionA, "")
	if extraIdent.Code != http.StatusCreated {
		t.Fatalf("extra identity %d %s", extraIdent.Code, extraIdent.Body.String())
	}
	removed := deleteWithMatch(t, engine, "/api/v1/portfolios/"+createdBody.ID, sessionA, listETag(t, engine, "/api/v1/portfolios/"+createdBody.ID, sessionA))
	if removed.Code != http.StatusNoContent {
		t.Fatalf("cascade delete %d %s", removed.Code, removed.Body.String())
	}
	gone := doJSON(t, engine, http.MethodGet, "/api/v1/portfolios/"+createdBody.ID, nil, sessionA, "")
	if gone.Code != http.StatusNotFound {
		t.Fatalf("deleted extra %d %s", gone.Code, gone.Body.String())
	}

	enabled := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+mainA+"/connections", map[string]any{
		"identityId": identBody.ID,
		"label":      "primary",
		"status":     "enabled",
	}, sessionA, "")
	if enabled.Code != http.StatusBadRequest {
		t.Fatalf("enabled without credentials %d %s", enabled.Code, enabled.Body.String())
	}
	conn := doJSON(t, engine, http.MethodPost, "/api/v1/portfolios/"+mainA+"/connections", map[string]any{
		"identityId": identBody.ID,
		"label":      "primary",
	}, sessionA, "")
	if conn.Code != http.StatusCreated {
		t.Fatalf("connection %d %s", conn.Code, conn.Body.String())
	}
	var connBody struct {
		CredentialsConfigured bool `json:"credentialsConfigured"`
	}
	if err := json.Unmarshal(conn.Body.Bytes(), &connBody); err != nil {
		t.Fatal(err)
	}
	if connBody.CredentialsConfigured {
		t.Fatal("credentials should not be configured")
	}
}

type sessionCookies struct {
	session string
	csrf    string
}

func loginSession(t *testing.T, engine http.Handler, username, password string) sessionCookies {
	t.Helper()
	csrfRec := httptest.NewRecorder()
	engine.ServeHTTP(csrfRec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	var csrfBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrfRec.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}
	csrfCookie := cookieValue(csrfRec, auth.CSRFCookieName)
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfBody.Token)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfCookie})
	engine.ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	return sessionCookies{session: cookieValue(login, auth.SessionCookieName), csrf: cookieValue(login, auth.CSRFCookieName)}
}

func doJSON(t *testing.T, engine http.Handler, method, path string, body any, sess sessionCookies, idem string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-CSRF-Token", sess.csrf)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.session})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sess.csrf})
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func patchWithMatch(t *testing.T, engine http.Handler, path string, body any, sess sessionCookies, match string) *httptest.ResponseRecorder {
	t.Helper()
	return jsonWithMatch(t, engine, http.MethodPatch, path, body, sess, match, "")
}

func jsonWithMatch(t *testing.T, engine http.Handler, method, path string, body any, sess sessionCookies, match, idem string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-CSRF-Token", sess.csrf)
	if match != "" {
		req.Header.Set("If-Match", match)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.session})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sess.csrf})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func deleteWithMatch(t *testing.T, engine http.Handler, path string, sess sessionCookies, match string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("X-CSRF-Token", sess.csrf)
	req.Header.Set("If-Match", match)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.session})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sess.csrf})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func listETag(t *testing.T, engine http.Handler, path string, sess sessionCookies) string {
	t.Helper()
	rec := doJSON(t, engine, http.MethodGet, path, nil, sess, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get %s %d %s", path, rec.Code, rec.Body.String())
	}
	return rec.Header().Get("ETag")
}
