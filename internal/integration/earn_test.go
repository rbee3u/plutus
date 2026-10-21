package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

type earnScenario struct{ failed, empty bool }

func (a *earnScenario) Validate(context.Context, adapter.Credentials, adapter.AccountStructure) (adapter.ValidationResult, error) {
	return adapter.ValidationResult{ExternalRef: "earn-test", MarginMode: adapter.MarginOKXPM}, nil
}

func (a *earnScenario) Collect(_ context.Context, _ adapter.Credentials, _ adapter.AccountStructure, types []adapter.AccountType) (adapter.Batch, error) {
	at := time.Now().UTC()
	out := adapter.Batch{CollectedAt: at, MarginMode: adapter.MarginOKXPM}
	for _, kind := range types {
		if a.failed {
			out.Reports = append(out.Reports, adapter.FailedReport(kind, adapter.NewError(adapter.ClassTemporary, "Synthetic refusal")))
			continue
		}
		r := adapter.AccountReport{AccountType: kind, Status: adapter.StatusSucceeded, CollectedAt: at}
		if !a.empty {
			r.Balances = []adapter.Balance{{AssetSymbol: "USDT", Equity: "40"}, {AssetSymbol: "USDC", Equity: "10"}}
			{
				p := adapter.NewEarnPosition("multi", "on_chain", "position", "principal")
				p.Assets = []adapter.EarnAmount{{AssetSymbol: "USDT", Amount: "15"}, {AssetSymbol: "USDC", Amount: "10"}}
				p.AddReward("USDT", "100", "estimated")
				p.Problems = append(p.Problems, "Estimated APR could not be read.")
				q := adapter.NewEarnPosition("fixed", "fixed", "position", "principal")
				q.Assets = []adapter.EarnAmount{{AssetSymbol: "USDT", Amount: "25"}}
				r.EarnPositions = []adapter.EarnPosition{p, q}
			}
		}
		out.Reports = append(out.Reports, r)
	}
	return out, nil
}

func TestEarnDetailsStaleEmptyAndIsolation(t *testing.T) {
	h := newHarness(t)
	a := &earnScenario{}
	h.registry.Register(adapter.PlatformOKX, a)
	id := h.createIdentity("Earn details", adapter.PlatformOKX, adapter.StructureOKXUnified, "earn")
	id = h.storeCredentials(id, "test")
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, id.ID, id.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
		t.Fatal(err)
	}
	first, _ := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	second, cycleID := h.runCycle()
	account := h.accountsByType(cycleID, id.ID)["earn"]
	if first.Point.TotalUSD != "50" || second.Point.TotalUSD != "50" || second.Point.BoundaryContributionUSD != "0" {
		t.Fatalf("descriptive detail changed value: %+v", second.Point)
	}
	view, err := h.financial.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || len(view.EarnPositions) != 2 {
		t.Fatalf("detail: %+v %v", view, err)
	}
	page, err := h.financial.FlatEarnPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "uSdT", Limit: 1, Offset: 1})
	if err != nil || page.Total != 2 || page.TotalUSD == nil || *page.TotalUSD != "50" || page.TotalCompleteness != "complete" || len(page.Items) != 1 || page.Items[0].PositionKey != "fixed" {
		t.Fatalf("sort/filter/page: %+v %v", page, err)
	}
	page, err = h.financial.FlatEarnPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "sDc", Limit: 50})
	if err != nil || page.Total != 1 || page.TotalUSD == nil || *page.TotalUSD != "25" || len(page.Items[0].Assets) != 2 || len(page.Items[0].Issues) != 1 {
		t.Fatalf("multiasset row: %+v %v", page, err)
	}
	if _, err := h.financial.AccountEarnPositions(h.ctx, financial.PortfolioContext{ID: uuid.New(), Timezone: "UTC"}, account.AccountID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("scope leak: %v", err)
	}
	a.failed = true
	stale, cycleID := h.runCycle()
	if stale.Point.TotalUSD != "50" {
		t.Fatal("failure removed funds")
	}
	view, err = h.financial.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || len(view.EarnPositions) != 2 || view.EarnPositions[0].Freshness != valuation.FreshnessStale || !view.EarnPositions[0].ReportObservedAt.Equal(*account.ReportObservedAt) {
		t.Fatalf("stale: %+v %v", view, err)
	}
	balances, err := h.financial.AccountBalances(h.ctx, pctx, account.AccountID)
	if err != nil || *view.Account.SourceReportID != *balances.Account.SourceReportID {
		t.Fatal("report mismatch")
	}
	point, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, cycleID)
	if err != nil || point.TotalUSD != "50" {
		t.Fatal("retry changed funds")
	}
	a.failed = false
	a.empty = true
	last, _ := h.runCycle()
	view, err = h.financial.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || len(view.EarnPositions) != 0 || last.Point.TotalUSD != "0" {
		t.Fatalf("empty: %+v %v", view, err)
	}
}
