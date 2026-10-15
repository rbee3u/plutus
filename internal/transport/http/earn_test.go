package httpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestEarnHTTPReadScopeAndUnknownDetails(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
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
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Services: &app.Services{Auth: authentication, Portfolios: portfolios, Financial: financial.NewService(db, valuation.NewService(db))}})
	request := func(path string, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	base := "/api/v1/portfolios/" + owned[0].ID.String()
	res := request(base+"/earn-positions", session.Token)
	var list api.EarnPositionList
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &list) != nil || list.DetailsComplete || len(list.Items) != 0 || len(list.Issues) == 0 || list.Page.Limit != 50 {
		t.Fatalf("no point: %d %s", res.Code, res.Body.String())
	}
	for _, suffix := range []string{"/earn-positions?offset=-1", "/earn-positions?limit=501", "/earn-positions?identityId=invalid"} {
		if res := request(base+suffix, session.Token); res.Code != 400 {
			t.Fatalf("invalid filter: %d %s", res.Code, res.Body.String())
		}
	}
	for _, suffix := range []string{"/earn-positions", "/accounts/" + uuid.NewString() + "/earn-positions"} {
		if res := request(base+suffix, ""); res.Code != 401 {
			t.Fatal("unauthenticated holdings read")
		}
	}
	foreign := otherUserSession(t, authentication)
	if res := request(base+"/earn-positions", foreign.Token); res.Code != 404 {
		t.Fatalf("another user read an existing portfolio: %d %s", res.Code, res.Body.String())
	}
	if res := request(base+"/accounts/"+uuid.NewString()+"/earn-positions", session.Token); res.Code != 404 {
		t.Fatal("unknown account read")
	}
}

// Typed earn details retain explicit source symbols across the API boundary.
func TestEarnStoredSymbolsBecomeExplicitAPIReferences(t *testing.T) {
	var details adapters.EarnDetails
	raw := []byte(`{"assets":[{"assetSymbol":"XBT","canonicalAssetSymbol":"BTC","amount":"1"}],"rates":[{"assetSymbol":"XBT","value":"0.02","kind":"apr","qualifier":"estimated","label":"APR"}],"rewards":[{"assetSymbol":"XBT","amount":"0.1","kind":"cumulative"}]}`)
	if err := json.Unmarshal(raw, &details); err != nil {
		t.Fatal(err)
	}
	stored := adapters.EarnPosition{Assets: details.Assets, Rates: details.Rates, Rewards: details.Rewards}
	rows := toAPIEarnPositions([]financial.EarnPositionRow{{EarnPosition: stored}})
	if rows[0].Assets[0].AssetSymbol != "XBT" || rows[0].Assets[0].CanonicalAssetSymbol == nil || *rows[0].Assets[0].CanonicalAssetSymbol != "BTC" || rows[0].Rates[0].AssetSymbol == nil || *rows[0].Rates[0].AssetSymbol != "XBT" || rows[0].Rewards[0].AssetSymbol != "XBT" {
		t.Fatalf("lost stored symbols: %+v", rows[0])
	}
	encoded, err := json.Marshal(rows)
	if err != nil || bytes.Contains(encoded, []byte(`"symbol"`)) || bytes.Count(encoded, []byte(`"assetSymbol":"XBT"`)) != 3 {
		t.Fatalf("API symbols: %s (%v)", encoded, err)
	}
	encoded, err = json.Marshal(details)
	if err != nil || !bytes.Contains(encoded, []byte(`"canonicalAssetSymbol":"BTC"`)) || bytes.Count(encoded, []byte(`"assetSymbol":"XBT"`)) != 3 {
		t.Fatalf("stored symbols: %s (%v)", encoded, err)
	}
}
