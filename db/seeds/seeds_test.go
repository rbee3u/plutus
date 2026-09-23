package seeds

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/valuation"
)

func TestBuildProfiles(t *testing.T) {
	t.Parallel()

	anchor := time.Date(2026, 9, 22, 10, 7, 0, 0, time.UTC)
	for _, name := range Profiles {
		profile, err := BuildAt(name, anchor)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(profile.SQL, "'alice'") || !strings.Contains(profile.SQL, "'bob'") {
			t.Fatalf("%s: both users must be seeded", name)
		}
		if strings.Count(profile.SQL, "(") != strings.Count(profile.SQL, ")") {
			t.Fatalf("%s: unbalanced parentheses", name)
		}
		if strings.Contains(profile.SQL, "INSERT INTO valuation_points") {
			t.Fatalf("%s: seeds must not write Valuation Points", name)
		}
		if len(profile.Live) == 0 {
			t.Fatalf("%s: expected live identities", name)
		}
		again, err := BuildAt(name, anchor)
		if err != nil {
			t.Fatal(err)
		}
		if again.SQL != profile.SQL {
			t.Fatalf("%s: seed is not deterministic", name)
		}
	}
	if _, err := Build("nope"); err == nil {
		t.Fatal("unknown profile must fail")
	}
}

func TestSeedIDIsStable(t *testing.T) {
	t.Parallel()
	if SeedID(kindUser, 1).String() != "01a0bdfc-2b00-7001-8000-000000000001" {
		t.Fatalf("unexpected id %s", SeedID(kindUser, 1))
	}
}

const seedCheckDB = "plutus_seedcheck"

func TestProfilesApplyAndRebuild(t *testing.T) {
	dsn := mysql.TestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	admin, err := mysql.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+seedCheckDB+"`"); err != nil {
		t.Skipf("cannot create %s: %v", seedCheckDB, err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS `"+seedCheckDB+"`")
	})
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = seedCheckDB
	db, err := mysql.Open(cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, name := range Profiles {
		if err := mysql.DropAllTables(ctx, db); err != nil {
			t.Fatal(err)
		}
		if err := mysql.Up(db); err != nil {
			t.Fatal(err)
		}
		// A fixed past anchor keeps the calendar assertions deterministic: the gap day (D-4)
		// falls mid-week so the surrounding week has a start point.
		now := time.Date(2026, 9, 13, 10, 7, 0, 0, time.UTC)
		profile, err := BuildAt(name, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := mysql.ExecSQLStatements(ctx, db, profile.SQL); err != nil {
			t.Fatalf("%s: apply: %v", name, err)
		}
		svc := valuation.NewService(db)
		total := 0
		for _, portfolioID := range profile.Portfolios {
			n, err := svc.Rebuild(ctx, portfolioID)
			if err != nil {
				t.Fatalf("%s: rebuild %s: %v", name, portfolioID, err)
			}
			total += n
		}
		var points, incomplete int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(incomplete_account_count > 0), 0) FROM valuation_points").Scan(&points, &incomplete); err != nil {
			t.Fatal(err)
		}
		if points != total || points == 0 {
			t.Fatalf("%s: rebuilt %d points but stored %d", name, total, points)
		}
		var missing int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE method = 'missing' AND status = 'ok'").Scan(&missing); err != nil {
			t.Fatal(err)
		}
		if missing != 0 {
			t.Fatalf("%s: %d ok reports were not valued", name, missing)
		}
		if name == ProfileEdgeCases {
			if incomplete == 0 {
				t.Fatalf("%s: expected incomplete points", name)
			}
			var boundary int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM valuation_points WHERE boundary_contribution_usd > 0 OR boundary_withdrawal_usd > 0").Scan(&boundary); err != nil {
				t.Fatal(err)
			}
			if boundary == 0 {
				t.Fatalf("%s: expected boundary events", name)
			}
			assertEdgeCases(ctx, t, db, now.UTC().Truncate(15*time.Minute))
		}
		t.Logf("%s: %d valuation points (%d incomplete)", name, points, incomplete)
	}
}

// assertEdgeCases checks the accepted valuation rules against the edge-cases profile (technical design §13).
func assertEdgeCases(ctx context.Context, t *testing.T, db *sql.DB, now time.Time) {
	t.Helper()
	svc := valuation.NewService(db)
	p := valuation.PortfolioContext{ID: PortfolioAliceMain, Timezone: "Asia/Shanghai"}

	overview, err := svc.Overview(ctx, p)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.Point == nil || overview.TotalUSD == nil {
		t.Fatal("overview: expected a latest point")
	}
	if overview.StaleAccountCount == 0 {
		t.Fatal("overview: the revoked OKX trading account must be carried forward as stale")
	}
	if overview.IncompleteAccountCount == 0 || overview.Completeness != valuation.CompletenessIncomplete {
		t.Fatal("overview: the never-collected Bybit account must make the point incomplete")
	}
	codes := map[string]bool{}
	for _, issue := range overview.Issues {
		codes[issue.Code] = true
	}
	if !codes["missing"] || !codes["stale"] {
		t.Fatalf("overview: expected missing and stale issues, got %v", codes)
	}

	tree, err := svc.Tree(ctx, p)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	methods := map[uuid.UUID]string{}
	for _, platform := range tree.Platforms {
		for _, ident := range platform.Identities {
			for _, account := range ident.Accounts {
				methods[account.AccountID] = account.Method
			}
		}
	}
	if methods[AccountAliceBybitUnified] != valuation.MethodMissing {
		t.Fatalf("tree: Bybit unified should be missing, got %q", methods[AccountAliceBybitUnified])
	}
	if methods[AccountAliceOKXTrading] == valuation.MethodMissing || methods[AccountAliceOKXTrading] == "" {
		t.Fatalf("tree: OKX trading should keep its last successful report, got %q", methods[AccountAliceOKXTrading])
	}
	if _, present := methods[AccountAliceBinanceSpot]; present {
		t.Fatal("tree: the deselected Binance spot account must have left the boundary")
	}

	from := now.AddDate(0, 0, -9)
	events, err := svc.BoundaryEvents(ctx, p, from, now)
	if err != nil {
		t.Fatalf("boundary events: %v", err)
	}
	directions := map[uuid.UUID]string{}
	for _, event := range events {
		directions[event.AccountID] = event.Direction
		if event.ValueUSD == nil {
			t.Fatalf("boundary events: %s %s is unvalued", event.AccountName, event.Direction)
		}
		if event.IdentityName == "" || event.Platform == "" || event.IdentityID == uuid.Nil || event.Method == valuation.MethodMissing {
			t.Fatalf("boundary events: %s %s lacks identity, platform, or method: %+v", event.AccountName, event.Direction, event)
		}
	}
	if directions[AccountAliceBinanceFunding] != valuation.DirectionContribution || directions[AccountAliceBinanceSpot] != valuation.DirectionWithdrawal {
		t.Fatalf("boundary events: expected funding contribution and spot withdrawal, got %v", directions)
	}

	closes, err := svc.PeriodCloses(ctx, p, valuation.PeriodDay, from, now)
	if err != nil {
		t.Fatalf("period closes: %v", err)
	}
	gapDay := localMidnight(now, 4, shanghai).Format("2006-01-02")
	collectedDay := localMidnight(now, 6, shanghai).Format("2006-01-02")
	closed := map[string]bool{}
	for _, c := range closes {
		closed[c.Label] = c.Point != nil
	}
	if closed[gapDay] {
		t.Fatalf("period closes: the gap day %s must have no close", gapDay)
	}
	if !closed[collectedDay] {
		t.Fatalf("period closes: %s must have a close", collectedDay)
	}

	weeks, err := svc.ValueChanges(ctx, p, valuation.PeriodWeek, from, now)
	if err != nil {
		t.Fatalf("value changes: %v", err)
	}
	gapReported := false
	for _, change := range weeks {
		for _, reason := range change.Reasons {
			if strings.HasPrefix(reason, "days_without_points") {
				gapReported = true
			}
		}
	}
	if !gapReported {
		t.Fatal("value changes: the week containing the gap day must be incomplete")
	}

	custom, err := svc.CustomValueChange(ctx, p, localMidnight(now, 6, shanghai), now)
	if err != nil {
		t.Fatalf("custom value change: %v", err)
	}
	if custom.AmountUSD == nil || custom.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("custom value change: expected an incomplete amount, got %+v", custom)
	}
	unvalued := false
	for _, reason := range custom.Reasons {
		if strings.HasPrefix(reason, "unvalued_flow: TOKENX") {
			unvalued = true
		}
	}
	if !unvalued {
		t.Fatalf("custom value change: the TOKENX flow must be reported as unvalued, got %v", custom.Reasons)
	}
	if custom.BoundaryContributionsUSD == "0" || custom.BoundaryWithdrawalsUSD == "0" {
		t.Fatalf("custom value change: boundary events must be subtracted, got %+v", custom)
	}

	var btcValued, tokenValued int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capital_flows WHERE id = ? AND value_usd IS NOT NULL", FlowAliceBTC[:]).Scan(&btcValued); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capital_flows WHERE id = ? AND value_usd IS NOT NULL", FlowAliceTOKENX[:]).Scan(&tokenValued); err != nil {
		t.Fatal(err)
	}
	if btcValued != 1 || tokenValued != 0 {
		t.Fatalf("capital flows: BTC must be valued and TOKENX unvalued (got %d, %d)", btcValued, tokenValued)
	}
	var omitted, wbtcOmitted int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE JSON_SEARCH(omitted_json, 'one', 'TOKENX') IS NOT NULL").Scan(&omitted); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE JSON_SEARCH(omitted_json, 'one', 'WBTC') IS NOT NULL").Scan(&wbtcOmitted); err != nil {
		t.Fatal(err)
	}
	if omitted == 0 || wbtcOmitted != 0 {
		t.Fatalf("omitted balances: TOKENX must be omitted (%d) and manually mapped WBTC valued (%d omitted)", omitted, wbtcOmitted)
	}
}
