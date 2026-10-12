package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestCatalogHTTPStatusFilter(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	username := "catalog-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, username, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("owned Portfolios: %+v %v", owned, err)
	}
	portfolioID := owned[0].ID
	service := catalog.NewService(db)
	for _, symbol := range []string{"BTC", "ETH", "SOL"} {
		id, _, err := service.EnsureAsset(ctx, sqlcgen.New(db), portfolioID, adapters.PlatformBinance, symbol, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if symbol == "BTC" {
			continue
		}
		excluded, err := service.PutExclusion(ctx, id, &catalog.ExclusionInput{
			PortfolioID: portfolioID, ActorUserID: user.ID, Excluded: true, Reason: "Reviewed asset",
		})
		if err != nil {
			t.Fatal(err)
		}
		if symbol == "SOL" {
			if _, err := service.PutExclusion(ctx, id, &catalog.ExclusionInput{
				PortfolioID: portfolioID, ActorUserID: user.ID, Version: excluded.ExclusionVersion,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, session, err := authentication.Login(ctx, username, "Correct=123456", "127.0.0.1", "Catalog test")
	if err != nil {
		t.Fatal(err)
	}
	engine := Engine(&EngineOptions{
		Logger:   slog.New(slog.DiscardHandler),
		Services: &app.Services{Auth: authentication, Portfolios: portfolios, Catalog: service},
	})
	base := "/api/v1/portfolios/" + portfolioID.String() + "/catalog/assets"
	request := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, base+query, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	for _, tc := range []struct {
		query   string
		symbols []string
		total   int
	}{
		{"", []string{"BTC", "ETH", "SOL"}, 3},
		{"?status=all", []string{"BTC", "ETH", "SOL"}, 3},
		{"?status=active", []string{"BTC", "SOL"}, 2},
		{"?status=excluded", []string{"ETH"}, 1},
		{"?status=active&limit=1&offset=1", []string{"SOL"}, 2},
		{"?status=active&platform=binance&q=SOL", []string{"SOL"}, 1},
	} {
		t.Run(tc.query, func(t *testing.T) {
			response := request(tc.query)
			if response.Code != http.StatusOK {
				t.Fatalf("list: %d %s", response.Code, response.Body.String())
			}
			var page api.AssetList
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			symbols := make([]string, 0, len(page.Items))
			for _, item := range page.Items {
				symbols = append(symbols, item.Symbol)
				if item.Excluded != (item.Symbol == "ETH") {
					t.Fatalf("incorrect exclusion state: %+v", item)
				}
			}
			if !reflect.DeepEqual(symbols, tc.symbols) || page.Page.Total != tc.total {
				t.Fatalf("got symbols=%v total=%d; want symbols=%v total=%d", symbols, page.Page.Total, tc.symbols, tc.total)
			}
		})
	}
	for _, status := range []string{"", "bogus", "ACTIVE", " active "} {
		t.Run("invalid "+status, func(t *testing.T) {
			response := request("?status=" + url.QueryEscape(status))
			var problem api.Problem
			if response.Code != http.StatusBadRequest || json.Unmarshal(response.Body.Bytes(), &problem) != nil || problem.Code != "validation_failed" {
				t.Fatalf("invalid status: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
