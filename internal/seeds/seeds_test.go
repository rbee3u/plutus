package seeds

import (
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
)

func TestMain(m *testing.M) { offline.Install(); mysqltest.RunTestMain(m) }

func TestBuild(t *testing.T) {
	anchor := time.Date(2026, 9, 22, 10, 7, 0, 0, time.UTC)
	seed, err := BuildAt(anchor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seed.SQL, "'alice'") || strings.Contains(seed.SQL, "'bob'") {
		t.Fatal("only Alice must be seeded")
	}
	if len(seed.Portfolios) != 4 {
		t.Fatal("expected four theme Portfolios")
	}
	if strings.Contains(seed.SQL, "INSERT INTO valuation_points") {
		t.Fatal("seeds must not write derived points")
	}
	again, err := BuildAt(anchor)
	if err != nil || again.SQL != seed.SQL {
		t.Fatalf("non-deterministic seed: %v", err)
	}
	if UserAlice != SeedID(kindUser, 1) || PortfolioAliceMain.String() != "01a0bdfc-2b00-7002-8000-000000000001" {
		t.Fatal("retained IDs changed")
	}
	for _, plan := range seed.Live {
		if plan.Credentials.APIKey == "" || plan.Status == "" {
			t.Fatal("incomplete activation plan")
		}
	}
}

func TestDefinitionFailures(t *testing.T) {
	if _, err := BuildAt(time.Time{}); err == nil {
		t.Fatal("zero anchor accepted")
	}
	b := newBuilder()
	b.jsonLiteral(make(chan int))
	if b.err == nil {
		t.Fatal("serialization failure silently discarded")
	}
	b = newBuilder()
	b.add("price_observations", bin(uuid.New()), q("binance"), q("BTC"), q("BTCUSDT"), q("USDT"), "oops", ts(time.Now()))
	if b.err == nil {
		t.Fatal("invalid decimal accepted")
	}
	b = newBuilder()
	b.accounts(uuid.New(), nil, nil, time.Now())
	if b.err == nil {
		t.Fatal("unknown Identity accepted")
	}
	r := adapters.AccountReport{Status: adapters.StatusSucceeded, CollectedAt: time.Now(), Balances: []adapters.Balance{{AssetSymbol: "BTC", Equity: ""}}}
	if validateReport(&r) == nil {
		t.Fatal("unknown Balance equity accepted")
	}
}

func TestCalendarTimeline(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, anchor := range []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC),
	} {
		points := cycleTimes(anchor, berlin, false)
		days := map[string]bool{}
		for i, at := range points {
			if i > 0 && !at.After(points[i-1]) {
				t.Fatal("unordered or duplicate timeline")
			}
			if at.After(anchor) {
				t.Fatal("future seed event")
			}
			days[at.In(berlin).Format("2006-01-02")] = true
		}
		local := anchor.In(berlin)
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, berlin).AddDate(0, -3, 0)
		if anchor.Month() == time.May {
			start = time.Date(2026, time.February, 28, 0, 0, 0, 0, berlin)
		}
		if !points[0].Equal(start) {
			t.Fatalf("three-month start: %s instead of %s", points[0], start)
		}
		expectedDays := 0
		for day := start; !day.After(anchor); day = day.AddDate(0, 0, 1) {
			expectedDays++
			if !days[day.Format("2006-01-02")] {
				t.Fatalf("missing local calendar day %s", day)
			}
		}
		if len(days) != expectedDays {
			t.Fatalf("expected %d local days including today, got %d", expectedDays, len(days))
		}
		if !points[len(points)-1].Equal(anchor) {
			t.Fatal("latest cycle does not equal anchor")
		}
	}
}

func TestSeedsApplyRetryAndCollect(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	anchor := time.Now().UTC().Truncate(15 * time.Minute)
	seed, err := BuildAt(anchor)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, seed); err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertSequentialHistory(ctx, t, db, seed)
	registry := adapters.NewRegistry()
	clock := anchor.Add(time.Minute)
	for _, platform := range adapters.OfferedPlatforms() {
		registry.Register(platform, fake.New(platform).WithClock(func() time.Time { return clock }))
	}
	ids, err := identity.NewService(db, identity.Options{MasterKey: []byte("0123456789abcdef0123456789abcdef"), Adapters: registry})
	if err != nil {
		t.Fatal(err)
	}
	if err := ActivateConnections(ctx, ids, seed.Live, io.Discard); err != nil {
		t.Fatal(err)
	}
	v := valuation.NewService(db)
	assertImmutableRetries(ctx, t, db, v)
	assertWalkthroughReads(ctx, t, db, anchor)
	queue := cycles.NewService(db)
	assets := catalog.NewService(db)
	runner := collection.NewService(db, collection.Options{Adapters: registry, Identities: ids, Catalog: assets, Manual: manual.NewService(db, assets), Valuation: v, RetryDelays: []time.Duration{}})
	for _, p := range []uuid.UUID{PortfolioAliceMain, PortfolioAliceEdge, PortfolioAliceLarge} {
		cycle, _, err := queue.Enqueue(ctx, p, cycles.TriggerUser)
		if err != nil {
			t.Fatal(err)
		}
		if cycle.Trigger != cycles.TriggerUser {
			t.Fatalf("collection trigger lost: %s", cycle.Trigger)
		}
		claimed, ok, err := queue.ClaimOne(ctx, "seed-test", time.Minute)
		if err != nil || !ok || claimed.ID != cycle.ID {
			t.Fatalf("claim: %v %v", ok, err)
		}
		_, err = runner.RunCycle(ctx, p, cycle.ID, &claimed)
		if err != nil {
			t.Fatal(err)
		}
	}
	assertWalkthroughReads(ctx, t, db, anchor)
	for _, d := range fake.DemoIdentities() {
		stored, err := ids.Get(ctx, portfolioID(d.Portfolio), identityID(d.ID))
		if err != nil {
			t.Fatal(err)
		}
		if stored.Connection.Status != d.Connection {
			t.Fatalf("%s Connection: %s instead of %s", d.ID, stored.Connection.Status, d.Connection)
		}
	}
	scheduled, err := queue.ListForScheduling(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range scheduled {
		if p.PortfolioID == PortfolioAliceEmpty {
			t.Fatal("empty Portfolio must not start collecting")
		}
	}
}

func assertWalkthroughReads(ctx context.Context, t *testing.T, db *sql.DB, anchor time.Time) {
	t.Helper()
	v := financial.NewService(db, valuation.NewService(db))
	main := financial.PortfolioContext{ID: PortfolioAliceMain, Timezone: "Asia/Shanghai"}
	edge := financial.PortfolioContext{ID: PortfolioAliceEdge, Timezone: "Europe/Berlin"}
	large := financial.PortfolioContext{ID: PortfolioAliceLarge, Timezone: "UTC"}
	overview, err := v.Overview(ctx, main)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Point == nil || overview.Completeness != valuation.CompletenessComplete || overview.StaleAccountCount != 0 {
		t.Fatalf("daily combination must be healthy: %+v", overview)
	}
	incomplete, err := v.Overview(ctx, edge)
	if err != nil {
		t.Fatal(err)
	}
	if incomplete.Completeness != valuation.CompletenessIncomplete || incomplete.StaleAccountCount == 0 {
		t.Fatalf("edge combination lost missing/stale states: %+v", incomplete)
	}
	earn, err := v.FlatEarnPositions(ctx, edge, &financial.ListFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(earn.Items) == 0 {
		t.Fatal("Earn details lost")
	}
	empty, err := v.Overview(ctx, financial.PortfolioContext{ID: PortfolioAliceEmpty, Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Point != nil {
		t.Fatal("empty Portfolio must have no point")
	}
	assertLargePages(ctx, t, db, large)
	closes, err := v.PeriodCloses(ctx, edge, financial.PeriodDay, anchor.AddDate(0, 0, -7), anchor)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	gap := anchor.AddDate(0, 0, -4).In(loc).Format("2006-01-02")
	found := false
	for _, close := range closes {
		if close.Label == gap {
			found = true
			if close.Point != nil {
				t.Fatal("intentional empty day gained a point")
			}
		}
	}
	if !found {
		t.Fatal("empty calendar day missing from query")
	}
	events, err := v.BoundaryEvents(ctx, edge, anchor.AddDate(0, 0, -7), anchor.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	entries, exits := 0, 0
	for _, event := range events {
		if event.Direction == "contribution" {
			entries++
		}
		if event.Direction == "withdrawal" {
			exits++
		}
	}
	if entries == 0 || exits == 0 {
		t.Fatal("boundary entry/exit lost")
	}
	change, err := v.CustomValueChange(ctx, edge, anchor.AddDate(0, 0, -7), anchor)
	if err != nil {
		t.Fatal(err)
	}
	if change.Completeness != valuation.CompletenessIncomplete {
		t.Fatal("unvalued flow comparison must be incomplete")
	}
	reasons := strings.Join(change.Reasons, "\n")
	for _, reason := range []string{"unvalued_flow: DEMO-NOPRICE", "asset_exclusion_changed:"} {
		if !strings.Contains(reasons, reason) {
			t.Fatalf("missing comparison reason %s: %v", reason, change.Reasons)
		}
	}
	for _, tc := range []struct {
		name, kind, freshness, method string
		incomplete                    bool
	}{
		{"edge-stale", "cm_futures", valuation.FreshnessStale, valuation.MethodCalculated, false},
		{"edge-stale-incomplete", "spot", valuation.FreshnessStale, valuation.MethodCalculated, true},
		{"edge-unpriced", "spot", valuation.FreshnessFresh, valuation.MethodCalculated, true},
		{"edge-missing", "unified", valuation.FreshnessFresh, valuation.MethodMissing, true},
	} {
		row, err := v.AccountBalances(ctx, edge, accountID(tc.name, tc.kind))
		if err != nil {
			t.Fatal(err)
		}
		if row.Account.Method != tc.method || (row.Account.Completeness() == valuation.CompletenessIncomplete) != tc.incomplete {
			t.Fatalf("%s lost expected valuation state: %+v", tc.name, row.Account)
		}
		if tc.method != valuation.MethodMissing && row.Account.Freshness != tc.freshness {
			t.Fatalf("%s freshness changed", tc.name)
		}
		if tc.freshness == valuation.FreshnessStale && (row.Account.ReportObservedAt == nil || !row.Account.ReportObservedAt.Before(anchor.AddDate(0, 0, -2))) {
			t.Fatalf("%s stale observation time changed", tc.name)
		}
	}
	count := 0
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatalf("Alice-only ownership: %d %v", count, err)
	}
}

func assertLargePages(ctx context.Context, t *testing.T, db *sql.DB, p financial.PortfolioContext) {
	t.Helper()
	v := financial.NewService(db, valuation.NewService(db))
	seenPages := map[int]map[string]bool{}
	for _, offset := range []int{0, 50} {
		f := &financial.ListFilter{Limit: 50, Offset: offset}
		b, err := v.FlatBalances(ctx, p, f)
		if err != nil {
			t.Fatal(err)
		}
		s, err := v.FlatPositions(ctx, p, f)
		if err != nil {
			t.Fatal(err)
		}
		e, err := v.FlatEarnPositions(ctx, p, f)
		if err != nil {
			t.Fatal(err)
		}
		d, err := v.FlatDualPositions(ctx, p, f)
		if err != nil {
			t.Fatal(err)
		}
		c := catalog.NewService(db)
		l, err := c.ListAssets(ctx, catalog.ListFilter{PortfolioID: p.ID, Limit: 50, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		i, err := c.ListFuturesContracts(ctx, catalog.ListFilter{PortfolioID: p.ID, Limit: 50, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		totals := []int{b.Total, s.Total, e.Total, d.Total, l.Total, i.Total}
		lengths := []int{len(b.Items), len(s.Items), len(e.Items), len(d.Items), len(l.Items), len(i.Items)}
		keys := make([][]string, 6)
		for _, r := range b.Items {
			keys[0] = append(keys[0], r.AccountID.String()+"/"+r.AssetID.String())
		}
		for _, r := range s.Items {
			keys[1] = append(keys[1], r.AccountID.String()+"/"+r.PositionKey)
		}
		for _, r := range e.Items {
			keys[2] = append(keys[2], r.AccountID.String()+"/"+r.PositionKey)
		}
		for _, r := range d.Items {
			keys[3] = append(keys[3], r.AccountID.String()+"/"+r.PositionKey)
		}
		for _, r := range l.Items {
			keys[4] = append(keys[4], r.ID.String())
		}
		for _, r := range i.Items {
			keys[5] = append(keys[5], r.ID.String())
		}
		for n, rows := range keys {
			if seenPages[n] == nil {
				seenPages[n] = map[string]bool{}
			}
			for _, key := range rows {
				if seenPages[n][key] {
					t.Fatalf("table %d repeated row across pages", n)
				}
				seenPages[n][key] = true
			}
		}

		for n, total := range totals {
			if total < 60 || lengths[n] < 10 || offset == 0 && lengths[n] != 50 {
				t.Fatalf("table %d offset %d: total=%d rows=%d", n, offset, total, lengths[n])
			}
		}
	}
	a := audit.NewService(db)
	first, err := a.List(ctx, p.ID, &audit.ListFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasMore || first.NextCursor == nil || len(first.Items) != 50 {
		t.Fatal("audit second page unavailable")
	}
	second, err := a.List(ctx, p.ID, &audit.ListFilter{Limit: 50, Cursor: *first.NextCursor})
	if err != nil || len(second.Items) == 0 {
		t.Fatalf("audit second page: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, row := range first.Items {
		seen[row.ID] = true
	}
	for _, row := range second.Items {
		if seen[row.ID] {
			t.Fatal("audit cursor repeated an event")
		}
	}
}

func TestSeedManualRevisionKeysAndReportDecimalColumns(t *testing.T) {
	b := newBuilder()
	portfolioID, accountID := uuid.New(), uuid.New()
	for _, revision := range []string{"1", "2"} {
		b.add("manual_revisions", bin(portfolioID), bin(accountID), revision, "NULL")
	}
	if b.err != nil {
		t.Fatalf("distinct revisions collide: %v", b.err)
	}
	b.add("manual_revisions", bin(portfolioID), bin(accountID), "2", "NULL")
	if b.err == nil {
		t.Fatal("duplicate revision accepted")
	}
	for _, col := range []string{"reported_equity_usd", "adjusted_equity_usd", "available_equity_usd", "initial_margin_requirement_usd", "maintenance_margin_requirement_usd", "reported_initial_margin_ratio", "reported_maintenance_margin_ratio", "reported_margin_ratio", "spot_margin_unrealized_pnl_usd"} {
		b = newBuilder()
		columns := strings.Split(tableColumns["account_reports"], ", ")
		row := make([]string, len(columns))
		for i, name := range columns {
			row[i] = "NULL"
			if name == col {
				row[i] = "invalid"
			}
		}
		b.add("account_reports", row...)
		if b.err == nil {
			t.Fatalf("invalid %s skipped decimal validation", col)
		}
	}
}
