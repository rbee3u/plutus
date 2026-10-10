package integration

import (
	"database/sql"
	"errors"
	"reflect"
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
	checkedPruning := false
	for range 280 {
		if err := service.Step(h.ctx, now); err != nil {
			t.Fatal(err)
		}

		if !checkedPruning {
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
		}
	}
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
	for range 50 {
		if err := service.Step(h.ctx, now); err != nil {
			t.Fatal(err)
		}
	}
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
}
