package financial

import (
	"encoding/binary"
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
	service := NewService(db, valuation.NewService(db))
	change, err := service.CustomValueChange(ctx, p, start, ats[599])
	if err != nil {
		t.Fatal(err)
	}
	if change.AmountUSD == nil || *change.AmountUSD != "589" || change.Completeness != valuation.CompletenessIncomplete || len(change.BoundaryEvents) != 2 {
		t.Fatalf("comparison lost amounts or incompleteness: %+v", change)
	}
	counts := map[string]int{}
	for _, reason := range change.Reasons {
		counts[strings.SplitN(reason, ":", 2)[0]]++
	}
	for reason, want := range map[string]int{"calculation_version_changed": 2, "asset_mapping_changed": 2, "manual_balance_changed": 1, "boundary_incomplete": 1, "unvalued_boundary_event": 1} {
		if counts[reason] != want {
			t.Errorf("reason %s: got %d want %d (%v)", reason, counts[reason], want, change.Reasons)
		}
	}
	headers, err := sqlcgen.New(db).ListComparisonPointHeadersBetween(ctx, sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: start.Add(time.Microsecond), ToAt: ats[599]})
	if err != nil || len(headers) != 8 {
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
	if err != nil || len(monthly) != 1 || monthly[0].Completeness != valuation.CompletenessIncomplete || !strings.Contains(strings.Join(monthly[0].Reasons, ";"), "2025-01-05") {
		t.Fatalf("calendar coverage lost missing day: %+v %v", monthly, err)
	}
}
