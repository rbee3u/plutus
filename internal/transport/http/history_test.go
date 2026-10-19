package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/capital"
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
	start := time.Date(2026, 9, 28, 0, 15, 0, 0, time.UTC)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for i := 0; i < 1101; i++ {
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
	query := "?from=" + url.QueryEscape(start.Format(time.RFC3339)) + "&to=" + url.QueryEscape(start.Add(1100*15*time.Minute).Format(time.RFC3339))
	for _, offset := range []int{0, 20, 1100, 1120} {
		response := request(base+"/valuation-points"+query+fmt.Sprintf("&offset=%d&limit=20", offset), session.Token)
		if response.Code != http.StatusOK {
			t.Fatalf("page: %d %s", response.Code, response.Body.String())
		}
		var page api.ValuationPointPage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		want := min(20, max(0, 1101-offset))
		if len(page.Items) != want || page.Page.Total != 1101 || page.Page.Offset != offset {
			t.Fatalf("page %+v", page.Page)
		}
		remaining := 1101 - offset - len(page.Items)
		if len(page.Items) > 0 && remaining > 0 {
			if page.PrecedingPoint == nil || page.PrecedingPoint.TotalUsd != fmt.Sprint(remaining) {
				t.Fatalf("page predecessor: %+v", page.PrecedingPoint)
			}
		} else if page.PrecedingPoint != nil {
			t.Fatal("invented page predecessor")
		}
		for i, item := range page.Items {
			n := 1101 - offset - i
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
		response := request(base+"/valuation-points/series"+query+"&granularity="+kind, session.Token)
		if response.Code != http.StatusOK {
			t.Fatalf("series %d %s", response.Code, response.Body.String())
		}
		var series api.ValuationPointList
		if err := json.Unmarshal(response.Body.Bytes(), &series); err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"raw": 1101, "hour": 276, "day": 12}[kind]
		if len(series.Items) != want || series.Items[len(series.Items)-1].TotalUsd != "1101" {
			t.Fatalf("%s: %d", kind, len(series.Items))
		}
		if kind == "hour" && series.Items[0].TotalUsd != "3" {
			t.Fatal("hour does not retain its final point")
		}
	}
	for _, tc := range []struct {
		kind           string
		budget, status int
	}{{"raw", 1000, 422}, {"hour", 276, 200}, {"hour", 275, 422}, {"day", 12, 200}, {"day", 11, 422}} {
		response := request(base+"/valuation-points/series"+query+fmt.Sprintf("&granularity=%s&maxPoints=%d", tc.kind, tc.budget), session.Token)
		if response.Code != tc.status {
			t.Fatalf("budget %s/%d: %d %s", tc.kind, tc.budget, response.Code, response.Body.String())
		}
		if tc.status == 422 {
			var problem api.Problem
			if json.Unmarshal(response.Body.Bytes(), &problem) != nil || problem.Code != "series_limit_exceeded" {
				t.Fatalf("series limit problem: %s", response.Body.String())
			}
		} else {
			var series api.ValuationPointList
			if json.Unmarshal(response.Body.Bytes(), &series) != nil || len(series.Items) != tc.budget || series.MaxPoints != tc.budget {
				t.Fatalf("bounded series: %s", response.Body.String())
			}
		}
	}
	window := request(base+"/history-window?fromLocal=2026-10-04T12:00&toLocal=2026-10-04T13:00", session.Token)
	var resolved api.HistoryWindow
	if window.Code != 200 || json.Unmarshal(window.Body.Bytes(), &resolved) != nil || resolved.From.Format(time.RFC3339) != "2026-10-04T04:00:00Z" {
		t.Fatalf("window: %d %s", window.Code, window.Body.String())
	}
	for _, suffix := range []string{"/valuation-points/series" + query + "&maxPoints=0", "/valuation-points/series" + query + "&maxPoints=10001", "/valuation-points" + query + "&limit=100", "/valuation-points" + query + "&offset=-1", "/history-window?fromLocal=2026-10-04T12:00", "/history-window?preset=all", "/history-window?fromLocal=&toLocal="} {
		response := request(base+suffix, session.Token)
		if response.Code != 400 {
			t.Fatalf("invalid %s: %d %s", suffix, response.Code, response.Body.String())
		}
	}
	foreign := otherUserSession(t, authentication)
	for _, suffix := range []string{"/history-window", "/valuation-points"} {
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

func TestRequestContractValidation(t *testing.T) {
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler)})
	base := "/api/v1/portfolios/00000000-0000-7000-8000-000000000001"
	for _, tc := range []struct{ name, method, path, body string }{
		{"required field", http.MethodPost, "/api/v1/portfolios", `{}`},
		{"unknown field", http.MethodPost, "/api/v1/portfolios", `{"name":"Portfolio","unexpected":"must-not-echo"}`},
		{"multiple JSON values", http.MethodPost, "/api/v1/portfolios", `{"name":"Portfolio"} {"unexpected":"must-not-echo"}`},
		{"trailing JSON data", http.MethodPatch, base, `{"name":"Portfolio"} must-not-echo`},
		{"nonnullable patch", http.MethodPatch, base, `{"name":null}`},
		{"length", http.MethodPost, "/api/v1/portfolios", `{"name":"` + strings.Repeat("x", 121) + `"}`},
		{"enum", http.MethodPost, base + "/capital-flows", `{"direction":"invalid","canonicalAssetSymbol":"BTC","quantity":"1","effectiveAt":"2026-10-01T00:00:00Z"}`},
		{"decimal", http.MethodPost, base + "/capital-flows", `{"direction":"contribution","canonicalAssetSymbol":"BTC","quantity":1,"effectiveAt":"2026-10-01T00:00:00Z"}`},
		{"query minimum", http.MethodGet, base + "/balances?offset=-1", ""},
		{"query enum", http.MethodGet, base + "/valuation-points/series?granularity=invalid", ""},
		{"uuid", http.MethodGet, base + "/accounts/invalid/report", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("If-Edit-Version", "1")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			var problem api.Problem
			if response.Code != 400 || json.Unmarshal(response.Body.Bytes(), &problem) != nil || problem.FieldErrors == nil || len(*problem.FieldErrors) == 0 {
				t.Fatalf("validation: %d %s", response.Code, response.Body.String())
			}
			if bytes.Contains(response.Body.Bytes(), []byte("must-not-echo")) {
				t.Fatal("validation disclosed an input value")
			}
		})
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, base+"/capital-flows/00000000-0000-7000-8000-000000000002", strings.NewReader(`{"note":null,"accountId":null,"valueUsd":null,"pricingPlatform":null}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Edit-Version", "1")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("nullable PATCH must pass validation: %d %s", response.Code, response.Body.String())
	}
}

func TestEditVersionSyntax(t *testing.T) {
	for _, value := range []string{"", "0", "01", "+1", "-1", " 1", "1 ", `"identity:1"`, "18446744073709551616"} {
		if _, ok := parseEditVersion(value, false); ok {
			t.Errorf("accepted invalid positive edit version %q", value)
		}
	}
	for _, value := range []string{"1", "27", "18446744073709551615"} {
		if _, ok := parseEditVersion(value, false); !ok {
			t.Errorf("rejected edit version %q", value)
		}
	}
	if version, ok := parseEditVersion("0", true); !ok || version != 0 {
		t.Fatal("initial exclusion version must accept zero")
	}
}

func TestPendingManualJSONHasNoCollectedFacts(t *testing.T) {
	rows := toAPIBalanceRows([]financial.BalanceRow{{Equity: "12.5", ManualQuantity: new("12.5"), ManualChange: "added"}})
	if len(rows) != 1 || rows[0].Equity != nil || rows[0].Freshness != nil || rows[0].ReportObservedAt != nil || rows[0].EquityUsd != nil || rows[0].ManualQuantity == nil || *rows[0].ManualQuantity != "12.5" {
		t.Fatalf("pending quantity became a collected fact: %+v", rows)
	}
}

func TestCapitalAndBoundaryHTTPPages(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	authentication := auth.NewService(db, time.Hour, 24*time.Hour)
	name := "timeline-" + uuid.NewString()[:8]
	user, err := authentication.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	owned, err := portfolios.List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %+v %v", owned, err)
	}
	p := owned[0]
	_, session, err := authentication.Login(ctx, name, "Correct=123456", "127.0.0.1", "timeline test")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	identityID, cycleID := uuid.New(), uuid.New()
	exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance','Timeline','timeline','binance_classic','unconfigured',1,?,?)", p.ID[:], identityID[:], at, at)
	exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','user',?,?,?,?)", p.ID[:], cycleID[:], at, at, at, at)
	exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,10,3,0,1,1)", p.ID[:], cycleID[:], at)
	accounts := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	flows := []uuid.UUID{}
	for i, kind := range []string{"spot", "funding", "manual"} {
		id := accounts[i]
		exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,?,TRUE)", p.ID[:], id[:], identityID[:], kind)
		value, method, complete := any("5"), "calculated", true
		if i == 2 {
			value, method, complete = nil, "missing", false
		}
		exec("INSERT INTO boundary_events(portfolio_id,cycle_id,account_id,direction,valuation_method,value_usd,valuation_complete) VALUES(?,?,?,'contribution',?,?,?)", p.ID[:], cycleID[:], id[:], method, value, complete)
		flow := uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", i+1))
		flows = append(flows, flow)
		var acc any
		if i == 0 {
			acc = id[:]
		}
		exec("INSERT INTO capital_flows(portfolio_id,id,account_id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,version,created_at,updated_at) VALUES(?,?,?,'contribution','USD',1,1,'explicit',?,1,?,1,?,?)", p.ID[:], flow[:], acc, at, at, at, at)
	}
	values := valuation.NewService(db)
	engine := Engine(&EngineOptions{Logger: slog.New(slog.DiscardHandler), Services: &app.Services{DB: db, Auth: authentication, Portfolios: portfolios, Capital: capital.NewService(db), Valuation: values, Financial: financial.NewService(db, values)}})
	base := "/api/v1/portfolios/" + p.ID.String()
	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	for i, expectedAccount := range append(accounts, uuid.Nil) {
		query := "?from=" + url.QueryEscape(at.Format(time.RFC3339)) + "&to=" + url.QueryEscape(at.Add(time.Hour).Format(time.RFC3339)) + fmt.Sprintf("&limit=1&offset=%d", i)
		res := get("/capital-flows" + query)
		var page api.CapitalFlowList
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &page) != nil || page.Page.Total != 3 || page.Page.Offset != i || len(page.Items) != min(1, 3-i) || !page.From.Equal(at) {
			t.Fatalf("flow page: %d %s", res.Code, res.Body.String())
		}
		if i < 3 && page.Items[0].Id != flows[2-i] {
			t.Fatal("equal-time flow order drifted")
		}
		if i == 2 {
			item := page.Items[0]
			if item.Platform == nil || *item.Platform != "binance" || item.IdentityName == nil || *item.IdentityName != "Timeline" || item.AccountName == nil || *item.AccountName != "Spot" {
				t.Fatalf("expected flow account details: %+v", item)
			}
		} else if i < 3 {
			item := page.Items[0]
			if item.Platform != nil || item.IdentityName != nil || item.AccountName != nil {
				t.Fatalf("expected unallocated flow without account details: %+v", item)
			}
		}
		res = get("/boundary-events" + query)
		var events api.BoundaryEventList
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &events) != nil || events.Page.Total != 3 || events.Page.Offset != i || len(events.Items) != min(1, 3-i) || !events.From.Equal(at) {
			t.Fatalf("boundary page: %d %s", res.Code, res.Body.String())
		}
		if i < 3 && events.Items[0].AccountId != expectedAccount {
			t.Fatal("account hierarchy changed across page boundaries")
		}
		if i == 2 && (events.Items[0].ValueUsd != nil || events.Items[0].Completeness != api.Incomplete) {
			t.Fatal("unknown boundary amount became zero or complete")
		}
	}
	for _, suffix := range []string{"/capital-flows?limit=201", "/boundary-events?offset=-1", "/capital-flows?offset=2147483648"} {
		if res := get(suffix); res.Code != 400 {
			t.Fatalf("invalid pagination: %d %s", res.Code, res.Body.String())
		}
	}
}
