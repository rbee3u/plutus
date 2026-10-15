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
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func TestIssueJSONPreservesSourceValues(t *testing.T) {
	observed := time.Date(2026, 10, 13, 2, 59, 2, 123456000, time.UTC)
	account := valuation.AccountValuation{
		AccountID: uuid.New(), Method: valuation.MethodCalculated,
		Freshness: valuation.FreshnessStale, ReportObservedAt: &observed,
		Omitted: []valuation.Omitted{
			{AssetSymbol: "DEMO", CanonicalAssetSymbol: "BTC", Equity: "1000.123456789", Reason: "no_price"},
			{AssetSymbol: "DUST", Equity: "0.000000000123456789", Reason: "no_price"},
		},
	}
	data, err := json.Marshal(toAPIIssues(valuation.AccountIssues(&account)))
	if err != nil {
		t.Fatal(err)
	}
	var issues []api.Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 || issues[0].ReportObservedAt == nil || !issues[0].ReportObservedAt.Equal(observed) {
		t.Fatalf("observation time lost: %s", data)
	}
	for i, want := range account.Omitted {
		got := issues[i+1].OmittedBalance
		if got == nil || got.Equity != want.Equity || got.AssetSymbol != want.AssetSymbol || got.Reason != want.Reason || issues[i+1].Code != want.Reason {
			t.Fatalf("omission precision or identity lost: %s", data)
		}
	}
	if got := issues[1].OmittedBalance.CanonicalAssetSymbol; got == nil || *got != "BTC" {
		t.Fatalf("mapping lost: %s", data)
	}
}

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
		cycle := uuid.New()
		at := start.Add(time.Duration(i) * 15 * time.Minute)
		if _, err := tx.ExecContext(ctx, "INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES (?,?,'succeeded','user',?,?,?,?)", p.ID[:], cycle[:], at, at, at, at); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO valuation_points (portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES (?,?,?,?,1,0,?,1)", p.ID[:], cycle[:], at, i+1, i%2); err != nil {
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

func TestCollectionCycleDetailWithoutRetainedHoldings(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "cycle-" + uuid.NewString()[:8]
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
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "cycle test")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	running, failed, expired := uuid.New(), uuid.New(), uuid.New()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,attempt_count,available_at,created_at,started_at,lease_owner,lease_expires_at) VALUES (?,?,'running','user',1,?,?,?,'test',?)", []any{p.ID[:], running[:], at, at, at, at.Add(time.Hour)}},
		{"INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,attempt_count,available_at,created_at,started_at,finished_at,error_class,error_detail) VALUES (?,?,'failed','schedule',3,?,?,?,?,'temporary','fixture outage')", []any{p.ID[:], failed[:], at, at, at, at}},
		{"INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,attempt_count,available_at,created_at,started_at,collection_finished_at,finished_at) VALUES (?,?,'succeeded','schedule',1,?,?,?,?,?)", []any{p.ID[:], expired[:], at, at, at, at, at}},
		{"INSERT INTO valuation_points (portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES (?,?,?,7,1,0,0,1,'expired')", []any{p.ID[:], expired[:], at}},
	} {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	service := valuation.NewService(db)
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Ready: StaticReady{}, Services: &app.Services{DB: db, Auth: authentication, Portfolios: portfolios, Valuation: service, Cycles: cycles.NewService(db)}})
	get := func(cycle uuid.UUID) api.CollectionCycleDetail {
		t.Helper()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/portfolios/"+p.ID.String()+"/collection-cycles/"+cycle.String(), nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		var detail api.CollectionCycleDetail
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &detail) != nil {
			t.Fatalf("cycle %s: %d %s", cycle, response.Code, response.Body.String())
		}
		return detail
	}
	if detail := get(running); detail.Cycle.Status != api.CollectionCycleStatusRunning || detail.Cycle.Point != nil || len(detail.Accounts) != 0 {
		t.Fatalf("running cycle: %+v", detail)
	}
	if detail := get(failed); detail.Cycle.AttemptCount != 3 || detail.Cycle.ErrorClass == nil || *detail.Cycle.ErrorClass != "temporary" || detail.Cycle.Point != nil {
		t.Fatalf("failed cycle: %+v", detail)
	}
	if detail := get(expired); detail.Cycle.Point == nil || detail.Cycle.Point.DetailState != api.Expired || len(detail.Accounts) != 0 {
		t.Fatalf("expired cycle: %+v", detail)
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/portfolios/"+p.ID.String()+"/collection-cycles", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	var list api.CollectionCycleList
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &list) != nil || len(list.Items) != 3 {
		t.Fatalf("cycle list: %d %s", response.Code, response.Body.String())
	}
	for _, item := range list.Items {
		if (item.Id == expired) != (item.Point != nil) || item.Point != nil && (item.Point.BoundaryContributionUsd != "0" || item.Point.BoundaryWithdrawalUsd != "0") {
			t.Fatalf("listed cycle point: %+v", item)
		}
	}
}

func TestStoredOmissionUsesExplicitAPIReference(t *testing.T) {
	var omitted []valuation.Omitted
	if err := json.Unmarshal([]byte(`[{"assetSymbol":"DEMO-BTC","canonicalAssetSymbol":"BTC","equity":"2","reason":"missing_price"}]`), &omitted); err != nil {
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
	if err != nil || !bytes.Contains(encoded, []byte(`"assetSymbol":"DEMO-BTC"`)) {
		t.Fatalf("stored omission: %s (%v)", encoded, err)
	}
}
