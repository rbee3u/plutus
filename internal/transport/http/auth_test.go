package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/capital"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestAuthHTTPFlow(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	svc := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "http-" + uuid.NewString()[:8]
	password := "Correct=123456"
	if _, err := svc.CreateUser(context.Background(), username, password); err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger:   slog.New(slog.DiscardHandler),
		Ready:    StaticReady{},
		Services: &app.Services{Auth: svc},
	})

	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/csrf", nil))
	if csrf.Code != http.StatusOK {
		t.Fatalf("CSRF %d %s", csrf.Code, csrf.Body.String())
	}
	var csrfBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrf.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}
	csrfCookie := cookieValue(csrf, auth.CSRFCookieName)
	if csrfCookie == "" || csrfBody.Token == "" {
		t.Fatal("missing CSRF cookie or token")
	}

	missing := httptest.NewRecorder()
	engine.ServeHTTP(missing, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"username":"x","password":"correct horse battery staple"}`)))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("register missing %d %s", missing.Code, missing.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	loginReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(missingCSRF, loginReq)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF %d %s", missingCSRF.Code, missingCSRF.Body.String())
	}

	login := httptest.NewRecorder()
	okReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-Csrf-Token", csrfBody.Token)
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
	meReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	engine.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}

	logout := httptest.NewRecorder()
	outReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/logout", nil)
	outReq.Header.Set("X-Csrf-Token", sessionCSRF)
	outReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	outReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(logout, outReq)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout %d %s", logout.Code, logout.Body.String())
	}

	after := httptest.NewRecorder()
	afterReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	afterReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	engine.ServeHTTP(after, afterReq)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("after logout %d %s", after.Code, after.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	svc := auth.NewService(db, time.Hour, 24*time.Hour)
	engine := Engine(&EngineOptions{
		Logger:   slog.New(slog.DiscardHandler),
		Ready:    StaticReady{},
		Services: &app.Services{Auth: svc},
	})
	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/csrf", nil))
	var csrfBody struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(csrf.Body.Bytes(), &csrfBody)
	csrfCookie := cookieValue(csrf, auth.CSRFCookieName)
	username := "limit-" + uuid.NewString()[:8]

	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"wrong password!!"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Csrf-Token", csrfBody.Token)
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
	db := mysqltest.OpenTestDB(t)
	svc := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "cookie-" + uuid.NewString()[:8]
	password := "Correct=123456"
	if _, err := svc.CreateUser(context.Background(), username, password); err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger:            slog.New(slog.DiscardHandler),
		Ready:             StaticReady{},
		Services:          &app.Services{Auth: svc},
		SessionCookieName: "plutus_session_agent",
		CSRFCookieName:    "plutus_csrf_agent",
	})

	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/csrf", nil))
	if cookieValue(csrf, auth.CSRFCookieName) != "" {
		t.Fatal("default CSRF cookie should be absent")
	}
	csrfCookie := cookieValue(csrf, "plutus_csrf_agent")
	var csrfBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrf.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRecorder()
	okReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-Csrf-Token", csrfBody.Token)
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
	meReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
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

func TestPutAuthPassword(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	svc := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "change-pwd-" + uuid.NewString()[:8]
	oldPassword := "OldPass=123456"
	newPassword := "NewPass=123456"
	if _, err := svc.CreateUser(context.Background(), username, oldPassword); err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger:   slog.New(slog.DiscardHandler),
		Ready:    StaticReady{},
		Services: &app.Services{Auth: svc},
	})

	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/csrf", nil))
	csrfCookie := cookieValue(csrf, auth.CSRFCookieName)
	var csrfBody struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(csrf.Body.Bytes(), &csrfBody)

	login := httptest.NewRecorder()
	loginReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+oldPassword+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-Csrf-Token", csrfBody.Token)
	loginReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfCookie})
	engine.ServeHTTP(login, loginReq)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	sessionCookie := cookieValue(login, auth.SessionCookieName)
	sessionCSRF := cookieValue(login, auth.CSRFCookieName)

	// Weak password rejected
	weakRec := httptest.NewRecorder()
	weakReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/auth/password", bytes.NewBufferString(`{"currentPassword":"`+oldPassword+`","newPassword":"simplepassword"}`))
	weakReq.Header.Set("Content-Type", "application/json")
	weakReq.Header.Set("X-Csrf-Token", sessionCSRF)
	weakReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	weakReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(weakRec, weakReq)
	if weakRec.Code != http.StatusBadRequest {
		t.Fatalf("weak password expected 400, got %d %s", weakRec.Code, weakRec.Body.String())
	}

	// Same password rejected
	sameRec := httptest.NewRecorder()
	sameReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/auth/password", bytes.NewBufferString(`{"currentPassword":"`+oldPassword+`","newPassword":"`+oldPassword+`"}`))
	sameReq.Header.Set("Content-Type", "application/json")
	sameReq.Header.Set("X-Csrf-Token", sessionCSRF)
	sameReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	sameReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(sameRec, sameReq)
	if sameRec.Code != http.StatusBadRequest {
		t.Fatalf("same password expected 400, got %d %s", sameRec.Code, sameRec.Body.String())
	}

	// Wrong current password rejected
	wrongRec := httptest.NewRecorder()
	wrongReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/auth/password", bytes.NewBufferString(`{"currentPassword":"wrong password","newPassword":"`+newPassword+`"}`))
	wrongReq.Header.Set("Content-Type", "application/json")
	wrongReq.Header.Set("X-Csrf-Token", sessionCSRF)
	wrongReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	wrongReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(wrongRec, wrongReq)
	if wrongRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong password expected 422, got %d %s", wrongRec.Code, wrongRec.Body.String())
	}

	// Successful password change
	okRec := httptest.NewRecorder()
	okReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/auth/password", bytes.NewBufferString(`{"currentPassword":"`+oldPassword+`","newPassword":"`+newPassword+`"}`))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-Csrf-Token", sessionCSRF)
	okReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	okReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: sessionCSRF})
	engine.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusNoContent {
		t.Fatalf("change password expected 204, got %d %s", okRec.Code, okRec.Body.String())
	}

	// Verify session cookie was cleared in response headers
	clearedCookie := cookieValue(okRec, auth.SessionCookieName)
	if clearedCookie != "" {
		t.Fatalf("session cookie should be cleared, got %q", clearedCookie)
	}

	// Old session is now unauthenticated
	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	engine.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("old session should be unauthorized, got %d %s", meRec.Code, meRec.Body.String())
	}
}

// otherUserSession supplies a real second principal for ownership checks.
func otherUserSession(t *testing.T, svc *auth.Service) auth.Session {
	t.Helper()
	user, err := svc.CreateUser(t.Context(), "foreign-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := svc.Login(t.Context(), user.Username, "Correct=123456", "127.0.0.2", "foreign-user")
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestAuthenticationStorageFailureIsNotSessionExpiry(t *testing.T) {
	// Use an independent closed pool; the shared test database stays usable.
	db, err := sql.Open("mysql", mysqltest.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{Auth: auth.NewService(db, time.Hour, 24*time.Hour)}})
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/auth/csrf", "/api/v1/auth/sessions"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "existing-session"})
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("storage failure: %d %s", response.Code, response.Body.String())
		}
		if len(response.Header().Values("Set-Cookie")) != 0 {
			t.Fatal("storage failure replaced session cookies")
		}
	}
}

func TestPasswordHashFailureIsNotInvalidCredentials(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	svc := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "hash-failure-" + uuid.NewString()[:8]
	user, err := svc.CreateUser(t.Context(), username, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := svc.Login(t.Context(), username, "Correct=123456", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE users SET password_hash=? WHERE id=?", "invalid-hash", user.ID[:]); err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{Auth: svc}})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/auth/login", `{"username":"` + username + `","password":"Correct=123456"}`},
		{http.MethodPut, "/api/v1/auth/password", `{"currentPassword":"Correct=123456","newPassword":"Replacement=123456"}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", session.CSRF)
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
			req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: session.CSRF})
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
				t.Fatalf("hash failure: %d %s", response.Code, response.Body.String())
			}
			if len(response.Header().Values("Set-Cookie")) != 0 {
				t.Fatal("hash failure replaced session cookies")
			}
		})
	}
	if _, _, err := svc.Authenticate(t.Context(), session.Token); err != nil {
		t.Fatalf("hash failure invalidated the existing session: %v", err)
	}
}

func TestIdempotentCreationTouchesSessionOnce(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "create-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("owned portfolios: %+v %v", owned, err)
	}
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "Creation test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{
		DB: db, Auth: authentication, Portfolios: portfolios, Capital: capital.NewService(db), Idempotency: idempotency.NewStore(db),
	}})
	request := func(owner uuid.UUID, key, body, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/portfolios/"+owner.String()+"/capital-flows", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("X-Csrf-Token", csrf)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
		recorder := httptest.NewRecorder()
		stats.Queries.Store(0)
		stats.Writes.Store(0)
		engine.ServeHTTP(recorder, req)
		return recorder
	}
	body := `{"direction":"contribution","canonicalAssetSymbol":"USD","quantity":"1","effectiveAt":"2025-01-01T00:00:00Z"}`
	first := request(owned[0].ID, "create-once", body, session.CSRF)
	if first.Code != http.StatusCreated || stats.Writes.Load() != 4 || stats.Queries.Load() != 7 {
		t.Fatalf("first creation: %d %s reads=%d writes=%d", first.Code, first.Body.String(), stats.Queries.Load(), stats.Writes.Load())
	}
	replay := request(owned[0].ID, "create-once", body, session.CSRF)
	if replay.Code != http.StatusCreated || replay.Body.String() != first.Body.String() || stats.Writes.Load() != 1 || stats.Queries.Load() != 3 {
		t.Fatalf("replay: %d %s reads=%d writes=%d", replay.Code, replay.Body.String(), stats.Queries.Load(), stats.Writes.Load())
	}
	for _, tc := range []struct {
		owner           uuid.UUID
		key, body, csrf string
		status          int
	}{
		{owned[0].ID, "create-once", strings.Replace(body, `"1"`, `"2"`, 1), session.CSRF, http.StatusConflict},
		{owned[0].ID, "create-once", body, "incorrect", http.StatusForbidden},
		{uuid.New(), "foreign", body, session.CSRF, http.StatusNotFound},
	} {
		response := request(tc.owner, tc.key, tc.body, tc.csrf)
		if response.Code != tc.status || stats.Writes.Load() != 1 {
			t.Fatalf("rejected creation: %d %s writes=%d", response.Code, response.Body.String(), stats.Writes.Load())
		}
	}
	if err := authentication.Logout(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	response := request(owned[0].ID, "create-once", body, session.CSRF)
	if response.Code != http.StatusUnauthorized || stats.Writes.Load() != 0 || stats.Queries.Load() != 1 {
		t.Fatalf("revoked replay: %d %s reads=%d writes=%d", response.Code, response.Body.String(), stats.Queries.Load(), stats.Writes.Load())
	}
}

func TestIdentityAndCycleCreationTouchSessionOnce(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "create-resources-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %+v %v", owned, err)
	}
	identities, err := identity.NewService(db, identity.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "Creation test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{
		DB: db, Auth: authentication, Portfolios: portfolios, Identities: identities, Cycles: cycle.NewService(db), Idempotency: idempotency.NewStore(db),
	}})
	for _, tc := range []struct {
		path, body string
		status     int
		writes     int64
	}{
		{"identities", `{"platform":"binance","displayName":"Created once","accountStructure":"binance_classic","selectedAccountTypes":["spot"]}`, http.StatusCreated, int64(len(adapter.AccountTypes(adapter.StructureBinanceClassic)) + 6)},
		{"collection-cycles", `{}`, http.StatusAccepted, 3},
	} {
		t.Run(tc.path, func(t *testing.T) {
			request := func() *httptest.ResponseRecorder {
				req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/portfolios/"+owned[0].ID.String()+"/"+tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", "resource-once")
				req.Header.Set("X-Csrf-Token", session.CSRF)
				req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
				response := httptest.NewRecorder()
				stats.Queries.Store(0)
				stats.Writes.Store(0)
				engine.ServeHTTP(response, req)
				return response
			}
			first := request()
			if first.Code != tc.status || stats.Queries.Load() != 8 || stats.Writes.Load() != tc.writes {
				t.Fatalf("creation: %d %s reads=%d writes=%d", first.Code, first.Body.String(), stats.Queries.Load(), stats.Writes.Load())
			}
			replay := request()
			if replay.Code != tc.status || replay.Body.String() != first.Body.String() || stats.Queries.Load() != 3 || stats.Writes.Load() != 1 {
				t.Fatalf("replay: %d %s reads=%d writes=%d", replay.Code, replay.Body.String(), stats.Queries.Load(), stats.Writes.Load())
			}
		})
	}
}

func TestCreationRevalidatesAfterWaitingForUserLock(t *testing.T) {
	for _, resource := range []struct{ path, body, table string }{
		{"capital-flows", `{"direction":"contribution","canonicalAssetSymbol":"USD","quantity":"1","effectiveAt":"2025-01-01T00:00:00Z"}`, "capital_flows"},
		{"identities", `{"platform":"binance","displayName":"Blocked create","accountStructure":"binance_classic","selectedAccountTypes":["spot"]}`, "identities"},
		{"collection-cycles", `{}`, "collection_cycles"},
	} {
		for _, invalidation := range []string{"revoked", "disabled", "expired", "password_reset"} {
			t.Run(resource.path+"/"+invalidation, func(t *testing.T) {
				db, stats := mysqltest.ObserveQueries(t)
				ctx := t.Context()
				authentication := auth.NewService(db, time.Hour, 24*time.Hour)
				username := "blocked-" + uuid.NewString()[:8]
				user, err := authentication.CreateUser(ctx, username, "Correct=123456")
				if err != nil {
					t.Fatal(err)
				}
				portfolios := portfolio.NewService(db)
				owned, err := portfolios.List(ctx, user.ID)
				if err != nil {
					t.Fatal(err)
				}
				_, session, err := authentication.Login(ctx, username, "Correct=123456", "127.0.0.1", "Blocked create")
				if err != nil {
					t.Fatal(err)
				}
				identities, err := identity.NewService(db, identity.Options{})
				if err != nil {
					t.Fatal(err)
				}
				engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{
					DB: db, Auth: authentication, Portfolios: portfolios, Capital: capital.NewService(db), Identities: identities, Cycles: cycle.NewService(db), Idempotency: idempotency.NewStore(db),
				}})
				blocker, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = blocker.Rollback() }()
				q := sqlcgen.New(blocker)
				if _, err := q.LockUser(ctx, user.ID[:]); err != nil {
					t.Fatal(err)
				}
				stats.Writes.Store(0)
				req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/portfolios/"+owned[0].ID.String()+"/"+resource.path, strings.NewReader(resource.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Csrf-Token", session.CSRF)
				req.Header.Set("Idempotency-Key", "blocked-create")
				req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
				response := httptest.NewRecorder()
				done := make(chan struct{})
				go func() { defer close(done); engine.ServeHTTP(response, req) }()
				deadline := time.Now().Add(5 * time.Second)
				for stats.Writes.Load() < 1 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if stats.Writes.Load() != 1 {
					t.Fatal("initial session touch did not complete")
				}
				// The invalidation commits before the waiting create obtains its lock.
				switch invalidation {
				case "revoked":
					err = q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{UserID: user.ID[:], RevokedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true}})
				case "disabled":
					err = q.SetUserStatus(ctx, sqlcgen.SetUserStatusParams{ID: user.ID[:], Status: "disabled", UpdatedAt: time.Now().UTC()})
				case "expired":
					_, err = blocker.ExecContext(ctx, "UPDATE sessions SET absolute_expires_at=? WHERE id=?", time.Now().UTC().Add(-time.Minute), session.ID[:])
				case "password_reset":
					hash, hashErr := auth.HashPassword("Changed=123456")
					if hashErr != nil {
						t.Fatal(hashErr)
					}
					err = q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{ID: user.ID[:], PasswordHash: hash, UpdatedAt: time.Now().UTC()})
					if err == nil {
						err = q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{UserID: user.ID[:], RevokedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true}})
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := blocker.Commit(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("create remained blocked")
				}
				var records, retries int
				// Resource table names come only from the fixed test cases above.
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+resource.table+" WHERE portfolio_id=?", owned[0].ID[:]).Scan(&records); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM idempotency_records WHERE user_id=?", user.ID[:]).Scan(&retries); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusUnauthorized || records != 0 || retries != 0 {
					t.Fatalf("invalidated session wrote data: status=%d records=%d retries=%d body=%s", response.Code, records, retries, response.Body.String())
				}
			})
		}
	}
}
