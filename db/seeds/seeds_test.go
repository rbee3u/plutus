package seeds

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/valuation"
)

func TestBuild(t *testing.T) {
	t.Parallel()
	anchor := time.Date(2026, 9, 22, 10, 7, 0, 0, time.UTC)
	seed := BuildAt(anchor)
	if !strings.Contains(seed.SQL, "'alice'") || !strings.Contains(seed.SQL, "'bob'") {
		t.Fatal("both users must be seeded")
	}
	if strings.Contains(seed.SQL, "INSERT INTO valuation_points") {
		t.Fatal("seeds must not write Valuation Points")
	}
	if len(seed.Live) != 5 || len(seed.Portfolios) != 3 {
		t.Fatalf("expected five live Identities and three Portfolios, got %d and %d", len(seed.Live), len(seed.Portfolios))
	}
	if again := BuildAt(anchor); again.SQL != seed.SQL {
		t.Fatal("seed is not deterministic")
	}
}

func TestSeedIDIsStable(t *testing.T) {
	t.Parallel()
	if SeedID(kindUser, 1).String() != "01a0bdfc-2b00-7001-8000-000000000001" {
		t.Fatalf("unexpected id %s", SeedID(kindUser, 1))
	}
}

func TestMain(m *testing.M) {
	mysql.RunTestMain(m)
}

// Database cases run sequentially because each replaces the package's throwaway data.
func TestSeedsApplyAndRebuild(t *testing.T) {
	db := mysql.OpenTestDB(t)
	// D-4 is mid-week, giving the gap fixture a surrounding weekly start point.
	now := time.Date(2026, 9, 13, 10, 7, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		build func(time.Time) Seed
		check func(context.Context, *testing.T, *sql.DB, time.Time)
	}{
		{"development", BuildAt, assertDevelopmentSeed},
		{"anomaly fixture", buildEdgeCases, assertEdgeCases},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := mysql.DropAllTables(ctx, db); err != nil {
				t.Fatal(err)
			}
			if err := mysql.Up(db); err != nil {
				t.Fatal(err)
			}
			seed := tc.build(now)
			if err := mysql.ExecSQLStatements(ctx, db, seed.SQL); err != nil {
				t.Fatalf("apply: %v", err)
			}
			svc := valuation.NewService(db)
			total := 0
			for _, portfolioID := range seed.Portfolios {
				n, err := svc.Rebuild(ctx, portfolioID)
				if err != nil {
					t.Fatalf("rebuild %s: %v", portfolioID, err)
				}
				total += n
			}
			var points, unvalued int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM valuation_points").Scan(&points); err != nil {
				t.Fatal(err)
			}
			if points != total || points == 0 {
				t.Fatalf("rebuilt %d points but stored %d", total, points)
			}
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE method = 'missing' AND status = 'ok'").Scan(&unvalued); err != nil {
				t.Fatal(err)
			}
			if unvalued != 0 {
				t.Fatalf("%d successful reports were not valued", unvalued)
			}
			tc.check(ctx, t, db, now.UTC().Truncate(15*time.Minute))
		})
	}
}

func assertDevelopmentSeed(ctx context.Context, t *testing.T, db *sql.DB, now time.Time) {
	t.Helper()
	svc := valuation.NewService(db)
	p := valuation.PortfolioContext{ID: PortfolioAliceMain, Timezone: "Asia/Shanghai"}
	overview, err := svc.Overview(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Point == nil || overview.StaleAccountCount != 1 || overview.IncompleteAccountCount != 0 || overview.Completeness != valuation.CompletenessComplete {
		t.Fatalf("latest point must be complete with one stale Account: %+v", overview)
	}
	assertOnlyStaleIssue(t, overview.Issues, AccountAliceClassicCMFutures)
	positions, err := svc.FlatPositions(ctx, p, &valuation.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if positions.Total != 8 || len(positions.Items) != 8 || positions.Completeness != valuation.CompletenessComplete {
		t.Fatalf("expected eight known Futures: %+v", positions)
	}
	assertOnlyStaleIssue(t, positions.Issues, AccountAliceClassicCMFutures)
	stale := 0
	total := decimal.Zero()
	for _, row := range positions.Items {
		if row.Size == nil || row.SizeSymbol == nil || row.NotionalUSD == nil || row.ReportObservedAt == nil {
			t.Fatalf("%s: Size, notional, or observed time lost", row.Symbol)
		}
		total = decimal.Add(total, decimal.MustParse(*row.NotionalUSD))
		mode := "one_way"
		if row.Symbol == "ETHUSDT" {
			mode = "hedged"
		}
		if stringValue(row.PositionMode) != mode || stringValue(row.MarginMode) != "cross" || stringValue(row.MarginBasis) != "initial" || stringValue(row.MarginCurrency) != stringValue(row.SettlementSymbol) {
			t.Fatalf("%s/%s: source mode or margin provenance lost: %+v", row.AccountName, row.Symbol, row)
		}
		portfolio := row.AccountID == AccountAliceBinancePM
		if row.PortfolioMargin == nil || *row.PortfolioMargin != portfolio {
			t.Fatalf("%s: source Account Report Margin Mode lost", row.AccountName)
		}
		source := "reported"
		if portfolio || row.AccountID == AccountAliceClassicCMFutures {
			source = "calculated"
		}
		if row.SettleMargin == nil || stringValue(row.MarginSource) != source || row.MarginUnavailableReason != nil {
			t.Fatalf("%s/%s: incorrect margin source: %+v", row.AccountName, row.Symbol, row)
		}
		if decimal.MustParse(*row.Size).Sign() < 0 != (mode == "one_way" && row.Side == adapters.SideShort) || decimal.MustParse(row.Quantity).Sign() <= 0 {
			t.Fatalf("%s: source direction or Size sign changed", row.Symbol)
		}
		if row.AccountID == AccountAliceClassicCMFutures {
			stale++
			if row.Freshness != valuation.FreshnessStale || !row.ReportObservedAt.Before(now) {
				t.Fatalf("stale row must retain its earlier observation: %+v", row)
			}
			assertOnlyStaleIssue(t, row.Issues, row.AccountID)
		} else if row.Freshness != valuation.FreshnessFresh || len(row.Issues) != 0 {
			t.Fatalf("%s/%s: expected a healthy row: %+v", row.AccountName, row.Symbol, row)
		}
	}
	if stale != 1 || stringValue(positions.TotalUSD) != decimal.Format(total) {
		t.Fatalf("expected one stale row and complete notional sum, got %d and %v", stale, positions.TotalUSD)
	}

	// Historical fallback must retain its own mode and currency, too.
	day := localMidnight(now, 10, shanghai)
	var pointBytes []byte
	if err := db.QueryRowContext(ctx, `SELECT v.id FROM valuation_points v JOIN jobs j ON j.id = v.job_id
		WHERE j.portfolio_id = ? AND j.created_at >= ? AND j.created_at < ? ORDER BY j.created_at LIMIT 1`,
		PortfolioAliceMain[:], day, day.AddDate(0, 0, 1)).Scan(&pointBytes); err != nil {
		t.Fatal(err)
	}
	pointID, err := uuid.FromBytes(pointBytes)
	if err != nil {
		t.Fatal(err)
	}
	history, err := svc.AccountPositionsAt(ctx, p, AccountAliceBybitUnified, &pointID)
	if err != nil {
		t.Fatal(err)
	}
	if history.Account.Freshness != valuation.FreshnessStale || len(history.Positions) != 2 {
		t.Fatalf("historical rate-limited day lost its positions: %+v", history)
	}
	for _, row := range history.Positions {
		assertOnlyStaleIssue(t, row.Issues, AccountAliceBybitUnified)
		if row.PositionMode == nil || row.PortfolioMargin == nil || *row.PortfolioMargin || stringValue(row.MarginCurrency) != "USDT" {
			t.Fatalf("historical source metadata lost: %+v", row)
		}
	}
}

func assertOnlyStaleIssue(t *testing.T, issues []valuation.Issue, accountID uuid.UUID) {
	t.Helper()
	if len(issues) != 1 || issues[0].Code != "stale" || issues[0].AccountID == nil || *issues[0].AccountID != accountID {
		t.Fatalf("expected one stale issue for %s, got %+v", accountID, issues)
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
