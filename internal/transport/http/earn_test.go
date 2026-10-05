package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

func TestEarnHTTPReadScopeAndUnknownDetails(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "earn-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %v %v", owned, err)
	}
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "Earn test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Auth: authentication, Portfolios: portfolios, Valuation: valuation.NewService(db)})
	request := func(path string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if authenticated {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	base := "/api/v1/portfolios/" + owned[0].ID.String()
	res := request(base+"/earn-positions", true)
	var list api.EarnPositionList
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &list) != nil || list.DetailsComplete || len(list.Items) != 0 || len(list.Issues) == 0 || list.Page.Limit != 50 {
		t.Fatalf("no point: %d %s", res.Code, res.Body.String())
	}
	for _, suffix := range []string{"/earn-positions?offset=-1", "/earn-positions?limit=501", "/earn-positions?identityId=invalid"} {
		if res := request(base+suffix, true); res.Code != 400 {
			t.Fatalf("invalid filter: %d %s", res.Code, res.Body.String())
		}
	}
	for _, suffix := range []string{"/earn-positions", "/accounts/" + uuid.NewString() + "/earn-positions"} {
		if res := request(base+suffix, false); res.Code != 401 {
			t.Fatal("unauthenticated holdings read")
		}
		if res := request("/api/v1/portfolios/"+uuid.NewString()+suffix, true); res.Code != 404 {
			t.Fatal("foreign Portfolio read")
		}
	}
	if res := request(base+"/accounts/"+uuid.NewString()+"/earn-positions", true); res.Code != 404 {
		t.Fatal("unknown Account read")
	}
}
