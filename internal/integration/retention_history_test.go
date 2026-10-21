package integration

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/retention"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

func TestRollingHistoryPreservesWindowAndCurrentDependencies(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("rolling", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	var manualID uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "manual" {
			manualID = a.ID
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.db.ExecContext(h.ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	count := func(query string, want int, args ...any) {
		t.Helper()
		var got int
		if err := h.db.QueryRowContext(h.ctx, query, args...).Scan(&got); err != nil || got != want {
			t.Fatalf("count=%d want %d: %s: %v", got, want, query, err)
		}
	}
	exec("UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:])
	now := time.Date(2036, 10, 11, 12, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	addPoint := func(at time.Time, value int) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", h.portfolioID[:], id[:], at, at, at, at)
		exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,?,1,0,0,1)", h.portfolioID[:], id[:], at, value)
		return id
	}
	old := make([]uuid.UUID, 70)
	for i := range old {
		old[i] = addPoint(cutoff.AddDate(0, 0, -100+i), 50)
	}
	baselineAt := cutoff.Add(-30 * time.Minute)
	baseline := addPoint(baselineAt, 100)
	exec("UPDATE collection_cycles SET created_at=? WHERE portfolio_id=? AND id=?", cutoff.AddDate(0, 0, -8), h.portfolioID[:], baseline[:])
	first := addPoint(cutoff.Add(time.Hour), 140)
	at15 := now.AddDate(0, 0, -15).Truncate(24 * time.Hour)
	intraday := addPoint(at15, 140)
	daily := addPoint(at15.Add(30*time.Minute), 140)
	recent := addPoint(now.AddDate(0, 0, -14), 140)
	addPoint(now, 140)
	asset := uuid.New()
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','BTC','BTC',2,?,?)", h.portfolioID[:], asset[:], cutoff.AddDate(0, 0, -100), now)
	exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) VALUES(?,?,?,?,'included','BTC')", h.portfolioID[:], asset[:], cutoff.AddDate(0, 0, -100), old[0][:])
	// Only the last state before the opening point survives: an obsolete state
	// on old[1] must not resurrect or change the first in-window comparison.
	exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) VALUES(?,?,?,?,'included','BTC')", h.portfolioID[:], asset[:], cutoff.AddDate(0, 0, -99), old[1][:])
	exec("INSERT INTO manual_balance_changes(portfolio_id,cycle_id,account_id) VALUES(?,?,?)", h.portfolioID[:], first[:], manualID[:])
	exec("INSERT INTO boundary_events(portfolio_id,cycle_id,account_id,direction,valuation_method,value_usd,valuation_complete) VALUES(?,?,?,'contribution','calculated',10,TRUE)", h.portfolioID[:], first[:], manualID[:])
	for v := 1; v <= 4; v++ {
		at := cutoff.AddDate(0, 0, -10+v)
		if v == 4 {
			at = cutoff.Add(2 * time.Hour)
		}
		excluded := v%2 == 1
		var reason any
		if excluded {
			reason = "historical reason"
		}
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,?,?,?)", h.portfolioID[:], asset[:], v, excluded, reason, at)
	}
	auditID := uuid.New()
	exec("INSERT INTO audit_events(portfolio_id,id,action,resource_type,resource_id,before_json,after_json,occurred_at) VALUES(?,?,'catalog.asset.exclude','asset',?,'{\"version\":2}','{\"version\":3}',?)", h.portfolioID[:], auditID[:], asset[:], cutoff.Add(time.Hour))
	staleAudit := uuid.New()
	exec("INSERT INTO audit_events(portfolio_id,id,action,resource_type,resource_id,occurred_at) VALUES(?,?,'manual.balance.update','identity',?,?)", h.portfolioID[:], staleAudit[:], item.ID[:], cutoff.Add(-time.Hour))
	// Dormant manual state is applied, even after the owning point expires.
	for v := 2; v <= 3; v++ {
		exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES(?,?,?,?)", h.portfolioID[:], manualID[:], v, cutoff.AddDate(0, 0, -100+v))
	}
	report := uuid.New()
	exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at,manual_revision_number) VALUES(?,?,?,?,'succeeded',?,?,2)", h.portfolioID[:], report[:], old[2][:], manualID[:], cutoff.AddDate(0, 0, -98), cutoff.AddDate(0, 0, -98))
	for _, offset := range []time.Duration{-time.Hour, -15 * time.Minute, 0} {
		id := uuid.New()
		at := cutoff.Add(offset)
		exec("INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,version,created_at,updated_at) VALUES(?,?,'contribution','USD',3,3,'explicit',?,1,?,1,?,?)", h.portfolioID[:], id[:], at, at, at, at)
	}
	quote := uuid.New()
	exec("INSERT INTO price_observations(portfolio_id,id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at,daily_anchor) VALUES(?,?,'okx','BTC','BTCUSD','USD',100,?,TRUE)", h.portfolioID[:], quote[:], cutoff.AddDate(0, 0, -20))
	unused := uuid.New()
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','UNUSED','UNUSED',1,?,?)", h.portfolioID[:], unused[:], cutoff.AddDate(0, 0, -20), cutoff.AddDate(0, 0, -20))
	pc := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	before, err := h.financial.CustomValueChange(h.ctx, pc, cutoff, cutoff.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	service := retention.NewService(h.db)
	idleCount := 0
	for step := 0; step < 10000; step++ {
		idle, err := service.Step(h.ctx, now)
		if err != nil {
			t.Fatal(err)
		}
		// Recreate the service during a partially completed sweep.
		if step == 79 {
			service = retention.NewService(h.db)
		}
		if idle {
			idleCount++
		}
		if idleCount == 2 {
			break
		}
		if step == 9999 {
			t.Fatal("rolling cleanup did not settle")
		}
	}
	after, err := h.financial.CustomValueChange(h.ctx, pc, cutoff, cutoff.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if before.AmountUSD == nil || after.AmountUSD == nil || *before.AmountUSD != *after.AmountUSD || !reflect.DeepEqual(before.Issues, after.Issues) || !reflect.DeepEqual(before.BoundaryEvents, after.BoundaryEvents) {
		t.Fatalf("window comparison changed: %+v -> %+v", before, after)
	}
	count("SELECT COUNT(*) FROM valuation_points WHERE portfolio_id=? AND evaluated_at<?", 2, h.portfolioID[:], cutoff)
	count("SELECT COUNT(*) FROM valuation_points WHERE portfolio_id=? AND cycle_id=?", 0, h.portfolioID[:], old[2][:])
	count("SELECT COUNT(*) FROM collection_cycles WHERE portfolio_id=? AND id=?", 1, h.portfolioID[:], old[2][:])
	count("SELECT COUNT(*) FROM capital_flows WHERE portfolio_id=?", 2, h.portfolioID[:])
	count("SELECT COUNT(*) FROM audit_events WHERE portfolio_id=? AND id=?", 0, h.portfolioID[:], staleAudit[:])
	count("SELECT COUNT(*) FROM asset_exclusion_versions WHERE portfolio_id=? AND asset_id=?", 4, h.portfolioID[:], asset[:])
	count("SELECT COUNT(*) FROM price_observations WHERE portfolio_id=? AND id=?", 0, h.portfolioID[:], quote[:])
	count("SELECT COUNT(*) FROM assets WHERE portfolio_id=? AND id=?", 0, h.portfolioID[:], unused[:])
	for _, tc := range []struct {
		id    uuid.UUID
		state string
	}{{intraday, "expired"}, {daily, "daily"}, {recent, "full"}, {baseline, "daily"}, {old[1], "expired"}} {
		var state string
		if err := h.db.QueryRowContext(h.ctx, "SELECT detail_state FROM valuation_points WHERE portfolio_id=? AND cycle_id=?", h.portfolioID[:], tc.id[:]).Scan(&state); err != nil || state != tc.state {
			t.Fatalf("point %s state=%s want=%s: %v", tc.id, state, tc.state, err)
		}
	}
	expiredRange, err := h.financial.CustomValueChange(h.ctx, pc, cutoff.Add(-time.Hour), cutoff.Add(time.Hour))
	if err != nil || expiredRange.StartPoint != nil || !strings.Contains(fmt.Sprint(expiredRange.Issues), "no_start_point") {
		t.Fatalf("internal predecessor exposed as history: %+v %v", expiredRange, err)
	}
	if _, err := h.financial.Detail(h.ctx, pc, old[1]); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("internal predecessor detail exposed: %v", err)
	}
	pagePoints, err := h.financial.PointsPage(h.ctx, pc, cutoff.AddDate(0, 0, -200), now.Add(time.Hour), 0, 20)
	if err != nil || pagePoints.Total != 5 {
		t.Fatalf("public retained point count: %+v %v", pagePoints, err)
	}
	firstPublic, err := h.valuation.First(h.ctx, h.portfolioID)
	if err != nil || firstPublic.CycleID != first {
		t.Fatalf("public first point: %+v %v", firstPublic, err)
	}

	applied, err := sqlcgen.New(h.db).GetAppliedManualRevision(h.ctx, sqlcgen.GetAppliedManualRevisionParams{PortfolioID: h.portfolioID[:], AccountID: manualID[:]})
	if err != nil || applied != (sql.NullInt64{Int64: 2, Valid: true}) {
		t.Fatalf("lost applied manual state: %+v %v", applied, err)
	}
	page, err := h.services.Audit.List(h.ctx, h.portfolioID, &audit.ListFilter{Action: "catalog.asset.exclude"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("retained audit lost version: %+v %v", page, err)
	}
}

func TestRollingHistoryKeepsExactCutoffFlowWithoutOpeningPoint(t *testing.T) {
	h := newHarness(t)
	if _, err := h.db.ExecContext(h.ctx, "UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:]); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,version,created_at,updated_at) VALUES(?,?,'contribution','USD',3,3,'explicit',?,1,?,1,?,?)", h.portfolioID[:], id[:], at, at, at, at); err != nil {
		t.Fatal(err)
	}
	service := retention.NewService(h.db)
	for range 200 {
		idle, err := service.Step(h.ctx, at.AddDate(10, 0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if idle {
			break
		}
	}
	var got int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM capital_flows WHERE portfolio_id=? AND id=?", h.portfolioID[:], id[:]).Scan(&got); err != nil || got != 1 {
		t.Fatalf("cutoff flow lost: %d %v", got, err)
	}
}

func TestDormantCurrentPositionsSurviveHistoryExpiry(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Dormant", adapter.PlatformBinance, adapter.StructureBinanceClassic, "um_futures")
	item = h.storeCredentials(item, "dormant-positions")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	summary, cycleID := h.runCycle()
	accountID := h.accountsByType(cycleID, item.ID)["um_futures"].AccountID
	pc := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	before, err := h.financial.AccountPositionsAt(h.ctx, pc, accountID, &cycleID)
	if err != nil || len(before.Positions) == 0 {
		t.Fatalf("positions before: %+v %v", before, err)
	}
	readers := []struct {
		name string
		read func(*financial.ListFilter) (any, error)
	}{
		{"balances", func(f *financial.ListFilter) (any, error) { return h.financial.FlatBalances(h.ctx, pc, f) }},
		{"positions", func(f *financial.ListFilter) (any, error) { return h.financial.FlatPositions(h.ctx, pc, f) }},
		{"earn", func(f *financial.ListFilter) (any, error) { return h.financial.FlatEarnPositions(h.ctx, pc, f) }},
		{"dual", func(f *financial.ListFilter) (any, error) { return h.financial.FlatDualPositions(h.ctx, pc, f) }},
	}
	service := retention.NewService(h.db)
	for range 1000 {
		idle, err := service.Step(h.ctx, summary.Point.EvaluatedAt.AddDate(11, 0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if idle {
			break
		}
	}
	after, err := h.financial.AccountPositionsAt(h.ctx, pc, accountID, &cycleID)
	if err != nil || !reflect.DeepEqual(before.Positions, after.Positions) {
		t.Fatalf("current positions changed: %+v %v", after, err)
	}
	if _, err := h.financial.Detail(h.ctx, pc, cycleID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("expired public history: %v", err)
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			latest, err := reader.read(&financial.ListFilter{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			pinned, err := reader.read(&financial.ListFilter{CycleID: &cycleID, Limit: 1})
			if err != nil || !reflect.DeepEqual(latest, pinned) {
				t.Fatalf("pinning dormant current holdings changed the read: %v", err)
			}
			missing := uuid.New()
			if _, err := reader.read(&financial.ListFilter{CycleID: &missing}); !errors.Is(err, valuation.ErrNotFound) {
				t.Fatalf("unknown point: %v", err)
			}
		})
	}
}

func TestRollingHistoryPreservesPolicyForCycleCrossingCutoff(t *testing.T) {
	h := newHarness(t)
	at := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.db.ExecContext(h.ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:])
	asset, cycle := uuid.New(), uuid.New()
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','BTC','BTC',2,?,?)", h.portfolioID[:], asset[:], at, at)
	for v := 1; v <= 3; v++ {
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,TRUE,'test',?)", h.portfolioID[:], asset[:], v, at.Add(time.Duration(v-4)*time.Minute))
	}
	exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", h.portfolioID[:], cycle[:], at.Add(-90*time.Second), at.Add(-90*time.Second), at.Add(time.Minute), at.Add(time.Minute))
	exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,0,0,0,0,1)", h.portfolioID[:], cycle[:], at.Add(time.Minute))
	service := retention.NewService(h.db)
	for range 1000 {
		idle, err := service.Step(h.ctx, at.AddDate(10, 0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if idle {
			break
		}
	}
	var count, earliest int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*),MIN(version) FROM asset_exclusion_versions WHERE portfolio_id=? AND asset_id=?", h.portfolioID[:], asset[:]).Scan(&count, &earliest); err != nil || count != 2 || earliest != 2 {
		t.Fatalf("adopted version lost: count=%d earliest=%d %v", count, earliest, err)
	}
}

// Repeated window advancement exercises release of pins, not just deletion of
// an initially old fixture. Assets and contracts rotate while holdings stay fixed.
func TestRollingHistoryReleasesReferencesAcrossThirtyYears(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("rolling sources", adapter.PlatformBinance, adapter.StructureBinanceClassic, "um_futures")
	var account uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "um_futures" {
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
	exec("UPDATE portfolios SET timezone='UTC' WHERE id=?", h.portfolioID[:])
	pc := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	service := retention.NewService(h.db)
	var stable map[string]int
	policyAsset := uuid.New()
	firstAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", h.portfolioID[:], policyAsset[:], firstAt, firstAt)
	for year := 2000; year < 2035; year++ {
		at := time.Date(year, 1, 1, 12, 0, 0, 0, time.UTC)
		asset, contract, quote := uuid.New(), uuid.New(), uuid.New()
		symbol := fmt.Sprintf("ASSET%d", year)
		version := year - 1999
		exec("UPDATE assets SET last_observed_at=? WHERE portfolio_id=? AND id=?", at, h.portfolioID[:], policyAsset[:])
		excluded := version%2 == 1
		var reason any
		action := "catalog.asset.restore"
		if excluded {
			reason = "annual policy"
			action = "catalog.asset.exclude"
		}
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,?,?,?)", h.portfolioID[:], policyAsset[:], version, excluded, reason, at.Add(-time.Minute))
		auditID := uuid.New()
		exec("INSERT INTO audit_events(portfolio_id,id,action,resource_type,resource_id,before_json,after_json,occurred_at) VALUES(?,?,?,'asset',?,?,?,?)", h.portfolioID[:], auditID[:], action, policyAsset[:], fmt.Sprintf(`{"version":%d}`, version-1), fmt.Sprintf(`{"version":%d}`, version), at.Add(-time.Minute))
		flowID := uuid.New()
		exec("INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,version,created_at,updated_at) VALUES(?,?,'contribution','USD',3,3,'explicit',?,1,?,1,?,?)", h.portfolioID[:], flowID[:], at, at, at, at)
		exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance',?,?,1,?,?)", h.portfolioID[:], asset[:], symbol, symbol, at, at)
		exec("INSERT INTO futures_contracts(portfolio_id,id,platform,symbol,terms_hash,kind,first_observed_at,last_observed_at) VALUES(?,?,'binance',?,?,'linear_perpetual',?,?)", h.portfolioID[:], contract[:], symbol, make([]byte, 32), at, at)
		exec("INSERT INTO price_observations(portfolio_id,id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'binance',?,?,'USD',10,?)", h.portfolioID[:], quote[:], symbol, symbol+"USD", at)
		// One successful source is shared by a later failed daily snapshot.
		var latest valuation.Point
		for n := range 2 {
			cycle, report := uuid.New(), uuid.New()
			when := at.Add(time.Duration(n) * time.Hour)
			exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", h.portfolioID[:], cycle[:], when, when, when, when)
			if n == 0 {
				exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,collected_at,recorded_at) VALUES(?,?,?,?,'succeeded',?,?)", h.portfolioID[:], report[:], cycle[:], account[:], when, when)
				exec("INSERT INTO balances(portfolio_id,report_id,asset_id,equity) VALUES(?,?,?,1)", h.portfolioID[:], report[:], asset[:])
				exec("INSERT INTO balances(portfolio_id,report_id,asset_id,equity) VALUES(?,?,?,100)", h.portfolioID[:], report[:], policyAsset[:])
				exec("INSERT INTO futures_positions(portfolio_id,report_id,position_key,futures_contract_id,side,position_mode,source_quantity,source_quantity_unit) VALUES(?,?,'position',?,'long','one_way',1,'base')", h.portfolioID[:], report[:], contract[:])
			} else {
				exec("INSERT INTO account_reports(portfolio_id,id,cycle_id,account_id,status,error_class,recorded_at) VALUES(?,?,?,?,'failed','temporary',?)", h.portfolioID[:], report[:], cycle[:], account[:], when)
			}
			var err error
			latest, err = h.valuation.WritePoint(h.ctx, h.portfolioID, cycle, when)
			if err != nil {
				t.Fatal(err)
			}
		}
		now := at.AddDate(0, 0, 20)
		cutoff := time.Date(year-10, 1, 21, 0, 0, 0, 0, time.UTC)
		before, err := h.financial.CustomValueChange(h.ctx, pc, cutoff, latest.EvaluatedAt)
		if err != nil {
			t.Fatal(err)
		}
		settled := false
		for step := range 10000 {
			idle, err := service.Step(h.ctx, now)
			if err != nil {
				t.Fatalf("year %d step %d: %v", year, step, err)
			}
			if idle {
				settled = true
				break
			}
		}
		if !settled {
			t.Fatalf("year %d cleanup did not settle", year)
		}
		after, err := h.financial.CustomValueChange(h.ctx, pc, cutoff, latest.EvaluatedAt)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before.AmountUSD, after.AmountUSD) || !reflect.DeepEqual(before.Issues, after.Issues) || !reflect.DeepEqual(before.BoundaryEvents, after.BoundaryEvents) {
			t.Fatalf("year %d comparison changed: %+v -> %+v", year, before, after)
		}
		positions, err := h.financial.AccountPositionsAt(h.ctx, pc, account, &latest.CycleID)
		if err != nil || len(positions.Positions) != 1 {
			t.Fatalf("year %d source lost: %+v %v", year, positions, err)
		}
		audits, err := h.services.Audit.List(h.ctx, h.portfolioID, nil)
		if err != nil || len(audits.Items) == 0 {
			t.Fatalf("year %d audit policy references: %+v %v", year, audits, err)
		}
		if year >= 2010 {
			snapshot, err := h.financial.Snapshot(h.ctx, pc, fmt.Sprintf("%d-01-01", year-9))
			if err != nil || snapshot.Point.DetailState != "daily" || len(snapshot.Accounts) != 1 {
				t.Fatalf("year %d retained snapshot: %+v %v", year, snapshot, err)
			}
			historical, err := h.financial.AccountPositionsAt(h.ctx, pc, account, &snapshot.Point.CycleID)
			if err != nil || len(historical.Positions) != 1 {
				t.Fatalf("year %d historical source lost: %+v %v", year, historical, err)
			}
			if after.AmountUSD == nil || (after.ContributionsUSD == nil || *after.ContributionsUSD != "30") {
				t.Fatalf("year %d comparison not exercised: %+v", year, after)
			}
		}
		// At steady state the previous opening snapshot's pins must be released.
		if year >= 2013 {
			got := map[string]int{}
			for _, table := range []string{"valuation_points", "collection_cycles", "account_reports", "account_valuations", "balances", "balance_valuations", "futures_positions", "futures_contracts", "assets", "calculated_asset_changes", "price_observations", "capital_flows", "audit_events", "asset_exclusion_versions"} {
				got[table] = count(table)
			}
			if stable != nil && !reflect.DeepEqual(stable, got) {
				t.Fatalf("year %d retained rows grew: %v -> %v", year, stable, got)
			}
			stable = got
		}
		// A restart must not be necessary to release old references.
		if year == 2020 {
			service = retention.NewService(h.db)
		}
	}
	t.Logf("Steady retained row counts across 22 annual window advances: %v", stable)
}
