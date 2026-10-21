package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/manual"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestManualHTTPMutationPreconditionsAndOwnership(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
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
	registry := adapter.NewRegistry()
	registry.Register(adapter.PlatformOKX, fake.New(adapter.PlatformOKX))
	identities, err := identity.NewService(db, identity.Options{MasterKey: []byte("0123456789abcdef0123456789abcdef"), Adapters: registry})
	if err != nil {
		t.Fatal(err)
	}
	item, err := identities.Create(ctx, owned[0].ID, &identity.CreateInput{ActorUserID: user.ID, DisplayName: "Manual HTTP", Platform: adapter.PlatformOKX, AccountStructure: adapter.StructureOKXUnified, SelectedAccountTypes: []adapter.AccountType{"trading"}})
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := authentication.Login(ctx, username, "Correct=123456", "127.0.0.1", "Manual test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Services: &app.Services{Auth: authentication, Portfolios: portfolios, Manual: manual.NewService(db, catalog.NewService(db)), Identities: identities}})
	request := func(method, path, body, version string, authenticated, csrfPresent bool) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if version != "" {
			req.Header.Set("If-Edit-Version", version)
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
	if read.Code != 200 || read.Header().Get("ETag") != "" {
		t.Fatalf("read %d %s", read.Code, read.Body.String())
	}
	version := "1"
	for _, tc := range []struct {
		path, version string
		auth, csrf    bool
		code          int
	}{
		{base, version, false, true, 401}, {base, version, true, false, 403}, {base, "", true, true, 400}, {base, `"identity:1"`, true, true, 400}, {"/api/v1/portfolios/" + uuid.NewString() + "/identities/" + item.ID.String() + "/manual-balances", version, true, true, 404},
	} {
		res := request("POST", tc.path, `{"assetSymbol":"USD","quantity":"100"}`, tc.version, tc.auth, tc.csrf)
		if res.Code != tc.code {
			t.Fatalf("precondition want %d got %d %s", tc.code, res.Code, res.Body.String())
		}
	}
	added := request("POST", base, `{"assetSymbol":"usd","quantity":"100"}`, version, true, true)
	var state api.ManualBalanceState
	if added.Code != 200 || json.Unmarshal(added.Body.Bytes(), &state) != nil || !state.Pending || state.Items[0].Quantity != "100" {
		t.Fatalf("add %d %s", added.Code, added.Body.String())
	}
	next := strconv.FormatUint(state.Version, 10)
	ownerSession := session
	session = otherUserSession(t, authentication)
	for _, tc := range []struct{ method, suffix, body string }{
		{"GET", "", ""},
		{"POST", "", `{"assetSymbol":"BTC","quantity":"2"}`},
		{"PATCH", "/USD", `{"quantity":"-999"}`},
		{"DELETE", "/USD", ""},
	} {
		t.Run("foreign "+tc.method, func(t *testing.T) {
			res := request(tc.method, base+tc.suffix, tc.body, next, true, true)
			if res.Code != http.StatusNotFound {
				t.Fatalf("foreign access: %d %s", res.Code, res.Body.String())
			}
		})
	}
	session = ownerSession
	unchanged := request("GET", base, "", "", true, false)
	var after api.ManualBalanceState
	if unchanged.Code != 200 || json.Unmarshal(unchanged.Body.Bytes(), &after) != nil || !reflect.DeepEqual(state, after) || unchanged.Header().Get("ETag") != "" {
		t.Fatalf("rejected writes changed manual balances: %d %s", unchanged.Code, unchanged.Body.String())
	}
	if res := request("POST", base, `{"assetSymbol":"USD","quantity":"10"}`, next, true, true); res.Code != 409 {
		t.Fatalf("duplicate %d %s", res.Code, res.Body.String())
	}
	if res := request("PATCH", base+"/USD", `{"quantity":"-100"}`, version, true, true); res.Code != 412 {
		t.Fatalf("concurrent %d %s", res.Code, res.Body.String())
	}
	edited := request("PATCH", base+"/USD", `{"quantity":"-100"}`, next, true, true)
	if edited.Code != 200 {
		t.Fatalf("edit %d %s", edited.Code, edited.Body.String())
	}
	deleted := request("DELETE", base+"/USD", "", "3", true, true)
	if deleted.Code != 200 || json.Unmarshal(deleted.Body.Bytes(), &state) != nil || len(state.Items) != 0 || state.Pending {
		t.Fatalf("delete %d %s", deleted.Code, deleted.Body.String())
	}
	item, err = identities.PutCredentials(ctx, owned[0].ID, item.ID, item.Version, identity.ActionInput{ActorUserID: user.ID}, adapter.Credentials{APIKey: "invalid-http", APISecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	connection := "/api/v1/portfolios/" + owned[0].ID.String() + "/identities/" + item.ID.String() + "/connection"
	rejected := request("POST", connection+"/enable", "", strconv.FormatUint(item.Version, 10), true, true)
	var problem api.Problem
	if rejected.Code != http.StatusUnprocessableEntity || rejected.Header().Get("Content-Type") != "application/problem+json" || json.Unmarshal(rejected.Body.Bytes(), &problem) != nil || problem.Code != "connection_validation_failed" || problem.ValidationOutcome == nil || problem.ValidationOutcome.Ok || problem.ValidationOutcome.Identity.Connection.Status != api.ConnectionStatusInvalid {
		t.Fatalf("validation failure lost Problem Details or identity: %d %s", rejected.Code, rejected.Body.String())
	}
	validated := request("POST", connection+"/validate", "", "", true, true)
	var outcome api.ValidationOutcome
	if validated.Code != http.StatusOK || json.Unmarshal(validated.Body.Bytes(), &outcome) != nil || outcome.Ok {
		t.Fatalf("executed validation must remain a 200 outcome: %d %s", validated.Code, validated.Body.String())
	}
}
