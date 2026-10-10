package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestHistoryHTTPPaginationAndSeries(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "history-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("%v %v", owned, err)
	}
	p := owned[0]
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "history test")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for i := 0; i < 561; i++ {
		id, job := uuid.New(), uuid.New()
		at := start.Add(time.Duration(i) * 15 * time.Minute)
		if _, err := tx.ExecContext(ctx, "INSERT INTO jobs (id,portfolio_id,kind,status,`trigger`,dedupe_key,available_at,created_at,updated_at) VALUES (?,?,'collection_cycle','succeeded','manual','cycle',?,?,?)", job[:], p.ID[:], at, at, at); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO valuation_points (id,portfolio_id,job_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,boundary_json,issues_json,created_at) VALUES (?,?,?,?,?,1,0,?,'[]','[]',?)", id[:], p.ID[:], job[:], at, i+1, i%2, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	service := valuation.NewService(db)
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Services: &app.Services{Auth: authentication, Portfolios: portfolios, Valuation: service, Financial: financial.NewService(db, service)}})
	request := func(path string, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		result := httptest.NewRecorder()
		engine.ServeHTTP(result, req)
		return result
	}
	base := "/api/v1/portfolios/" + p.ID.String()
	query := "?from=" + url.QueryEscape(start.Format(time.RFC3339)) + "&to=" + url.QueryEscape(start.Add(560*15*time.Minute).Format(time.RFC3339))
	for _, offset := range []int{0, 20, 560, 580} {
		response := request(base+"/valuation-points/page"+query+fmt.Sprintf("&offset=%d&limit=20", offset), session.Token)
		if response.Code != http.StatusOK {
			t.Fatalf("page: %d %s", response.Code, response.Body.String())
		}
		var page api.ValuationPointPage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		want := min(20, max(0, 561-offset))
		if len(page.Items) != want || page.Page.Total != 561 || page.Page.Offset != offset {
			t.Fatalf("page %+v", page.Page)
		}
		for i, item := range page.Items {
			n := 561 - offset - i
			if item.TotalUsd != fmt.Sprint(n) || !item.EvaluatedAt.Equal(start.Add(time.Duration(n-1)*15*time.Minute)) {
				t.Fatalf("point %+v", item)
			}
			wantComplete := api.Complete
			if (n-1)%2 == 1 {
				wantComplete = api.Incomplete
			}
			if item.Completeness != wantComplete {
				t.Fatalf("completeness %+v", item)
			}
		}
	}
	for _, kind := range []string{"raw", "hour", "day"} {
		response := request(base+"/valuation-points"+query+"&granularity="+kind, session.Token)
		if response.Code != http.StatusOK {
			t.Fatalf("series %d %s", response.Code, response.Body.String())
		}
		var series api.ValuationPointList
		if err := json.Unmarshal(response.Body.Bytes(), &series); err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"raw": 561, "hour": 141, "day": 7}[kind]
		if len(series.Items) != want || series.Items[len(series.Items)-1].TotalUsd != "561" {
			t.Fatalf("%s: %d", kind, len(series.Items))
		}
		if kind == "hour" && series.Items[0].TotalUsd != "4" {
			t.Fatal("hour does not retain its final point")
		}
	}
	window := request(base+"/history-window?fromLocal=2026-10-04T12:00&toLocal=2026-10-04T13:00", session.Token)
	var resolved api.HistoryWindow
	if window.Code != 200 || json.Unmarshal(window.Body.Bytes(), &resolved) != nil || resolved.From.Format(time.RFC3339) != "2026-10-04T04:00:00Z" {
		t.Fatalf("window: %d %s", window.Code, window.Body.String())
	}
	for _, suffix := range []string{"/valuation-points/page" + query + "&limit=100", "/valuation-points/page" + query + "&offset=-1", "/history-window?fromLocal=2026-10-04T12:00", "/history-window?preset=all", "/history-window?fromLocal=&toLocal="} {
		response := request(base+suffix, session.Token)
		if response.Code != 400 {
			t.Fatalf("invalid %s: %d %s", suffix, response.Code, response.Body.String())
		}
	}
	foreign := otherUserSession(t, authentication)
	for _, suffix := range []string{"/history-window", "/valuation-points/page"} {
		if response := request(base+suffix, ""); response.Code != 401 {
			t.Fatalf("unauthenticated %d", response.Code)
		}
		if response := request(base+suffix, foreign.Token); response.Code != 404 {
			t.Fatalf("unowned %d", response.Code)
		}
	}
}

func TestStoredOmissionUsesExplicitAPIReference(t *testing.T) {
	var omitted []valuation.Omitted
	if err := json.Unmarshal([]byte(`[{"symbol":"DEMO-BTC","assetSymbol":"BTC","equity":"2","reason":"missing_price"}]`), &omitted); err != nil {
		t.Fatal(err)
	}
	out := toAPIAccountValuation(&valuation.AccountValuation{Omitted: omitted})
	if len(out.Omitted) != 1 || out.Omitted[0].AssetSymbol != "DEMO-BTC" || out.Omitted[0].CanonicalAssetSymbol == nil || *out.Omitted[0].CanonicalAssetSymbol != "BTC" || out.Omitted[0].Equity != "2" {
		t.Fatalf("lost stored omission: %+v", out.Omitted)
	}
	encoded, err := json.Marshal(out.Omitted)
	if err != nil || bytes.Contains(encoded, []byte(`"symbol"`)) || !bytes.Contains(encoded, []byte(`"assetSymbol":"DEMO-BTC"`)) {
		t.Fatalf("API omission: %s (%v)", encoded, err)
	}
	encoded, err = json.Marshal(omitted)
	if err != nil || !bytes.Contains(encoded, []byte(`"symbol":"DEMO-BTC"`)) {
		t.Fatalf("stored omission: %s (%v)", encoded, err)
	}
}
