package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/manual"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestManualHTTPMutationPreconditionsAndOwnership(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "manual-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, username, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	registry := adapters.NewRegistry()
	registry.Register(adapters.PlatformOKX, fake.New(adapters.PlatformOKX))
	identities, err := identity.NewService(db, identity.Options{MasterKey: []byte("0123456789abcdef0123456789abcdef"), Adapters: registry})
	if err != nil {
		t.Fatal(err)
	}
	item, err := identities.Create(ctx, owned[0].ID, &identity.CreateInput{ActorUserID: user.ID, DisplayName: "Manual HTTP", Platform: adapters.PlatformOKX, AccountStructure: adapters.StructureOKXUnified, SelectedAccountTypes: []adapters.AccountType{"trading"}})
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := authentication.Login(ctx, username, "Correct=123456", "127.0.0.1", "Manual test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Auth: authentication, Portfolios: portfolios, Manual: manual.NewService(db)})
	csrf := httptest.NewRecorder()
	engine.ServeHTTP(csrf, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/csrf", nil))
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(csrf.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, etag string, authenticated, csrfPresent bool) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		if authenticated {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		if csrfPresent {
			req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: session.CSRF, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
			req.Header.Set("X-CSRF-Token", session.CSRF)
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	base := "/api/v1/portfolios/" + owned[0].ID.String() + "/identities/" + item.ID.String() + "/manual-balances"
	read := request("GET", base, "", "", true, false)
	if read.Code != 200 || read.Header().Get("ETag") == "" {
		t.Fatalf("read %d %s", read.Code, read.Body.String())
	}
	etag := read.Header().Get("ETag")
	for _, tc := range []struct {
		path, etag string
		auth, csrf bool
		code       int
	}{
		{base, etag, false, true, 401}, {base, etag, true, false, 403}, {base, "", true, true, 400}, {base, `"identity:1"`, true, true, 400}, {"/api/v1/portfolios/" + uuid.NewString() + "/identities/" + item.ID.String() + "/manual-balances", etag, true, true, 404},
	} {
		res := request("POST", tc.path, `{"symbol":"USD","quantity":"100"}`, tc.etag, tc.auth, tc.csrf)
		if res.Code != tc.code {
			t.Fatalf("precondition want %d got %d %s", tc.code, res.Code, res.Body.String())
		}
	}
	added := request("POST", base, `{"symbol":"usd","quantity":"100"}`, etag, true, true)
	var state api.ManualBalanceState
	if added.Code != 200 || json.Unmarshal(added.Body.Bytes(), &state) != nil || !state.Pending || state.Items[0].Quantity != "100" {
		t.Fatalf("add %d %s", added.Code, added.Body.String())
	}
	next := added.Header().Get("ETag")
	if res := request("POST", base, `{"symbol":"USD","quantity":"10"}`, next, true, true); res.Code != 409 {
		t.Fatalf("duplicate %d %s", res.Code, res.Body.String())
	}
	if res := request("PATCH", base+"/USD", `{"quantity":"-100"}`, etag, true, true); res.Code != 412 {
		t.Fatalf("concurrent %d %s", res.Code, res.Body.String())
	}
	edited := request("PATCH", base+"/USD", `{"quantity":"-100"}`, next, true, true)
	if edited.Code != 200 {
		t.Fatalf("edit %d %s", edited.Code, edited.Body.String())
	}
	deleted := request("DELETE", base+"/USD", "", edited.Header().Get("ETag"), true, true)
	if deleted.Code != 200 || json.Unmarshal(deleted.Body.Bytes(), &state) != nil || len(state.Items) != 0 || state.Pending {
		t.Fatalf("delete %d %s", deleted.Code, deleted.Body.String())
	}
}
