package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func TestAuthHTTPFlow(t *testing.T) {
	db := mysql.OpenTestDB(t)
	svc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	username := "http-" + uuid.NewString()[:8]
	password := "correct horse battery staple"
	if _, err := svc.CreateUser(context.Background(), username, password); err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:               StaticReady{},
		Auth:                svc,
		RegistrationEnabled: false,
	})

	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	if csrf.Code != http.StatusOK {
		t.Fatalf("csrf %d %s", csrf.Code, csrf.Body.String())
	}
	var csrfBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrf.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}
	csrfCookie := cookieValue(csrf, auth.CSRFCookieName)
	if csrfCookie == "" || csrfBody.Token == "" {
		t.Fatal("missing csrf cookie or token")
	}

	disabled := httptest.NewRecorder()
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"username":"x","password":"correct horse battery staple"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regReq.Header.Set("X-CSRF-Token", csrfBody.Token)
	regReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfCookie})
	engine.ServeHTTP(disabled, regReq)
	if disabled.Code != http.StatusForbidden {
		t.Fatalf("register disabled %d %s", disabled.Code, disabled.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(missingCSRF, loginReq)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf %d %s", missingCSRF.Code, missingCSRF.Body.String())
	}

	login := httptest.NewRecorder()
	okReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-CSRF-Token", csrfBody.Token)
	okReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfCookie})
	engine.ServeHTTP(login, okReq)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	sessionCookie := cookieValue(login, auth.SessionCookieName)
	sessionCSRF := cookieValue(login, auth.CSRFCookieName)
	if sessionCookie == "" || sessionCSRF == "" {
		t.Fatal("missing session cookies")
	}
	if !strings.Contains(login.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("login missing no-store")
	}

	me := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	engine.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}

	logout := httptest.NewRecorder()
	outReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	outReq.Header.Set("X-CSRF-Token", sessionCSRF)
	outReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	outReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(logout, outReq)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout %d %s", logout.Code, logout.Body.String())
	}

	after := httptest.NewRecorder()
	afterReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	afterReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	engine.ServeHTTP(after, afterReq)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("after logout %d %s", after.Code, after.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	db := mysql.OpenTestDB(t)
	svc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	engine := Engine(&EngineOptions{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:  StaticReady{},
		Auth:   svc,
	})
	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	var csrfBody struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(csrf.Body.Bytes(), &csrfBody)
	csrfCookie := cookieValue(csrf, auth.CSRFCookieName)
	username := "limit-" + uuid.NewString()[:8]

	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"wrong password!!"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrfBody.Token)
		req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfCookie})
		req.RemoteAddr = "10.20.30.40:1234"
		engine.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit %d %s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestConfiguredCookieNames(t *testing.T) {
	db := mysql.OpenTestDB(t)
	svc := auth.NewService(db, false, time.Hour, 24*time.Hour)
	username := "cookie-" + uuid.NewString()[:8]
	password := "correct horse battery staple"
	if _, err := svc.CreateUser(context.Background(), username, password); err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:             StaticReady{},
		Auth:              svc,
		SessionCookieName: "plutus_session_agent",
		CSRFCookieName:    "plutus_csrf_agent",
	})

	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	if cookieValue(csrf, auth.CSRFCookieName) != "" {
		t.Fatal("default csrf cookie should be absent")
	}
	csrfCookie := cookieValue(csrf, "plutus_csrf_agent")
	var csrfBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrf.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRecorder()
	okReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-CSRF-Token", csrfBody.Token)
	okReq.AddCookie(&http.Cookie{Name: "plutus_csrf_agent", Value: csrfCookie})
	engine.ServeHTTP(login, okReq)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	if cookieValue(login, auth.SessionCookieName) != "" {
		t.Fatal("default session cookie should be absent")
	}
	sessionCookie := cookieValue(login, "plutus_session_agent")
	if sessionCookie == "" {
		t.Fatal("missing configured session cookie")
	}

	me := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "plutus_session_agent", Value: sessionCookie})
	engine.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}
}

func cookieValue(rec *httptest.ResponseRecorder, name string) string {
	for _, raw := range rec.Header().Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(raw)
		if err != nil {
			continue
		}
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
