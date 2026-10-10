package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
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
	batch := adapters.Batch{CollectedAt: now, MarginMode: adapters.MarginOKXPM}
	for _, kind := range types {
		if (kind == "earn" && a.failEarn) || (kind == adapters.TypeDualInvestment && a.failDual) {
			batch.Reports = append(batch.Reports, adapters.FailedReport(kind, adapters.NewError(adapters.ClassTemporary, "test failure")))
			continue
		}
		r := adapters.AccountReport{AccountType: kind, Status: adapters.StatusSucceeded, CollectedAt: now}
		if kind == "earn" {
			r.Balances = []adapters.Balance{{AssetSymbol: "USDT", Equity: a.earn}}
			p := adapters.NewEarnPosition("earn", "flexible", "position", "principal")
			p.Assets = []adapters.EarnAmount{{AssetSymbol: "USDT", Amount: r.Balances[0].Equity}}
			r.EarnPositions = []adapters.EarnPosition{p}
		} else if !a.empty {
			r.Balances = []adapters.Balance{{AssetSymbol: "USDT", Equity: "40"}}
			r.DualPositions = []adapters.DualPosition{{PositionKey: "one", PrincipalAssetSymbol: "USDT", Principal: "15", Status: "active"}, {PositionKey: "two", PrincipalAssetSymbol: "USDT", Principal: "25", Status: "active"}}
			expiry := now.Add(24 * time.Hour)
			r.DualPositions[1].ExpiryAt = &expiry
		}
		batch.Reports = append(batch.Reports, r)
	}
	return batch, nil
}

func TestDualSeparateAccountsStaleAndRetry(t *testing.T) {
	for _, firstFail := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh Earn", true: "stale Earn"}[firstFail], func(t *testing.T) {
			h := newHarness(t)
			adapter := &dualScenario{earn: "60"}
			h.registry.Register(adapters.PlatformOKX, adapter)
			item := h.createIdentity("Dual accounts", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn", "dual_investment")
			item = h.storeCredentials(item, "test")
			if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
				t.Fatal(err)
			}
			first, firstCycle := h.runCycle()
			adapter.earn = "60"
			adapter.failEarn = firstFail
			second, secondCycle := h.runCycle()
			want := "100"
			if second.Point.TotalUSD != want || second.Point.BoundaryContributionUSD != "0" {
				t.Fatalf("cutover: %+v", second.Point)
			}
			accounts := h.accountsByType(secondCycle, item.ID)
			earn, dual := accounts["earn"], accounts["dual_investment"]
			if firstFail && (earn.Freshness != valuation.FreshnessStale || earn.SourceReportID == nil || earn.ValueUSD == nil) {
				t.Fatalf("stale report not reused: %+v", earn)
			}
			pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
			change, err := h.financial.CustomValueChange(h.ctx, pctx, first.Point.EvaluatedAt, second.Point.EvaluatedAt)
			if err != nil || change.Completeness != valuation.CompletenessComplete {
				t.Fatalf("cross-scope change: %+v %v", change, err)
			}
			view, err := h.financial.AccountDualPositions(h.ctx, pctx, dual.AccountID)
			if err != nil || len(view.DualPositions) != 2 || len(view.Issues) == 0 {
				t.Fatalf("dual positions and missing-term warnings: %+v %v", view, err)
			}

			for _, row := range view.DualPositions {
				if len(row.Issues) != 1 || !strings.Contains(row.Issues[0].Detail, "Dual position "+row.PositionKey+" ") {
					t.Fatalf("same-asset row lost its own issue: %+v", row)
				}
				if strings.Contains(row.Issues[0].Detail, "expiry") != (row.PositionKey == "one") {
					t.Fatalf("expiry issue crossed same-asset subscriptions: %+v", row)
				}
			}
			flat, err := h.financial.FlatDualPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "USDT", Limit: 1, Offset: 1})
			if err != nil || flat.Total != 2 || len(flat.Items) != 1 || flat.Items[0].PositionKey != "one" {
				t.Fatalf("pagination: %+v %v", flat, err)
			}
			adapter.failEarn = false
			third, thirdCycle := h.runCycle()
			thirdAccounts := h.accountsByType(thirdCycle, item.ID)
			adapter.failEarn = true
			adapter.failDual = true
			fourth, fourthCycle := h.runCycle()
			fourthAccounts := h.accountsByType(fourthCycle, item.ID)
			change, err = h.financial.CustomValueChange(h.ctx, pctx, third.Point.EvaluatedAt, fourth.Point.EvaluatedAt)
			if err != nil || change.Completeness != valuation.CompletenessComplete {
				t.Fatalf("post-cutover comparison remains incomplete: %+v %v", change, err)
			}
			if fourth.Point.TotalUSD != "100" || fourthAccounts["earn"].SourceReportID == nil || *fourthAccounts["earn"].SourceReportID != thirdAccounts["earn"].ReportID {
				t.Fatalf("compatible stale fallback: %+v", fourthAccounts)
			}
			balances, err := h.financial.AccountBalances(h.ctx, pctx, dual.AccountID)
			if err != nil {
				t.Fatal(err)
			}
			stale, err := h.financial.AccountDualPositions(h.ctx, pctx, dual.AccountID)
			if err != nil || len(stale.DualPositions) != 2 || stale.DualPositions[0].Freshness != valuation.FreshnessStale || *stale.Account.SourceReportID != *balances.Account.SourceReportID {
				t.Fatalf("stale alignment: %+v %v", stale, err)
			}
			rebuilt, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, secondCycle)
			if err != nil || rebuilt.TotalUSD != want || rebuilt.BoundaryContributionUSD != "0" {
				t.Fatalf("retry changed cutover: %+v %v", rebuilt, err)
			}
			old := h.accountsByType(firstCycle, item.ID)["earn"]
			if old.ValueUSD == nil || *old.ValueUSD != "60" {
				t.Fatalf("old scope was reinterpreted: %+v", old)
			}
			adapter.failEarn = false
			adapter.failDual = false
			adapter.empty = true
			_, lastCycle := h.runCycle()
			last := h.accountsByType(lastCycle, item.ID)["dual_investment"]
			complete, err := h.financial.AccountDualPositions(h.ctx, pctx, last.AccountID)
			if err != nil || len(complete.DualPositions) != 0 || last.ValueUSD == nil || *last.ValueUSD != "0" {
				t.Fatalf("successful empty report must clear positions: %+v %v", complete, err)
			}
		})
	}
}
