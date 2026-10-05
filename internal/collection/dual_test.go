package collection

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/db/migrations"
	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/valuation"
)

type dualScenario struct {
	earn                      string
	failEarn, failDual, empty bool
}

func (a *dualScenario) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "dual-test", MarginMode: adapters.MarginOKXPM}, nil
}

func (a *dualScenario) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, types []adapters.AccountType) (adapters.Batch, error) {
	now := time.Now().UTC()
	batch := adapters.Batch{ObservedAt: now, MarginMode: adapters.MarginOKXPM}
	for _, kind := range types {
		if (kind == "earn" && a.failEarn) || (kind == adapters.TypeDualInvestment && a.failDual) {
			batch.Reports = append(batch.Reports, adapters.FailedReport(kind, adapters.NewError(adapters.ClassTemporary, "test failure")))
			continue
		}
		r := adapters.AccountReport{AccountType: kind, Status: adapters.StatusOK, ObservedAt: now}
		if kind == "earn" {
			r.Balances = []adapters.Balance{{Symbol: "USDT", Equity: a.earn}}
		} else if !a.empty {
			r.Balances = []adapters.Balance{{Symbol: "USDT", Equity: "40"}}
			r.DualPositions = []adapters.DualPosition{{PlatformPositionRef: "one", InvestSymbol: "USDT", Principal: "15", Status: "active"}, {PlatformPositionRef: "two", InvestSymbol: "USDT", Principal: "25", Status: "active"}}
			expiry := now.Add(24 * time.Hour)
			r.DualPositions[1].ExpiryAt = &expiry
		}
		batch.Reports = append(batch.Reports, r)
	}
	return batch, nil
}

func TestDualCutoverStaleAndRebuild(t *testing.T) {
	for _, firstFail := range []bool{false, true} {
		t.Run(map[bool]string{false: "successful cutover", true: "first Earn missing"}[firstFail], func(t *testing.T) {
			h := newHarness(t)
			adapter := &dualScenario{earn: "100"}
			h.registry.Register(adapters.PlatformOKX, adapter)
			item := h.createIdentity("Dual migration", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn")
			item = h.storeCredentials(item, "test")
			if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
				t.Fatal(err)
			}
			first, firstJob := h.runCycle()
			// Simulate a persisted report from the pre-split release.
			if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE account_reports SET scope_version=1 WHERE job_id=?", firstJob[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE accounts SET selected=TRUE WHERE identity_id=? AND account_type='dual_investment'", item.ID[:]); err != nil {
				t.Fatal(err)
			}
			adapter.earn = "60"
			adapter.failEarn = firstFail
			second, secondJob := h.runCycle()
			want := "100"
			if firstFail {
				want = "40"
			}
			if second.Point.TotalUSD != want || second.Point.BoundaryContributionUSD != "0" {
				t.Fatalf("cutover: %+v", second.Point)
			}
			accounts := h.accountsByType(secondJob, item.ID)
			earn, dual := accounts["earn"], accounts["dual_investment"]
			if firstFail && (earn.Method != valuation.MethodMissing || earn.SourceReportID != nil || earn.ValueUSD != nil) {
				t.Fatalf("legacy report reused: %+v", earn)
			}
			pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
			change, err := h.valuation.CustomValueChange(h.ctx, pctx, first.Point.EvaluatedAt, second.Point.EvaluatedAt)
			if err != nil || change.Completeness != valuation.CompletenessIncomplete || !strings.Contains(strings.Join(change.Reasons, ","), "account_scope_changed") {
				t.Fatalf("cross-scope change: %+v %v", change, err)
			}
			view, err := h.valuation.AccountDualPositions(h.ctx, pctx, dual.AccountID)
			if err != nil || len(view.DualPositions) != 2 || len(view.Issues) == 0 {
				t.Fatalf("dual positions and missing-term warnings: %+v %v", view, err)
			}

			for _, row := range view.DualPositions {
				if len(row.Issues) != 1 || !strings.Contains(row.Issues[0].Detail, "Dual position "+row.PlatformPositionRef+" ") {
					t.Fatalf("same-asset row lost its own issue: %+v", row)
				}
				if strings.Contains(row.Issues[0].Detail, "expiry") != (row.PlatformPositionRef == "one") {
					t.Fatalf("expiry issue crossed same-asset subscriptions: %+v", row)
				}
			}
			flat, err := h.valuation.FlatDualPositions(h.ctx, pctx, &valuation.ListFilter{Symbol: "USDT", Limit: 1, Offset: 1})
			if err != nil || flat.Total != 2 || len(flat.Items) != 1 || flat.Items[0].PlatformPositionRef != "one" {
				t.Fatalf("pagination: %+v %v", flat, err)
			}
			adapter.failEarn = false
			third, thirdJob := h.runCycle()
			thirdAccounts := h.accountsByType(thirdJob, item.ID)
			adapter.failEarn = true
			adapter.failDual = true
			fourth, fourthJob := h.runCycle()
			fourthAccounts := h.accountsByType(fourthJob, item.ID)
			change, err = h.valuation.CustomValueChange(h.ctx, pctx, third.Point.EvaluatedAt, fourth.Point.EvaluatedAt)
			if err != nil || change.Completeness != valuation.CompletenessComplete {
				t.Fatalf("post-cutover comparison remains incomplete: %+v %v", change, err)
			}
			if fourth.Point.TotalUSD != "100" || fourthAccounts["earn"].SourceReportID == nil || *fourthAccounts["earn"].SourceReportID != thirdAccounts["earn"].ReportID {
				t.Fatalf("compatible stale fallback: %+v", fourthAccounts)
			}
			balances, err := h.valuation.AccountBalances(h.ctx, pctx, dual.AccountID)
			if err != nil {
				t.Fatal(err)
			}
			stale, err := h.valuation.AccountDualPositions(h.ctx, pctx, dual.AccountID)
			if err != nil || len(stale.DualPositions) != 2 || stale.DualPositions[0].Freshness != valuation.FreshnessStale || *stale.Account.SourceReportID != *balances.Account.SourceReportID {
				t.Fatalf("stale alignment: %+v %v", stale, err)
			}
			if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := h.valuation.PointByJob(h.ctx, h.portfolioID, secondJob)
			if err != nil || rebuilt.TotalUSD != want || rebuilt.BoundaryContributionUSD != "0" {
				t.Fatalf("rebuild changed cutover: %+v %v", rebuilt, err)
			}
			old := h.accountsByType(firstJob, item.ID)["earn"]
			if old.ScopeVersion != 1 || old.ValueUSD == nil || *old.ValueUSD != "100" {
				t.Fatalf("old scope was reinterpreted: %+v", old)
			}
			adapter.failEarn = false
			adapter.failDual = false
			adapter.empty = true
			_, lastJob := h.runCycle()
			last := h.accountsByType(lastJob, item.ID)["dual_investment"]
			complete, err := h.valuation.AccountDualPositions(h.ctx, pctx, last.AccountID)
			if err != nil || len(complete.DualPositions) != 0 || last.ValueUSD == nil || *last.ValueUSD != "0" {
				t.Fatalf("successful empty report must clear positions: %+v %v", complete, err)
			}
		})
	}
}

func TestDualAccountBackfillPreservesSelections(t *testing.T) {
	h := newHarness(t)
	selected := h.createIdentity("Selected Earn", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn")
	unselected := h.createIdentity("Unselected Earn", adapters.PlatformBybit, adapters.StructureBybitUTA2, "funding")
	kraken := h.createIdentity("Unaffected Kraken", adapters.PlatformKraken, adapters.StructureKrakenSeparate, "earn")
	for _, item := range []identity.Identity{selected, unselected} {
		if _, err := h.collection.db.ExecContext(h.ctx, "DELETE FROM accounts WHERE identity_id=? AND account_type='dual_investment'", item.ID[:]); err != nil {
			t.Fatal(err)
		}
	}
	source, err := migrations.FS.ReadFile("20260929010100_dual_accounts.sql")
	if err != nil {
		t.Fatal(err)
	}
	apply := func() {
		t.Helper()
		up, _, _ := strings.Cut(string(source), "-- +goose Down")
		if err := mysql.ExecSQLStatements(h.ctx, h.collection.db, up); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	unchanged, err := h.identities.Get(h.ctx, h.portfolioID, kraken.ID)
	if err != nil || unchanged.Version != kraken.Version || len(unchanged.Accounts) != len(kraken.Accounts) {
		t.Fatalf("backfill changed Kraken: %+v %v", unchanged, err)
	}
	for _, tc := range []struct {
		item     identity.Identity
		selected bool
	}{{selected, true}, {unselected, false}} {
		current, err := h.identities.Get(h.ctx, h.portfolioID, tc.item.ID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, a := range current.Accounts {
			if a.AccountType == adapters.TypeDualInvestment {
				found = true
				if a.Selected != tc.selected {
					t.Fatal("selection not inherited")
				}
			}
		}
		if !found || current.Version != tc.item.Version+1 {
			t.Fatalf("backfill: %+v", current)
		}
	}
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE accounts SET selected=FALSE WHERE identity_id=? AND account_type='dual_investment'", selected.ID[:]); err != nil {
		t.Fatal(err)
	}
	apply()
	current, err := h.identities.Get(h.ctx, h.portfolioID, selected.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range current.Accounts {
		if a.AccountType == adapters.TypeDualInvestment && a.Selected {
			t.Fatal("rerun overwrote a later selection")
		}
	}
	if current.Version != selected.Version+1 {
		t.Fatal("rerun incremented configuration version")
	}
}
