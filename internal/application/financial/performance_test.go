package financial

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/capital"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func performancePortfolio(tb testing.TB, db *sql.DB) PortfolioContext {
	tb.Helper()
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(tb.Context(), "perf-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		tb.Fatal(err)
	}
	owned, err := portfolio.NewService(db).List(tb.Context(), user.ID)
	if err != nil || len(owned) != 1 {
		tb.Fatalf("portfolios: %+v %v", owned, err)
	}
	return PortfolioContext{ID: owned[0].ID, Timezone: "UTC"}
}

// BenchmarkHistoryQueries is opt-in: ten years of 30-minute summaries should
// not make ordinary regression tests allocate a production-sized timeline.
func BenchmarkHistoryQueries(b *testing.B) {
	db, stats := mysqltest.ObserveQueries(b)
	p := performancePortfolio(b, db)
	start := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(10, 0, 0)
	historySummaries(b, db, p, start, end)
	service := NewService(db, valuation.NewService(db))
	for _, zone := range []struct{ name, timezone string }{{"utc", "UTC"}, {"shanghai", "Asia/Shanghai"}, {"new_york", "America/New_York"}} {
		b.Run(zone.name, func(b *testing.B) {
			p := p
			p.Timezone = zone.timezone
			for _, window := range []struct {
				name, kind string
				from       time.Time
			}{{"week", PeriodDay, end.AddDate(0, 0, -7)}, {"year", PeriodMonth, end.AddDate(-1, 0, 0)}, {"ten_years", PeriodMonth, start}} {
				b.Run(window.name, func(b *testing.B) {
					for _, operation := range []string{"changes", "closes", "series", "series_hour", "range"} {
						b.Run(operation, func(b *testing.B) {
							b.ReportAllocs()
							stats.Queries.Store(0)
							stats.Rows.Store(0)
							b.ResetTimer()
							for b.Loop() {
								var err error
								switch operation {
								case "changes":
									_, err = service.ValueChanges(b.Context(), p, window.kind, window.from, end)
								case "range":
									_, err = service.CustomValueChange(b.Context(), p, window.from, end)
								case "closes":
									_, err = service.PeriodCloses(b.Context(), p, window.kind, window.from, end)
								case "series_hour":
									_, err = service.Points(b.Context(), p, window.from, end, "hour")
									if window.name == "ten_years" && errors.Is(err, ErrSeriesLimit) {
										err = nil
									}
								case "series":
									_, err = service.Points(b.Context(), p, window.from, end, "day")
								}
								if err != nil {
									b.Fatal(err)
								}
							}
							b.ReportMetric(float64(stats.Queries.Load())/float64(b.N), "queries/op")
							b.ReportMetric(float64(stats.Rows.Load())/float64(b.N), "rows/op")
						})
					}
				})
			}
		})
	}

	rows, err := db.QueryContext(b.Context(), "EXPLAIN ANALYZE SELECT cycle_id,evaluated_at,calculation_version FROM valuation_points FORCE INDEX(idx_valuation_points_timeline) WHERE portfolio_id=? AND evaluated_at>=? AND evaluated_at<=? ORDER BY evaluated_at,cycle_id", p.ID[:], end.AddDate(0, 0, -7), end)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var plan string
		if err := rows.Scan(&plan); err != nil {
			b.Fatal(err)
		}
		b.Log(fmt.Sprintf("week timeline plan: %s", plan))
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
}

func historySummaries(tb testing.TB, db *sql.DB, p PortfolioContext, start, end time.Time) {
	tb.Helper()
	for at := start; at.Before(end); {
		var cycles, points []string
		var cycleArgs, pointArgs []any
		for i := 0; i < 500 && at.Before(end); i++ {
			id := uuid.New()
			cycles = append(cycles, "(?,?,'succeeded','schedule',?,?,?,?)")
			cycleArgs = append(cycleArgs, p.ID[:], id[:], at, at, at, at)
			points = append(points, "(?,?,?,100,0,0,0,1,'expired')")
			pointArgs = append(pointArgs, p.ID[:], id[:], at)
			at = at.Add(30 * time.Minute)
		}
		for _, statement := range []struct {
			query string
			args  []any
		}{
			{"INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES " + strings.Join(cycles, ","), cycleArgs},
			{"INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES " + strings.Join(points, ","), pointArgs},
		} {
			if _, err := db.ExecContext(tb.Context(), statement.query, statement.args...); err != nil {
				tb.Fatal(err)
			}
		}
	}
}

func TestCalculatedAssetChangeRangeScan(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	p := performancePortfolio(t, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5000 * 30 * time.Minute)
	historySummaries(t, db, p, start, end)
	var ids [][]byte
	for i := range 3 {
		asset := uuid.New()
		symbol := fmt.Sprintf("ASSET%d", i)
		if _, err := db.ExecContext(t.Context(), "INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance',?,'USD',1,?,?)", p.ID[:], asset[:], symbol, start, start); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(t.Context(), "INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) SELECT portfolio_id,?,evaluated_at,cycle_id,IF(MOD(TIMESTAMPDIFF(MINUTE,?,evaluated_at),60)=0,'included','absent'),IF(MOD(TIMESTAMPDIFF(MINUTE,?,evaluated_at),60)=0,'USD',NULL) FROM valuation_points WHERE portfolio_id=?", asset[:], start, start, p.ID[:]); err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			ids = append(ids, asset[:])
		}
	}
	rows, err := sqlcgen.New(db).ListCalculatedAssetChangesBetween(t.Context(), sqlcgen.ListCalculatedAssetChangesBetweenParams{PortfolioID: p.ID[:], AssetIds: ids, FromAt: end.Add(-time.Hour), ToAt: end})
	if err != nil || len(rows) != 4 {
		t.Fatalf("bounded changes: %+v %v", rows, err)
	}
	for _, row := range rows {
		if (string(row.AssetID) != string(ids[0]) && string(row.AssetID) != string(ids[1])) || row.EvaluatedAt.Before(end.Add(-time.Hour)) {
			t.Fatalf("asset/time scope changed: %+v", row)
		}
	}
	if _, err := db.ExecContext(t.Context(), "ANALYZE TABLE assets, calculated_asset_changes"); err != nil {
		t.Fatal(err)
	}
	plan := explainRead(t, db, "asset_exclusions.sql", "ListCalculatedAssetChangesBetween", map[string]any{"portfolio_id": p.ID[:], "asset_ids": ids, "from_at": end.Add(-time.Hour), "to_at": end})
	if !strings.Contains(plan, "Index range scan on c using PRIMARY") || !strings.Contains(plan, "rows=4 loops=1") {
		t.Fatalf("asset/time predicates did not bound the scan:\n%s", plan)
	}
}

func TestComparisonQueryBudget(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	historySummaries(t, db, p, start, start.AddDate(0, 0, 32))
	service := NewService(db, valuation.NewService(db))
	for _, days := range []int{7, 30} {
		stats.Queries.Store(0)
		changes, err := service.ValueChanges(t.Context(), p, PeriodDay, start.AddDate(0, 0, 1), start.AddDate(0, 0, days+1))
		if err != nil || len(changes) != days {
			t.Fatalf("%d periods: %d %v", days, len(changes), err)
		}
		if queries := stats.Queries.Load(); queries > 16 {
			t.Fatalf("%d periods amplified reads: %d queries", days, queries)
		}
		for _, change := range changes {
			if !change.Comparable || change.AmountUSD == nil || *change.AmountUSD != "0" {
				t.Fatalf("known zero comparison changed: %+v", change)
			}
		}
	}
}

func TestOverviewAndDetailQueryBudgets(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cycle := uuid.New()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", p.ID[:], cycle[:], at, at, at, at)
	exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,0,40,0,0,1)", p.ID[:], cycle[:], at)
	for i := range 40 {
		identity, account, report := uuid.New(), uuid.New(), uuid.New()
		name := fmt.Sprintf("Empty %02d", i)
		exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance',?,?,'classic','unconfigured',1,?,?)", p.ID[:], identity[:], name, name, at, at)
		exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,'spot',TRUE)", p.ID[:], account[:], identity[:])
		exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at) VALUES(?,?,?,?,'succeeded',?,?)", p.ID[:], report[:], cycle[:], account[:], at, at)
		exec("INSERT INTO account_valuations(portfolio_id,cycle_id,account_id,source_report_id,valuation_method,value_usd) VALUES(?,?,?,?,'calculated',0)", p.ID[:], cycle[:], account[:], report[:])
	}
	service := NewService(db, valuation.NewService(db))
	stats.Queries.Store(0)
	overview, err := service.Overview(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if queries := stats.Queries.Load(); queries > 12 {
		t.Fatalf("empty accounts amplified reads: %d queries", queries)
	}
	for _, platform := range overview.Platforms {
		for _, identity := range platform.Identities {
			if len(identity.Accounts) != 0 {
				t.Fatalf("confirmed empty account became visible: %+v", identity)
			}
		}
	}
	stats.Queries.Store(0)
	detail, err := service.Detail(t.Context(), p, cycle)
	if err != nil || len(detail.Accounts) != 40 {
		t.Fatalf("detail: %d accounts, %v", len(detail.Accounts), err)
	}
	if queries := stats.Queries.Load(); queries > 8 {
		t.Fatalf("account evidence reread: %d queries", queries)
	}
}

func BenchmarkRevisionQueries(b *testing.B) {
	db, stats := mysqltest.ObserveQueries(b)
	p := performancePortfolio(b, db)
	identity, account, asset := uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	exec := func(query string, args ...any) {
		b.Helper()
		if _, err := db.ExecContext(b.Context(), query, args...); err != nil {
			b.Fatal(err)
		}
	}
	exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance','Manual','manual','classic','unconfigured',1,?,?)", p.ID[:], identity[:], at, at)
	exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,'manual',TRUE)", p.ID[:], account[:], identity[:])
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", p.ID[:], asset[:], at, at)
	for start := 1; start <= 10000; start += 500 {
		var revisions, balances, policies []string
		var revisionArgs, balanceArgs, policyArgs []any
		for i := start; i < start+500; i++ {
			revisions = append(revisions, "(?,?,?,?)")
			var changed any
			if i > 1 {
				changed = at
			}
			revisionArgs = append(revisionArgs, p.ID[:], account[:], i, changed)
			if i > 1 {
				balances = append(balances, "(?,?,?,?,?,?)")
				balanceArgs = append(balanceArgs, p.ID[:], account[:], i, asset[:], i, at)
			}
			policies = append(policies, "(?,?,?,?,?,?)")
			var reason any
			excluded := i%2 == 0
			if excluded {
				reason = "Reviewed"
			}
			policyArgs = append(policyArgs, p.ID[:], asset[:], i, excluded, reason, at)
		}
		exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES "+strings.Join(revisions, ","), revisionArgs...)
		exec("INSERT INTO manual_revision_balances(portfolio_id,account_id,revision_number,asset_id,quantity,changed_at) VALUES "+strings.Join(balances, ","), balanceArgs...)
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES "+strings.Join(policies, ","), policyArgs...)
	}
	for range 200 {
		flow := uuid.New()
		exec("INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,account_id,version,created_at,updated_at) VALUES(?,?,'contribution','USD',1,1,'explicit',?,1,?,?,1,?,?)", p.ID[:], flow[:], at, at, account[:], at, at)
	}
	financial := NewService(db, valuation.NewService(db))
	assets := catalog.NewService(db)
	flows := capital.NewService(db)
	for _, operation := range []string{"manual", "catalog", "capital_page"} {
		b.Run(operation, func(b *testing.B) {
			stats.Queries.Store(0)
			stats.Rows.Store(0)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				switch operation {
				case "manual":
					rows, err := financial.currentManualBalances(b.Context(), p)
					if err != nil || len(rows[account]) != 1 || rows[account][0].Equity != "10000" || !rows[account][0].Excluded {
						b.Fatalf("manual state: %+v %v", rows, err)
					}
				case "catalog":
					page, err := assets.ListAssets(b.Context(), catalog.ListFilter{PortfolioID: p.ID, Status: "excluded", Limit: 50})
					if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ExclusionVersion != 10000 {
						b.Fatalf("catalog state: %+v %v", page, err)
					}
				case "capital_page":
					page, err := flows.ListPage(b.Context(), p.ID, at.Add(-time.Hour), at.Add(time.Hour), 0, 50)
					if err != nil || page.Total != 200 || len(page.Items) != 50 || page.Items[0].AccountName == nil || *page.Items[0].AccountName != "Manual / Manual" {
						b.Fatalf("capital page: %+v %v", page, err)
					}
				}
			}
			b.ReportMetric(float64(stats.Queries.Load())/float64(b.N), "queries/op")
			b.ReportMetric(float64(stats.Rows.Load())/float64(b.N), "rows/op")
		})
	}
	for _, query := range []struct{ file, name string }{{"manual.sql", "ListCurrentManualBalancesByAccounts"}, {"asset_exclusions.sql", "ListCurrentAssetExclusions"}} {
		explainRead(b, db, query.file, query.name, map[string]any{"portfolio_id": p.ID[:], "account_ids": account[:]})
	}
}

// Read the owned SQL so the same benchmark can measure an earlier checkout.
func explainRead(b testing.TB, db *sql.DB, file, name string, values map[string]any) string {
	b.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "queries", file))
	if err != nil {
		b.Fatal(err)
	}
	_, query, ok := strings.Cut(string(source), "-- name: "+name+" :")
	if !ok {
		b.Fatalf("query %s missing", name)
	}
	_, query, _ = strings.Cut(query, "\n")
	query, _, _ = strings.Cut(query, ";")
	query = regexp.MustCompile(`sqlc\.embed\(([a-z_]+)\)`).ReplaceAllString(query, "$1.*")
	if strings.Contains(query, "LIMIT ? OFFSET ?") {
		query = strings.ReplaceAll(query, "LIMIT ? OFFSET ?", fmt.Sprintf("LIMIT %d OFFSET %d", values["page_limit"], values["page_offset"]))
	}
	params := regexp.MustCompile(`sqlc\.(?:arg|slice)\('?([a-z_]+)'?\)`)
	var args []any
	query = params.ReplaceAllStringFunc(query, func(param string) string {
		value := values[params.FindStringSubmatch(param)[1]]
		if ids, ok := value.([][]byte); ok {
			for _, id := range ids {
				args = append(args, id)
			}
			return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		}
		args = append(args, value)
		return "?"
	})
	rows, err := db.QueryContext(b.Context(), "EXPLAIN ANALYZE "+query, args...)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plans []string
	for rows.Next() {
		var plan string
		if err := rows.Scan(&plan); err != nil {
			b.Fatal(err)
		}
		b.Logf("%s: %s", name, plan)
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
	return strings.Join(plans, "\n")
}

// Read-scope fixtures amplify unrelated current records and old policy versions.
// No benchmark depends on a live or pre-created database.
func readScopeFixture(tb testing.TB, db *sql.DB) (PortfolioContext, []uuid.UUID, []uuid.UUID, []valuation.Point) {
	tb.Helper()
	p := performancePortfolio(tb, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	exec := func(query string, args ...any) {
		tb.Helper()
		if _, err := db.ExecContext(tb.Context(), query, args...); err != nil {
			tb.Fatal(err)
		}
	}
	assets := make([]uuid.UUID, 2000)
	for offset := 0; offset < len(assets); offset += 100 {
		var assetRows, policyRows []string
		var assetArgs, policyArgs []any
		for i := offset; i < min(offset+100, len(assets)); i++ {
			assets[i] = uuid.New()
			symbol := fmt.Sprintf("ASSET%04d", i)
			assetRows = append(assetRows, "(?,?,'binance',?,'USD',1,?,?)")
			assetArgs = append(assetArgs, p.ID[:], assets[i][:], symbol, start, start)
			for version := 1; version <= 5; version++ {
				policyRows = append(policyRows, "(?,?,?,TRUE,'Reviewed',?)")
				policyArgs = append(policyArgs, p.ID[:], assets[i][:], version, start.Add(-time.Duration(6-version)*time.Hour))
			}
		}
		exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES "+strings.Join(assetRows, ","), assetArgs...)
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES "+strings.Join(policyRows, ","), policyArgs...)
	}
	var accounts, identities []uuid.UUID
	for i := range 100 {
		account, identity := uuid.New(), uuid.New()
		name := fmt.Sprintf("Manual %03d", i)
		accounts, identities = append(accounts, account), append(identities, identity)
		exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance',?,?,'binance_classic','unconfigured',1,?,?)", p.ID[:], identity[:], name, name, start, start)
		exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,'manual',TRUE)", p.ID[:], account[:], identity[:])
		exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES(?,?,1,NULL),(?,?,2,?)", p.ID[:], account[:], p.ID[:], account[:], start)
		if i != 99 {
			exec("INSERT INTO manual_revision_balances(portfolio_id,account_id,revision_number,asset_id,quantity,changed_at) VALUES(?,?,2,?,1,?)", p.ID[:], account[:], assets[i][:], start)
		}
	}
	historySummaries(tb, db, p, start, start.Add(2*time.Hour))
	headers, err := sqlcgen.New(db).ListValuationPointHeadersBetween(tb.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: start.Add(2 * time.Hour)})
	if err != nil {
		tb.Fatal(err)
	}
	var points []valuation.Point
	for _, h := range headers {
		id, err := uuid.FromBytes(h.CycleID)
		if err != nil {
			tb.Fatal(err)
		}
		points = append(points, valuation.Point{CycleID: id, EvaluatedAt: h.EvaluatedAt})
	}
	for _, account := range accounts {
		report := uuid.New()
		exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at,manual_revision_number) VALUES(?,?,?,?,'succeeded',?,?,2)", p.ID[:], report[:], points[0].CycleID[:], account[:], start, start)
	}
	return p, accounts, identities, points
}

func BenchmarkReadScopeQueries(b *testing.B) {
	db, stats := mysqltest.ObserveQueries(b)
	p, accounts, identities, points := readScopeFixture(b, db)
	service := NewService(db, valuation.NewService(db))
	catalogService := catalog.NewService(db)
	manualService := manual.NewService(db, catalogService)
	for _, operation := range []string{"policies", "catalog_all", "catalog_filtered", "manual_scoped", "manual_applied"} {
		b.Run(operation, func(b *testing.B) {
			b.ReportAllocs()
			stats.Queries.Store(0)
			stats.Rows.Store(0)
			b.ResetTimer()
			for b.Loop() {
				switch operation {
				case "policies":
					r := &comparisonRead{excluded: map[uuid.UUID]map[uuid.UUID]bool{}, assets: map[uuid.UUID]map[uuid.UUID]comparisonAsset{}}
					if err := service.comparisonPolicies(b.Context(), p, r, points); err != nil || len(r.excluded[points[0].CycleID]) != 2000 {
						b.Fatalf("policies: %d %v", len(r.excluded[points[0].CycleID]), err)
					}
				case "catalog_all", "catalog_filtered":
					status := ""
					if operation == "catalog_filtered" {
						status = "excluded"
					}
					page, err := catalogService.ListAssets(b.Context(), catalog.ListFilter{PortfolioID: p.ID, Status: status, Limit: 20, Offset: 20})
					if err != nil || len(page.Items) != 20 || page.Total != 2000 || !page.Items[0].Excluded {
						b.Fatalf("catalog: %+v %v", page, err)
					}
				case "manual_scoped":
					rows, overlay, err := service.manualOverlayBalances(b.Context(), p, map[uuid.UUID]bool{accounts[0]: true})
					if err != nil || len(rows[accounts[0]]) != 1 || len(overlay.Versions) != 1 || !rows[accounts[0]][0].Excluded {
						b.Fatalf("manual: %+v %v", rows, err)
					}
				case "manual_applied":
					state, err := manualService.Get(b.Context(), p.ID, identities[0])
					if err != nil || state.Pending || len(state.AppliedItems) != 1 {
						b.Fatalf("applied ledger: %+v %v", state, err)
					}
				}
			}
			b.ReportMetric(float64(stats.Queries.Load())/float64(b.N), "queries/op")
			b.ReportMetric(float64(stats.Rows.Load())/float64(b.N), "rows/op")
		})
	}
}

func TestManualOverlayReadScope(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p, accounts, _, points := readScopeFixture(t, db)
	service := NewService(db, valuation.NewService(db))
	for _, index := range []int{0, 99} {
		stats.Queries.Store(0)
		stats.Rows.Store(0)
		rows, overlay, err := service.manualOverlayBalances(t.Context(), p, map[uuid.UUID]bool{accounts[index]: true})
		if err != nil || len(overlay.Versions) != 1 || overlay.Versions[0].AccountID != accounts[index] || overlay.Versions[0].Version != 2 {
			t.Fatalf("scoped version: %+v %v", overlay, err)
		}
		if index == 0 && (len(rows[accounts[index]]) != 1 || !rows[accounts[index]][0].Excluded || stats.Queries.Load() != 3 || stats.Rows.Load() != 3) {
			t.Fatalf("scoped rows: %+v reads=%d rows=%d", rows, stats.Queries.Load(), stats.Rows.Load())
		}
		if index == 99 && (len(rows) != 0 || stats.Queries.Load() != 2 || stats.Rows.Load() != 1) {
			t.Fatalf("empty ledger: %+v reads=%d rows=%d", rows, stats.Queries.Load(), stats.Rows.Load())
		}
	}
	other := performancePortfolio(t, db)
	for _, selected := range []map[uuid.UUID]bool{{}, {accounts[0]: true}} {
		rows, overlay, err := service.manualOverlayBalances(t.Context(), other, selected)
		if err != nil || len(rows) != 0 || len(overlay.Versions) != 0 {
			t.Fatalf("foreign or empty scope: %+v %+v %v", rows, overlay, err)
		}
	}
	rows, overlay, err := service.manualOverlayBalances(t.Context(), p, nil)
	if err != nil || len(rows) != 99 || len(overlay.Versions) != 100 {
		t.Fatalf("portfolio scope: rows=%d versions=%+v error=%v", len(rows), overlay, err)
	}
	explainRead(t, db, "asset_exclusions.sql", "ListAssetExclusionsForWindow", map[string]any{
		"portfolio_id": p.ID[:], "from_at": points[0].EvaluatedAt, "to_at": points[len(points)-1].EvaluatedAt,
	})
	explainRead(t, db, "asset_exclusions.sql", "ListCurrentAssetExclusionsByAssets", map[string]any{
		"portfolio_id": p.ID[:], "asset_ids": [][]byte{rows[accounts[0]][0].AssetID[:]},
	})
	params := map[string]any{
		"portfolio_id": p.ID[:], "filter_platform": 0, "platform": "", "filter_query": 0, "query": "",
		"platform_order": ordering.PlatformsCSV(), "page_limit": 20, "page_offset": 20,
		"filter_status": 0, "status": "",
	}
	explainRead(t, db, "catalog.sql", "ListAssetsWithoutStatus", params)
	explainRead(t, db, "catalog.sql", "ListAssets", params)
	explainRead(t, db, "catalog.sql", "CountAssets", params)
	explainRead(t, db, "catalog.sql", "CountAssetsWithoutStatus", params)
}

func BenchmarkPolicyVersionQueries(b *testing.B) {
	db, stats := mysqltest.ObserveQueries(b)
	p := performancePortfolio(b, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	asset := uuid.New()
	if _, err := db.ExecContext(b.Context(), "INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", p.ID[:], asset[:], start, start); err != nil {
		b.Fatal(err)
	}
	for offset := 0; offset < 10000; offset += 500 {
		var rows []string
		var args []any
		for i := offset; i < offset+500; i++ {
			rows = append(rows, "(?,?,?,TRUE,'Reviewed',?)")
			args = append(args, p.ID[:], asset[:], i+1, start.Add(time.Duration(i)*time.Minute))
		}
		if _, err := db.ExecContext(b.Context(), "INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES "+strings.Join(rows, ","), args...); err != nil { // #nosec G202 -- Only fixed placeholders are concatenated; fixture values are bound.
			b.Fatal(err)
		}
	}
	service := NewService(db, valuation.NewService(db))
	for _, scenario := range []struct {
		name string
		from time.Time
	}{{"recent", start.Add(10001 * time.Minute)}, {"early", start.Add(time.Minute)}} {
		historySummaries(b, db, p, scenario.from, scenario.from.Add(2*time.Hour))
		headers, err := sqlcgen.New(db).ListValuationPointHeadersBetween(b.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: scenario.from, ToAt: scenario.from.Add(2 * time.Hour)})
		if err != nil {
			b.Fatal(err)
		}
		var points []valuation.Point
		for _, h := range headers {
			id, err := uuid.FromBytes(h.CycleID)
			if err != nil {
				b.Fatal(err)
			}
			points = append(points, valuation.Point{CycleID: id, EvaluatedAt: h.EvaluatedAt})
		}
		b.Run(scenario.name, func(b *testing.B) {
			stats.Queries.Store(0)
			stats.Rows.Store(0)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				r := &comparisonRead{excluded: map[uuid.UUID]map[uuid.UUID]bool{}, assets: map[uuid.UUID]map[uuid.UUID]comparisonAsset{}}
				if err := service.comparisonPolicies(b.Context(), p, r, points); err != nil || !r.excluded[points[0].CycleID][asset] {
					b.Fatalf("policy state: %+v %v", r.excluded, err)
				}
			}
			b.ReportMetric(float64(stats.Queries.Load())/float64(b.N), "queries/op")
			b.ReportMetric(float64(stats.Rows.Load())/float64(b.N), "rows/op")
		})
	}
	for _, from := range []time.Time{start.Add(time.Minute), start.Add(10001 * time.Minute)} {
		// The baseline checkout lacks the window query; retain portable benchmarks.
		if source, err := os.ReadFile("../../../db/queries/asset_exclusions.sql"); err == nil && strings.Contains(string(source), "-- name: ListAssetExclusionsForWindow") {
			explainRead(b, db, "asset_exclusions.sql", "ListAssetExclusionsForWindow", map[string]any{"portfolio_id": p.ID[:], "from_at": from, "to_at": from.Add(90 * time.Minute)})
		}
	}
}

func TestPointPredecessorSeeks(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5000 * 30 * time.Minute)
	historySummaries(t, db, p, start, end)
	headers, err := sqlcgen.New(db).ListValuationPointHeadersBetween(t.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: end})
	if err != nil {
		t.Fatal(err)
	}
	var selected []sqlcgen.ValuationPoint
	for _, index := range []int{0, 1, 20, len(headers) - 1} {
		h := headers[index]
		selected = append(selected, sqlcgen.ValuationPoint{PortfolioID: p.ID[:], CycleID: h.CycleID, EvaluatedAt: h.EvaluatedAt, CalculationVersion: h.CalculationVersion, DetailState: "expired", TotalUsd: "100"})
	}
	stats.Queries.Store(0)
	stats.Rows.Store(0)
	points, err := valuation.NewService(db).PointsWithEvents(t.Context(), selected)
	if err != nil || len(points) != len(selected) {
		t.Fatalf("points: %+v %v", points, err)
	}
	if stats.Queries.Load() != 4 || stats.Rows.Load() != 3 {
		t.Fatalf("sparse predecessor reads: queries=%d rows=%d", stats.Queries.Load(), stats.Rows.Load())
	}
}

func comparisonEventFixture(tb testing.TB, db *sql.DB, dense bool) (PortfolioContext, time.Time, time.Time) {
	tb.Helper()
	p := performancePortfolio(tb, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	historySummaries(tb, db, p, start, end)
	q := sqlcgen.New(db)
	headers, err := q.ListValuationPointHeadersBetween(tb.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: end})
	if err != nil {
		tb.Fatal(err)
	}
	exec := func(query string, args ...any) {
		tb.Helper()
		if _, err := db.ExecContext(tb.Context(), query, args...); err != nil {
			tb.Fatal(err)
		}
	}
	identity, account, asset := uuid.New(), uuid.New(), uuid.New()
	exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance','Comparison','comparison','binance_classic','unconfigured',1,?,?)", p.ID[:], identity[:], start, start)
	exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,'manual',TRUE)", p.ID[:], account[:], identity[:])
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','BTC','USD',2,?,?)", p.ID[:], asset[:], start, start)
	for _, index := range []int{1, len(headers) - 2} {
		h := headers[index]
		exec("INSERT INTO boundary_events(portfolio_id,cycle_id,account_id,direction,valuation_method,value_usd,valuation_complete) VALUES(?,?,?,'contribution','calculated',10,TRUE)", p.ID[:], h.CycleID, account[:])
		exec("INSERT INTO manual_balance_changes(portfolio_id,cycle_id,account_id) VALUES(?,?,?)", p.ID[:], h.CycleID, account[:])
		if !dense {
			exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) VALUES(?,?,?,?,'included','USD')", p.ID[:], asset[:], h.EvaluatedAt, h.CycleID)
		}
	}
	if dense {
		exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) SELECT portfolio_id,?,evaluated_at,cycle_id,IF(MOD(TIMESTAMPDIFF(MINUTE,?,evaluated_at),60)=0,'included','absent'),IF(MOD(TIMESTAMPDIFF(MINUTE,?,evaluated_at),60)=0,'USD',NULL) FROM valuation_points WHERE portfolio_id=?", asset[:], start, start, p.ID[:])
	}
	return p, start, end
}

func TestComparisonEventWindowPlans(t *testing.T) {
	for _, dense := range []bool{false, true} {
		t.Run(fmt.Sprint(dense), func(t *testing.T) {
			db, stats := mysqltest.ObserveQueries(t)
			p, start, end := comparisonEventFixture(t, db, dense)
			for _, analyze := range []bool{false, true} {
				if analyze {
					if _, err := db.ExecContext(t.Context(), "ANALYZE TABLE assets,calculated_asset_changes,valuation_points,manual_balance_changes,boundary_events"); err != nil {
						t.Fatal(err)
					}
				}
				params := map[string]any{"portfolio_id": p.ID[:], "from_at": end.Add(-time.Hour), "to_at": end}
				plan := explainRead(t, db, "valuation.sql", "ListComparisonPointHeadersBetween", params)
				if dense && strings.Contains(plan, "rows=17520 loops=") {
					t.Fatalf("short window scanned old asset history:\n%s", plan)
				}
				params["from_at"] = start
				plan = explainRead(t, db, "valuation.sql", "ListComparisonPointHeadersBetween", params)
				if !dense && strings.Contains(plan, "rows=17520 loops=") {
					t.Fatalf("long sparse window scanned all points:\n%s", plan)
				}
			}
			// Check both sides of the bounded-probe threshold against timeline facts,
			// including inclusive cutoffs, event deduplication, and empty windows.
			headers, err := sqlcgen.New(db).ListValuationPointHeadersBetween(t.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: end})
			if err != nil {
				t.Fatal(err)
			}
			for _, count := range []int{0, 1, 2, 1000, 1001, len(headers)} {
				from := end.Add(-time.Duration(count) * 30 * time.Minute)
				rows, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(t.Context(), sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: from, ToAt: end})
				if err != nil {
					t.Fatal(err)
				}
				expected := make([]sqlcgen.ListComparisonPointHeadersBetweenRow, 0)
				for i, h := range headers {
					if !h.EvaluatedAt.Before(from) && (i == 1 || i == len(headers)-2 || (dense && i%2 == 0)) {
						expected = append(expected, sqlcgen.ListComparisonPointHeadersBetweenRow{CycleID: h.CycleID, EvaluatedAt: h.EvaluatedAt})
					}
				}
				if !reflect.DeepEqual(rows, expected) {
					t.Fatalf("window %d: got %d events, want %d", count, len(rows), len(expected))
				}
			}
			rows, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(t.Context(), sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: end.Add(-time.Hour), ToAt: end})
			if err != nil || len(rows) != 1 {
				t.Fatalf("deduplicated events: %+v %v", rows, err)
			}
			foreign, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(t.Context(), sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: make([]byte, 16), FromAt: start, ToAt: end})
			if err != nil || len(foreign) != 0 {
				t.Fatalf("foreign events: %+v %v", foreign, err)
			}
			if !dense {
				stats.Queries.Store(0)
				stats.Rows.Store(0)
				change, err := NewService(db, valuation.NewService(db)).CustomValueChange(t.Context(), p, start, end)
				if err != nil || len(change.BoundaryEvents) != 2 || change.AmountUSD == nil || *change.AmountUSD != "-20" {
					t.Fatalf("custom comparison: %+v %v", change, err)
				}
				if stats.Queries.Load() != 19 || stats.Rows.Load() != 18 {
					t.Fatalf("custom reads: queries=%d rows=%d", stats.Queries.Load(), stats.Rows.Load())
				}
			}
		})
	}
}

func BenchmarkComparisonEventQueries(b *testing.B) {
	for _, dense := range []bool{false, true} {
		db, stats := mysqltest.ObserveQueries(b)
		p, start, end := comparisonEventFixture(b, db, dense)
		for _, operation := range []string{"hour", "year", "custom"} {
			if dense && operation == "custom" {
				continue
			}
			b.Run(fmt.Sprintf("dense_%t/%s", dense, operation), func(b *testing.B) {
				service := NewService(db, valuation.NewService(db))
				stats.Queries.Store(0)
				stats.Rows.Store(0)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if operation == "custom" {
						change, err := service.CustomValueChange(b.Context(), p, start, end)
						if err != nil || len(change.BoundaryEvents) != 2 || change.AmountUSD == nil || *change.AmountUSD != "-20" {
							b.Fatalf("custom comparison: %+v %v", change, err)
						}
					} else {
						from := start
						if operation == "hour" {
							from = end.Add(-time.Hour)
						}
						rows, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(b.Context(), sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: from, ToAt: end})
						if err != nil || len(rows) == 0 {
							b.Fatalf("comparison headers: %+v %v", rows, err)
						}
					}
				}
				b.ReportMetric(float64(stats.Queries.Load())/float64(b.N), "queries/op")
				b.ReportMetric(float64(stats.Rows.Load())/float64(b.N), "rows/op")
			})
		}
	}
}
