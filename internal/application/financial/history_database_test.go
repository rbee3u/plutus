package financial

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestGroupedHistoryAcrossHeaderPages(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	// Header 10,000 falls inside New York's repeated hour. Its bucket must
	// survive both the SQL cursor and the next batch's carried final point.
	cut := time.Date(2025, 11, 2, 5, 15, 0, 0, time.UTC)
	start := cut.Add(-9999 * 30 * time.Minute)
	historySummaries(t, db, p, start, start.Add(10005*30*time.Minute))
	service := NewService(db, valuation.NewService(db))
	for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
		t.Run(zone, func(t *testing.T) {
			p := p
			p.Timezone = zone
			for _, count := range []int{9999, 10000, 10001, 10005} {
				to := start.Add(time.Duration(count) * 30 * time.Minute).Add(-time.Microsecond)
				headers, err := sqlcgen.New(db).ListValuationPointHeadersBetween(t.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: to})
				if err != nil || len(headers) != count {
					t.Fatalf("headers: %d %v", len(headers), err)
				}
				for _, granularity := range []string{"hour", "day"} {
					want := appendSample([]sqlcgen.ListValuationPointHeadersBetweenRow{}, headers, granularity, p.location())
					got, err := service.PointsWithLimit(t.Context(), p, start, to, granularity, len(want))
					if err != nil || len(got) != len(want) {
						t.Fatalf("%s/%d: points=%d want=%d error=%v", granularity, count, len(got), len(want), err)
					}
					for i, point := range got {
						if string(point.CycleID[:]) != string(want[i].CycleID) || !point.EvaluatedAt.Equal(want[i].EvaluatedAt) || point.TotalUSD != "100" {
							t.Fatalf("%s/%d: point %d changed: %+v", granularity, count, i, point)
						}
					}
					if _, err := service.PointsWithLimit(t.Context(), p, start, to, granularity, len(want)-1); !errors.Is(err, ErrSeriesLimit) {
						t.Fatalf("%s/%d lost exact output limit: %v", granularity, count, err)
					}
				}
				_, err = service.Points(t.Context(), p, start, to, "raw")
				if (count > 10000 && !errors.Is(err, ErrSeriesLimit)) || (count <= 10000 && err != nil) {
					t.Fatalf("raw/%d changed limit: %v", count, err)
				}
			}
		})
	}
	// Public raw pages retain their exact total, descending order, and the
	// point immediately following each page, including a partial final page.
	to := start.Add(10005 * 30 * time.Minute)
	for _, offset := range []int{9990, 10000, 10005} {
		page, err := service.PointsPage(t.Context(), p, start, to, offset, 10)
		want := min(10, 10005-offset)
		if err != nil || page.Total != 10005 || len(page.Items) != want || (page.PrecedingPoint != nil) != (offset+10 < 10005) {
			t.Fatalf("offset %d: %+v %v", offset, page, err)
		}
		for i, point := range page.Items {
			if !point.EvaluatedAt.Equal(start.Add(time.Duration(10004-offset-i) * 30 * time.Minute)) {
				t.Fatalf("offset %d point %d changed order: %+v", offset, i, point)
			}
		}
	}
	stats.Queries.Store(0)
	if _, err := service.PointsWithLimit(t.Context(), p, start, to, "hour", 1); !errors.Is(err, ErrSeriesLimit) || stats.Queries.Load() != 1 {
		t.Fatalf("over-limit series did not stop after its first header page: reads=%d error=%v", stats.Queries.Load(), err)
	}
	// An equal-time successor on the next header page must win the cycle-ID
	// tie, even when the query ends exactly at that evaluation time.
	tied := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	for _, query := range []string{
		"INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)",
		"INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES(?,?,?,200,0,0,0,1,'expired')",
	} {
		args := []any{p.ID[:], tied[:], cut}
		if strings.Contains(query, "collection_cycles") {
			args = append(args, cut, cut, cut)
		}
		if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
		p.Timezone = zone
		points, err := service.Points(t.Context(), p, start, cut, "hour")
		if err != nil || len(points) == 0 || points[len(points)-1].CycleID != tied || points[len(points)-1].TotalUSD != "200" {
			t.Fatalf("%s: equal-time page boundary changed: points=%d error=%v", zone, len(points), err)
		}
	}
}

func TestComparisonPoliciesAcrossAssetBatches(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	start := time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC)
	historySummaries(t, db, p, start, start.Add(48*time.Hour+time.Minute))
	q := sqlcgen.New(db)
	rows, err := q.ListValuationPointHeadersBetween(t.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: start.Add(49 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	endpoints := []sqlcgen.ListValuationPointHeadersBetweenRow{rows[0], rows[48], rows[96]}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 257 {
		asset := uuid.New()
		symbol := fmt.Sprintf("ASSET%03d", i)
		exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance',?,'USD',1,?,?)", p.ID[:], asset[:], symbol, start, start)
		for j, point := range endpoints {
			treatment := "absent"
			if j == 1 {
				treatment = "excluded"
			}
			exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment) VALUES(?,?,?,?,?)", p.ID[:], asset[:], point.EvaluatedAt, point.CycleID, treatment)
		}
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,1,TRUE,'Reviewed',?),(?,?,2,FALSE,NULL,?)", p.ID[:], asset[:], endpoints[1].EvaluatedAt.Add(-time.Microsecond), p.ID[:], asset[:], endpoints[2].EvaluatedAt.Add(-time.Microsecond))
	}
	stats.Queries.Store(0)
	changes, err := NewService(db, valuation.NewService(db)).ValueChanges(t.Context(), p, PeriodDay, start.Add(11*time.Hour), start.Add(59*time.Hour))
	if err != nil || len(changes) != 2 {
		t.Fatalf("changes: %+v %v", changes, err)
	}
	if queries := stats.Queries.Load(); queries > 20 {
		t.Fatalf("asset batches amplified reads: %d", queries)
	}
	for _, change := range changes {
		if change.Completeness != valuation.CompletenessIncomplete || len(change.Issues) != 257 {
			t.Fatalf("policy state lost across batches: %+v", change)
		}
		for i, issue := range change.Issues {
			if issue.Code != "asset_exclusion_changed" || issue.Symbol != fmt.Sprintf("ASSET%03d", i) {
				t.Fatalf("policy issue %d changed ordering: %+v", i, issue)
			}
		}
	}
}

func TestSparseHistoryPreservesExpiredComparisonEvidence(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "history-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %+v %v", owned, err)
	}
	p := PortfolioContext{ID: owned[0].ID, Timezone: "UTC"}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	identity, account, asset := uuid.New(), uuid.New(), uuid.New()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO identities(portfolio_id,id,platform,display_name,display_name_key,account_structure,connection_status,version,created_at,updated_at) VALUES(?,?,'binance','history','history','classic','unconfigured',1,?,?)", p.ID[:], identity[:], start, start)
	exec("INSERT INTO accounts(portfolio_id,id,identity_id,account_type,selected) VALUES(?,?,?,'manual',TRUE)", p.ID[:], account[:], identity[:])
	exec("INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','BTC','BTC',3,?,?)", p.ID[:], asset[:], start, start)
	ids := make([]uuid.UUID, 600)
	ats := make([]time.Time, len(ids))
	for i := range ids {
		binary.BigEndian.PutUint64(ids[i][8:], uint64(i+1))
		at := start.Add(time.Duration(i) * time.Hour)
		if i == 18 {
			at = ats[17] // A -> B -> A at equal instants still has two transitions.
		}
		if i >= 96 {
			at = at.Add(24 * time.Hour) // A whole calendar day has no points.
		}
		ats[i] = at
		version := 1
		if i == 17 {
			version = 2
		}
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", p.ID[:], ids[i][:], at, at, at, at)
		exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES(?,?,?,?,1,0,0,?,'expired')", p.ID[:], ids[i][:], at, i+1, version)
	}
	for _, change := range []struct {
		at     time.Time
		cycle  uuid.UUID
		symbol string
	}{{ats[99], ids[99], "BTC"}, {ats[100], ids[100], "ETH"}, {ats[101], ids[101], "BTC"}} {
		exec("INSERT INTO calculated_asset_changes(portfolio_id,asset_id,evaluated_at,cycle_id,treatment,canonical_asset_symbol) VALUES(?,?,?,?,'included',?)", p.ID[:], asset[:], change.at, change.cycle[:], change.symbol)
	}
	exec("INSERT INTO manual_balance_changes(portfolio_id,cycle_id,account_id) VALUES(?,?,?)", p.ID[:], ids[512][:], account[:])
	exec("INSERT INTO boundary_events(portfolio_id,cycle_id,account_id,direction,valuation_method,value_usd,valuation_complete) VALUES(?,?,?,'contribution','calculated',10,TRUE)", p.ID[:], ids[515][:], account[:])
	exec("INSERT INTO boundary_events(portfolio_id,cycle_id,account_id,direction,valuation_method,value_usd,valuation_complete) VALUES(?,?,?,'withdrawal','missing',NULL,FALSE)", p.ID[:], ids[519][:], account[:])
	for i, offset := range []int{510, 520, 530, 540} {
		excluded := i%2 == 0
		var reason any
		if excluded {
			reason = "Reviewed"
		}
		exec("INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,?,?,?)", p.ID[:], asset[:], i+1, excluded, reason, ats[offset])
	}
	service := NewService(db, valuation.NewService(db))
	change, err := service.CustomValueChange(ctx, p, start, ats[599])
	if err != nil {
		t.Fatal(err)
	}
	if change.AmountUSD == nil || *change.AmountUSD != "589" || change.Completeness != valuation.CompletenessIncomplete || len(change.BoundaryEvents) != 2 {
		t.Fatalf("comparison lost amounts or incompleteness: %+v", change)
	}
	counts := map[string]int{}
	for _, reason := range change.Issues {
		counts[reason.Code]++
	}
	for reason, want := range map[string]int{"calculation_version_changed": 2, "asset_mapping_changed": 2, "manual_balance_changed": 1, "boundary_incomplete": 1, "unvalued_boundary_event": 1} {
		if counts[reason] != want {
			t.Errorf("reason %s: got %d want %d (%v)", reason, counts[reason], want, change.Issues)
		}
	}
	// Independent requests must agree with the shared window, including policy
	// round trips, empty periods, equal-time cycles and endpoint issue ordering.
	for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
		p.Timezone = zone
		calendar, err := service.ValueChanges(ctx, p, PeriodDay, start, ats[599])
		if err != nil {
			t.Fatal(err)
		}
		for _, batched := range calendar {
			independent, err := service.CustomValueChange(ctx, p, batched.From.Add(-time.Microsecond), batched.To.Add(-time.Microsecond))
			if err != nil {
				t.Fatal(err)
			}
			independent.PeriodKind, independent.Label = batched.PeriodKind, batched.Label
			independent.From, independent.To = batched.From, batched.To
			if !reflect.DeepEqual(batched, independent) {
				t.Fatalf("shared window changed %s:\ngot  %+v\nwant %+v", batched.Label, batched, independent)
			}
		}
	}
	p.Timezone = "UTC"

	headers, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(ctx, sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start.Add(time.Microsecond), ToAt: ats[599]})
	// Version transitions are now derived from the request's shared timeline.
	if err != nil || len(headers) != 6 {
		t.Fatalf("sparse candidates: %d %v", len(headers), err)
	}
	events, err := service.BoundaryEvents(ctx, p, start, ats[599])
	if err != nil || len(events) != 2 || events[0].CycleID != ids[515] || events[1].CycleID != ids[519] {
		t.Fatalf("boundary timeline: %+v %v", events, err)
	}
	closes, err := service.PeriodCloses(ctx, p, PeriodDay, start, start.AddDate(0, 0, 6))
	if err != nil || len(closes) != 6 {
		t.Fatalf("closes: %+v %v", closes, err)
	}
	if closes[0].Point == nil || closes[0].Point.CycleID != ids[23] || closes[4].Point != nil || closes[4].Completeness != valuation.CompletenessIncomplete || closes[5].Point == nil || closes[5].Point.CycleID != ids[119] {
		t.Fatalf("calendar closing boundary or missing day changed: %+v", closes)
	}
	monthly, err := service.PeriodCloses(ctx, p, PeriodMonth, start, start.AddDate(0, 0, 1))
	if err != nil || len(monthly) != 1 || monthly[0].Completeness != valuation.CompletenessIncomplete || !strings.Contains(fmt.Sprint(monthly[0].Issues), "2025-01-05") {
		t.Fatalf("calendar coverage lost missing day: %+v %v", monthly, err)
	}
}

func TestCalendarListsStartAtRetainedHistoryAndPreserveGaps(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "calendar-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %+v %v", owned, err)
	}
	p := PortfolioContext{ID: owned[0].ID, Timezone: "Asia/Shanghai"}
	loc := p.location()
	day := time.Date(2025, 1, 12, 0, 0, 0, 0, loc)
	service := NewService(db, valuation.NewService(db))
	assertLabels := func(from, to time.Time, want []string) {
		t.Helper()
		closes, err := service.PeriodCloses(ctx, p, PeriodDay, from, to)
		if err != nil {
			t.Fatal(err)
		}
		changes, err := service.ValueChanges(ctx, p, PeriodDay, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if len(closes) != len(want) || len(changes) != len(want) {
			t.Fatalf("rows: closes=%d changes=%d want=%v", len(closes), len(changes), want)
		}
		for i, label := range want {
			if closes[i].Label != label || changes[i].Label != label {
				t.Fatalf("row %d: close=%s change=%s want=%s", i, closes[i].Label, changes[i].Label, label)
			}
			if label == "2025-01-13" && (closes[i].Point != nil || closes[i].Completeness != valuation.CompletenessIncomplete || changes[i].Completeness != valuation.CompletenessIncomplete) {
				t.Fatalf("missing day lost: close=%+v change=%+v", closes[i], changes[i])
			}
		}
	}
	assertLabels(day.AddDate(-2, 0, 0), day.AddDate(0, 0, 4), nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, offset := range []int{0, 2, 3} {
		cycle := uuid.New()
		at := day.AddDate(0, 0, offset).Add(30 * time.Minute).UTC()
		incomplete := 0
		if offset == 0 {
			incomplete = 1 // An incomplete first point still establishes history.
		}
		exec("INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','schedule',?,?,?,?)", p.ID[:], cycle[:], at, at, at, at)
		exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version,detail_state) VALUES(?,?,?,100,1,0,?,1,'expired')", p.ID[:], cycle[:], at, incomplete)
	}
	// The two-year leading blank range must not consume the 400-period limit.
	assertLabels(day.AddDate(-2, 0, 0), day.AddDate(0, 0, 4), []string{"2025-01-12", "2025-01-13", "2025-01-14", "2025-01-15"})
	assertLabels(day.AddDate(0, 0, -7), day, nil)
	// No point in this query: the gap remains because earlier history exists.
	assertLabels(day.AddDate(0, 0, 1), day.AddDate(0, 0, 2), []string{"2025-01-13"})
	assertLabels(day.AddDate(0, 0, 1), day.AddDate(0, 0, 3), []string{"2025-01-13", "2025-01-14"})
	// Whole periods remain intact, but pre-history days are not missing coverage.
	monthly, err := service.PeriodCloses(ctx, p, PeriodMonth, day.AddDate(0, -1, 0), day.AddDate(0, 0, 4))
	if err != nil || len(monthly) != 1 || monthly[0].Label != "2025-01" {
		t.Fatalf("monthly rows: %+v %v", monthly, err)
	}
	if !monthly[0].PeriodStart.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, loc)) || (len(monthly[0].Issues) != 1 || monthly[0].Issues[0].Code != "days_without_points" || !strings.HasPrefix(monthly[0].Issues[0].Detail, "2025-01-13, 2025-01-16")) {
		t.Fatalf("monthly history coverage: %+v", monthly[0])
	}
	weekly, err := service.PeriodCloses(ctx, p, PeriodWeek, day.AddDate(0, 0, -7), day.AddDate(0, 0, 1))
	if err != nil || len(weekly) != 1 || weekly[0].Completeness != valuation.CompletenessIncomplete || strings.Contains(fmt.Sprint(weekly[0].Issues), "days_without_points") {
		t.Fatalf("first incomplete point or pre-history weekly coverage lost: %+v %v", weekly, err)
	}
	// Retention can leave older physical rows while its read boundary advances.
	exec("UPDATE portfolios SET history_before=? WHERE id=?", day.AddDate(0, 0, 2).UTC(), p.ID[:])
	assertLabels(day.AddDate(0, 0, -7), day.AddDate(0, 0, 4), []string{"2025-01-14", "2025-01-15"})
}

func TestComparisonAvailabilityAndKnownZero(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "compare-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolios: %+v %v", owned, err)
	}
	p := PortfolioContext{ID: owned[0].ID, Timezone: "UTC"}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	for i, id := range ids {
		at := start.Add(time.Duration(i) * time.Hour)
		if _, err := db.ExecContext(ctx, "INSERT INTO collection_cycles(portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES(?,?,'succeeded','user',?,?,?,?)", p.ID[:], id[:], at, at, at, at); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,100,1,0,0,1)", p.ID[:], id[:], at); err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(db, valuation.NewService(db))
	for _, window := range [][2]time.Time{{start.Add(-time.Hour), start.Add(time.Hour)}, {start, start.Add(time.Minute)}, {start.Add(-2 * time.Hour), start.Add(-time.Hour)}} {
		change, err := service.CustomValueChange(ctx, p, window[0], window[1])
		if err != nil {
			t.Fatal(err)
		}
		if change.Comparable || change.AmountUSD != nil || change.ContributionsUSD != nil || change.WithdrawalsUSD != nil || change.BoundaryContributionsUSD != nil || change.BoundaryWithdrawalsUSD != nil || len(change.Issues) == 0 || change.FlowsCompleteness != valuation.CompletenessIncomplete {
			t.Fatalf("unknown comparison encoded a zero: %+v", change)
		}
	}
	change, err := service.CustomValueChange(ctx, p, start, start.Add(time.Hour))
	if err != nil || !change.Comparable || change.AmountUSD == nil || *change.AmountUSD != "0" || change.ContributionsUSD == nil || *change.ContributionsUSD != "0" || change.FlowsCompleteness != valuation.CompletenessComplete {
		t.Fatalf("known zero: %+v %v", change, err)
	}
	flow := uuid.New()
	if _, err := db.ExecContext(ctx, "INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,effective_at,pricing_method,pricing_attempted_at,calculation_version,version,created_at,updated_at) VALUES(?,?,'contribution','DEMO',2,?,'unavailable',?,1,1,?,?)", p.ID[:], flow[:], start.Add(30*time.Minute), start, start, start); err != nil {
		t.Fatal(err)
	}
	change, err = service.CustomValueChange(ctx, p, start, start.Add(time.Hour))
	if err != nil || !change.Comparable || change.AmountUSD == nil || *change.AmountUSD != "0" || change.Completeness != valuation.CompletenessIncomplete || change.FlowsCompleteness != valuation.CompletenessIncomplete || len(change.Issues) != 1 || change.Issues[0].Code != "unvalued_flow" || change.Issues[0].FlowID == nil || *change.Issues[0].FlowID != flow {
		t.Fatalf("unknown flow lost identity or known values: %+v %v", change, err)
	}
	// Committed point summaries can still be incomplete without losing the known difference.
	if _, err := db.ExecContext(ctx, "UPDATE valuation_points SET incomplete_account_count=1 WHERE portfolio_id=? AND cycle_id=?", p.ID[:], ids[1][:]); err != nil {
		t.Fatal(err)
	}
	change, err = service.CustomValueChange(ctx, p, start, start.Add(time.Hour))
	if err != nil || !change.Comparable || change.AmountUSD == nil || *change.AmountUSD != "0" || change.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("partial comparison: %+v %v", change, err)
	}
}

func TestComparisonPolicyWindowMatchesVersionSelection(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	p := performancePortfolio(t, db)
	other := performancePortfolio(t, db)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	historySummaries(t, db, p, start, start.Add(3*time.Hour))
	q := sqlcgen.New(db)
	headers, err := q.ListValuationPointHeadersBetween(t.Context(), sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start, ToAt: start.Add(3 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	points := make([]valuation.Point, 0, len(headers))
	for _, h := range headers {
		id, err := uuid.FromBytes(h.CycleID)
		if err != nil {
			t.Fatal(err)
		}
		points = append(points, valuation.Point{CycleID: id, EvaluatedAt: h.EvaluatedAt})
	}
	for _, owner := range []uuid.UUID{p.ID, other.ID} {
		for assetIndex := range 3 {
			asset := uuid.New()
			if _, err := db.ExecContext(t.Context(), "INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance',?,'USD',1,?,?)", owner[:], asset[:], fmt.Sprintf("POLICY%d", assetIndex), start, start); err != nil {
				t.Fatal(err)
			}
			// Nonmonotonic times, equal-time versions, and changes exactly at
			// endpoints exercise strict cutoff and highest-version selection.
			for version, offset := range []time.Duration{-time.Hour, 0, time.Hour, 30 * time.Minute, time.Hour, 90 * time.Minute, 90 * time.Minute, 150 * time.Minute} {
				excluded := (version+assetIndex)%2 == 0
				var reason any
				if excluded {
					reason = "Reviewed"
				}
				if _, err := db.ExecContext(t.Context(), "INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,?,?,?)", owner[:], asset[:], version+1, excluded, reason, start.Add(offset)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Selection must follow cycle creation time, independent of
	// evaluation order. Duplicate cutoff times must see the same rules.
	if _, err := db.ExecContext(t.Context(), "UPDATE collection_cycles SET created_at=? WHERE portfolio_id=? AND id=?", start.Add(30*time.Minute), p.ID[:], points[4].CycleID[:]); err != nil {
		t.Fatal(err)
	}
	prepared := &comparisonRead{excluded: map[uuid.UUID]map[uuid.UUID]bool{}, assets: map[uuid.UUID]map[uuid.UUID]comparisonAsset{}}
	stats.Queries.Store(0)
	stats.Rows.Store(0)
	if err := NewService(db, valuation.NewService(db)).comparisonPolicies(t.Context(), p, prepared, points); err != nil {
		t.Fatal(err)
	}
	if stats.Rows.Load() > 35 {
		t.Fatalf("unrelated policy rows loaded: %d", stats.Rows.Load())
	}
	for _, point := range points {
		cycle, err := q.GetCollectionCycle(t.Context(), sqlcgen.GetCollectionCycleParams{PortfolioID: p.ID[:], ID: point.CycleID[:]})
		if err != nil {
			t.Fatal(err)
		}
		legacy, err := q.ListAssetExclusionsBefore(t.Context(), sqlcgen.ListAssetExclusionsBeforeParams{PortfolioID: p.ID[:], EffectiveAt: cycle.CreatedAt})
		if err != nil {
			t.Fatal(err)
		}
		want, decided := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
		for _, row := range legacy {
			asset, err := uuid.FromBytes(row.AssetID)
			if err != nil {
				t.Fatal(err)
			}
			if !decided[asset] {
				decided[asset] = true
				if row.Excluded {
					want[asset] = true
				}
			}
		}
		if !reflect.DeepEqual(prepared.excluded[point.CycleID], want) {
			t.Fatalf("cutoff %s: got %v want %v", cycle.CreatedAt, prepared.excluded[point.CycleID], want)
		}
	}
}
