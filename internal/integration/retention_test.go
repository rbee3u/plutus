package integration

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/retention"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestRetentionPreservesSummariesSourcesAndDailyFallback(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("archive", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	account := item.Accounts[0].ID
	for _, a := range item.Accounts {
		if a.AccountType == "spot" {
			account = a.ID
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.db.ExecContext(h.ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:])
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	asset := uuid.New()
	exec("INSERT INTO assets(id,portfolio_id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", asset[:], h.portfolioID[:], at, at)
	times := []time.Time{at, at.Add(11 * time.Hour), at.Add(24 * time.Hour), at.Add(35 * time.Hour), at.AddDate(0, 0, 49)}
	points := []valuation.Point{}
	reports := []uuid.UUID{}
	for i, when := range times {
		cycle, report := uuid.New(), uuid.New()
		reports = append(reports, report)
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','user',?,?,?,?)", h.portfolioID[:], cycle[:], when, when, when, when)
		if i == 1 || i == 4 {
			exec("INSERT INTO account_reports(id,portfolio_id,cycle_id,account_id,status,error_class,recorded_at) VALUES(?,?,?,?,'failed','temporary',?)", report[:], h.portfolioID[:], cycle[:], account[:], when)
		} else {
			exec("INSERT INTO account_reports(id,portfolio_id,cycle_id,account_id,status,collected_at,recorded_at) VALUES(?,?,?,?,'succeeded',?,?)", report[:], h.portfolioID[:], cycle[:], account[:], when, when)
			exec("INSERT INTO balances(report_id,portfolio_id,asset_id,equity) VALUES(?,?,?,?)", report[:], h.portfolioID[:], asset[:], i+10)
		}
		point, err := h.valuation.WritePoint(h.ctx, h.portfolioID, cycle, when)
		if err != nil {
			t.Fatal(err)
		}
		points = append(points, point)
	}
	// Unadopted quotes collapse to one eligible quote per local day. The latest
	// successful source's 24-hour price window remains pinned for future remaps.
	for i := range 3 {
		id := uuid.New()
		when := at.Add(time.Duration(i) * time.Hour)
		exec("INSERT INTO price_observations(id,portfolio_id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'okx','BTC','BTCUSD','USD',100,?)", id[:], h.portfolioID[:], when)
	}
	// The last successful report (times[3]) pins non-anchor quotes inside its
	// 24-hour window; a non-anchor quote before that window is removed.
	pinnedQuote, unpinnedQuote := uuid.New(), uuid.New()
	for _, quote := range []struct {
		id     uuid.UUID
		symbol string
		when   time.Time
	}{
		{pinnedQuote, "ETH", at.Add(13 * time.Hour)},
		{uuid.New(), "ETH", at.Add(20 * time.Hour)},
		{unpinnedQuote, "BTC", at.Add(time.Hour)},
		{uuid.New(), "BTC", at.Add(2 * time.Hour)},
	} {
		exec("INSERT INTO price_observations(id,portfolio_id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'binance',?,?,'USDT',100,?)", quote.id[:], h.portfolioID[:], quote.symbol, quote.symbol+"USDT", quote.when)
	}
	pc := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	before, err := h.financial.CustomValueChange(h.ctx, pc, times[0], times[4])
	if err != nil {
		t.Fatal(err)
	}
	service := retention.NewService(h.db)
	now := at.AddDate(0, 0, 50)
	// settle steps until a full round changes and scans nothing.
	settle := func(each func()) int {
		t.Helper()
		for steps := 1; steps <= 400; steps++ {
			idle, err := service.Step(h.ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if idle {
				return steps
			}
			each()
		}
		t.Fatal("retention never became idle")
		return 0
	}
	checkedPruning := false
	settle(func() {
		if checkedPruning {
			return
		}
		var state string
		if err := h.db.QueryRowContext(h.ctx, "SELECT detail_state FROM valuation_points WHERE portfolio_id=? AND cycle_id=?", h.portfolioID[:], points[0].CycleID[:]).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == "pruning" {
			detail, err := h.financial.Detail(h.ctx, pc, points[0].CycleID)
			if err != nil || detail.Point.DetailState != "expired" || len(detail.Accounts) != 0 {
				t.Fatalf("partially pruned details exposed: %+v %v", detail, err)
			}
			checkedPruning = true
			service = retention.NewService(h.db)
		}
	})
	if !checkedPruning {
		t.Fatal("pruning transition not exercised")
	}
	for i, old := range points {
		got, err := h.valuation.Get(h.ctx, h.portfolioID, old.CycleID)
		if err != nil {
			t.Fatal(err)
		}
		wantState := "daily"
		if i == 0 || i == 2 {
			wantState = "expired"
		}
		if i == 4 {
			wantState = "full"
		}
		if got.DetailState != wantState {
			t.Fatalf("point %d detail=%s want %s", i, got.DetailState, wantState)
		}
		got.DetailState = old.DetailState
		if !reflect.DeepEqual(old, got) {
			t.Fatalf("archive changed point %d: before=%+v after=%+v", i, old, got)
		}
	}
	after, err := h.financial.CustomValueChange(h.ctx, pc, times[0], times[4])
	if err != nil {
		t.Fatal(err)
	}
	before.StartPoint.DetailState = after.StartPoint.DetailState
	before.EndPoint.DetailState = after.EndPoint.DetailState
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("archive changed comparison: %+v -> %+v", before, after)
	}
	expired, err := h.financial.Detail(h.ctx, pc, points[0].CycleID)
	if err != nil || expired.Point.DetailState != "expired" || len(expired.Accounts) != 0 {
		t.Fatalf("expired partial read: %+v %v", expired, err)
	}
	snapshot, err := h.financial.Snapshot(h.ctx, pc, "2026-10-03")
	if err != nil || snapshot.Point.CycleID != points[3].CycleID || len(snapshot.Accounts) != 1 {
		t.Fatalf("cross-day fallback: %+v %v", snapshot, err)
	}
	// A retained failed point must keep its earlier successful source intact.
	stale, err := h.financial.Detail(h.ctx, pc, points[1].CycleID)
	if err != nil || len(stale.Accounts) != 1 || stale.Accounts[0].SourceReportID == nil || *stale.Accounts[0].SourceReportID != reports[0] {
		t.Fatalf("stale source lost: %+v %v", stale, err)
	}
	var count int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM account_reports WHERE id=?", reports[2][:]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan report retained: %d %v", count, err)
	}
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM price_observations WHERE portfolio_id=? AND platform='okx'", h.portfolioID[:]).Scan(&count); err != nil || count != 1 {
		t.Fatalf("daily quotes=%d %v", count, err)
	}
	for quote, want := range map[uuid.UUID]int{pinnedQuote: 1, unpinnedQuote: 0} {
		if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM price_observations WHERE portfolio_id=? AND id=?", h.portfolioID[:], quote[:]).Scan(&count); err != nil || count != want {
			t.Fatalf("source-window quote %s count=%d want %d: %v", quote, count, want, err)
		}
	}
	exec("UPDATE portfolios SET timezone='America/Los_Angeles' WHERE id=?", h.portfolioID[:])
	service = retention.NewService(h.db)
	settle(func() {})
	for _, i := range []int{1, 3} {
		point, err := h.valuation.Get(h.ctx, h.portfolioID, points[i].CycleID)
		if err != nil || point.DetailState != "daily" {
			t.Fatalf("timezone replaced anchor: %+v %v", point, err)
		}
	}
	if _, err := h.financial.Snapshot(h.ctx, pc, "2026-09-01"); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("prehistory snapshot: %v", err)
	}
	var raw []byte
	if err := h.db.QueryRowContext(h.ctx, "SELECT id FROM account_reports WHERE id=?", reports[0][:]).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		t.Fatal("pinned source deleted")
	}
	// A settled service does not rescan pinned history: each of the eleven phases
	// visits the one portfolio and wraps. The next cutoff day starts new sweeps.
	if steps := settle(func() {}); steps != 11*2 {
		t.Fatalf("idle round took %d steps", steps)
	}
	late := uuid.New()
	exec("INSERT INTO price_observations(id,portfolio_id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'okx','BTC','BTCUSD','USD',100,?)", late[:], h.portfolioID[:], at.Add(3*time.Hour))
	now = now.AddDate(0, 0, 1)
	settle(func() {})
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM price_observations WHERE id=?", late[:]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("next-day sweep kept quote: %d %v", count, err)
	}
}

func TestRetentionResumesManualRevisionsAfterCutoffAndReportRelease(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("manual retention", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	var account uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "manual" {
			account = a.ID
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.db.ExecContext(h.ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:])
	at := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	for revision, when := range map[int]time.Time{2: at.Add(-24 * time.Hour), 3: at.Add(-23 * time.Hour), 4: at, 5: at.AddDate(0, 0, 30)} {
		exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES(?,?,?,?)", h.portfolioID[:], account[:], revision, when)
	}
	addPoint := func(when time.Time, revision int, state string) {
		t.Helper()
		cycle, report := uuid.New(), uuid.New()
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','user',?,?,?,?)", h.portfolioID[:], cycle[:], when, when, when, when)
		exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at,manual_revision_number) VALUES(?,?,?,?,'succeeded',?,?,?)", h.portfolioID[:], report[:], cycle[:], account[:], when, when, revision)
		exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES(?,?,?,0,1,0,0,1,?)", h.portfolioID[:], cycle[:], when, state)
		exec("INSERT INTO account_valuations(portfolio_id,cycle_id,account_id,source_report_id,valuation_method,value_usd) VALUES(?,?,?,?,'calculated',0)", h.portfolioID[:], cycle[:], account[:], report[:])
	}
	// More than two candidate pages stay pinned permanently. The expiring
	// report is reached after manual cleanup has found no eligible revisions.
	for day := range 130 {
		addPoint(at.AddDate(0, 0, day-200), 1, "daily")
	}
	addPoint(at, 2, "full")
	addPoint(at.Add(time.Hour), 3, "full")
	service := retention.NewService(h.db)
	now := at.AddDate(0, 0, 14)
	settle := func() {
		t.Helper()
		for range 500 {
			idle, err := service.Step(h.ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if idle {
				return
			}
		}
		t.Fatal("Manual retention never became idle")
	}
	assertRevision := func(revision, want int) {
		t.Helper()
		var got int
		if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM manual_revisions WHERE portfolio_id=? AND account_id=? AND revision_number=?", h.portfolioID[:], account[:], revision).Scan(&got); err != nil || got != want {
			t.Fatalf("revision %d count=%d want %d: %v", revision, got, want, err)
		}
	}
	settle()
	assertRevision(2, 1)
	assertRevision(4, 1)
	// The next cutoff releases revision 4 directly. Revision 2 becomes
	// removable only after its report is reached on the third candidate page.
	now = now.AddDate(0, 0, 1)
	settle()
	assertRevision(2, 0)
	assertRevision(4, 0)
	for _, revision := range []int{1, 3, 5} {
		assertRevision(revision, 1)
	}
}

func TestRetentionBatchesReportsAndResumesOversizedChildren(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("retention pages", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	var account uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "spot" {
			account = a.ID
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.db.ExecContext(h.ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	count := func(table string) int {
		t.Helper()
		var n int
		if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM "+table+" WHERE portfolio_id=?", h.portfolioID[:]).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := at.AddDate(0, 0, 60)
	// One report exceeds the per-table budget; the others are small enough to
	// share a visit. All four child families exercise parent deletion guards.
	for start := 0; start < 1201; start += 200 {
		var rows []string
		var args []any
		for i := start; i < min(start+200, 1201); i++ {
			asset := uuid.New()
			rows = append(rows, "(?,?,'binance',?,'USD',1,?,?)")
			args = append(args, h.portfolioID[:], asset[:], fmt.Sprintf("ASSET%d", i), at, at)
		}
		exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES "+strings.Join(rows, ","), args...)
	}
	contract := uuid.New()
	exec("INSERT INTO futures_contracts(portfolio_id,id,platform,symbol,terms_hash,kind,first_observed_at,last_observed_at) VALUES(?,?,'binance','BTCUSD',?,'linear_perpetual',?,?)", h.portfolioID[:], contract[:], make([]byte, 32), at, at)
	var latest uuid.UUID
	for i := range 70 {
		cycle, report := uuid.New(), uuid.New()
		when := at.Add(time.Duration(i) * time.Minute)
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','user',?,?,?,?)", h.portfolioID[:], cycle[:], when, when, when, when)
		exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at) VALUES(?,?,?,?,'succeeded',?,?)", h.portfolioID[:], report[:], cycle[:], account[:], when, when)
		balances := 5
		if i == 0 {
			balances = 1201
		}
		exec("INSERT INTO balances(portfolio_id,report_id,asset_id,equity) SELECT portfolio_id,?,id,1 FROM assets WHERE portfolio_id=? ORDER BY id LIMIT ?", report[:], h.portfolioID[:], balances)
		exec("INSERT INTO futures_positions(portfolio_id,report_id,position_key,futures_contract_id,side,position_mode,source_quantity,source_quantity_unit) VALUES(?,?,'position',?,'long','one_way',1,'base')", h.portfolioID[:], report[:], contract[:])
		exec("INSERT INTO earn_positions(portfolio_id,report_id,position_key,product_type,granularity,amount_basis,details_json) VALUES(?,?,'earn','flexible','position','principal','{}')", h.portfolioID[:], report[:])
		exec("INSERT INTO dual_positions(portfolio_id,report_id,position_key,principal_asset_symbol,principal,status) VALUES(?,?,'dual','USD',1,'active')", h.portfolioID[:], report[:])
		if i == 0 || i == 69 {
			exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?, ?,1,0,0,1)", h.portfolioID[:], cycle[:], when, balances)
			exec("INSERT INTO account_valuations(portfolio_id,cycle_id,account_id,source_report_id,valuation_method,value_usd) VALUES(?,?,?,?,'calculated',?)", h.portfolioID[:], cycle[:], account[:], report[:], balances)
			exec("INSERT INTO balance_valuations(portfolio_id,cycle_id,account_id,source_report_id,asset_id,canonical_asset_symbol,pricing_method,value_usd) SELECT portfolio_id,?,?,report_id,asset_id,'USD','usd_parity',equity FROM balances WHERE portfolio_id=? AND report_id=?", cycle[:], account[:], h.portfolioID[:], report[:])
		}
		price := uuid.New()
		exec("INSERT INTO price_observations(portfolio_id,id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'okx','BTC','BTCUSD','USD',100,?)", h.portfolioID[:], price[:], when)
		latest = report
	}
	active := uuid.New()
	exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at) VALUES(?,?,'queued','user',?,?)", h.portfolioID[:], active[:], now, now)
	service := retention.NewService(h.db)
	for range 24 {
		if _, err := service.Step(h.ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	if got := count("account_reports"); got != 70 {
		t.Fatalf("active cycle lost reports: %d", got)
	}
	if got := count("balances"); got != 1201+69*5 {
		t.Fatalf("active cycle lost balances: %d", got)
	}
	exec("UPDATE collection_cycles SET status='failed',finished_at=? WHERE portfolio_id=? AND id=?", now, h.portfolioID[:], active[:])
	service = retention.NewService(h.db)
	previousReports, previousBalances := count("account_reports"), count("balances")
	previousValuations := count("balance_valuations")
	previousPrices := count("price_observations")
	restarted, batched, idle := false, false, false
	for steps := 0; steps < 200; steps++ {
		var err error
		idle, err = service.Step(h.ctx, now)
		if err != nil {
			t.Fatal(err)
		}
		reports, balances := count("account_reports"), count("balances")
		if removed := previousReports - reports; removed > 64 {
			t.Fatalf("unbounded report deletion: %d", removed)
		} else if removed > 1 {
			batched = true
		}
		if removed := previousBalances - balances; removed > 1000 {
			t.Fatalf("unbounded child deletion: %d", removed)
		} else if removed > 0 && balances > 5 && !restarted {
			// Durable rows alone must suffice to resume a partly deleted page.
			service = retention.NewService(h.db)
			restarted = true
		}
		valuations := count("balance_valuations")
		if removed := previousValuations - valuations; removed > 1000 {
			t.Fatalf("unbounded valuation deletion: %d", removed)
		} else if removed > 0 && valuations > 5 {
			var state string
			if err := h.db.QueryRowContext(h.ctx, "SELECT detail_state FROM valuation_points WHERE portfolio_id=? ORDER BY evaluated_at LIMIT 1", h.portfolioID[:]).Scan(&state); err != nil || state != "pruning" {
				t.Fatalf("partial point expired: state=%s err=%v", state, err)
			}
		}
		prices := count("price_observations")
		if removed := previousPrices - prices; removed > 64 {
			t.Fatalf("unbounded price deletion: %d", removed)
		}
		previousPrices = prices
		previousValuations = valuations
		previousReports, previousBalances = reports, balances
		if idle {
			break
		}
	}
	if !idle || !batched || !restarted {
		t.Fatalf("idle=%t batched=%t restarted=%t", idle, batched, restarted)
	}
	for table, want := range map[string]int{"account_reports": 1, "balances": 5, "futures_positions": 1, "earn_positions": 1, "dual_positions": 1, "account_valuations": 1, "balance_valuations": 5, "price_observations": 1} {
		if got := count(table); got != want {
			t.Fatalf("%s count=%d want %d", table, got, want)
		}
	}
	var remaining []byte
	if err := h.db.QueryRowContext(h.ctx, "SELECT id FROM account_reports WHERE portfolio_id=?", h.portfolioID[:]).Scan(&remaining); err != nil || !bytes.Equal(remaining, latest[:]) {
		t.Fatalf("last successful report pin lost: %x %v", remaining, err)
	}
}
