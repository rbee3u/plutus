package integration

import (
	"testing"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestHoldingsSubstringFiltersBeforePagination(t *testing.T) {
	h := newHarness(t)
	// Exercise the production composition root through collection and reads.
	h.collection = h.services.Collection
	item := h.createIdentity("Search", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures", "cm_futures", "dual_investment")
	item = h.storeCredentials(item, "search-filters")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable: %+v %v", result, err)
	}
	_, job := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	full, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || full.Total != 2 {
		t.Fatalf("positions: %+v %v", full, err)
	}
	page, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "uSd", Offset: 1, Limit: 1})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].FuturesSymbol != "BTCUSD_PERP" || *page.TotalUSD != *full.TotalUSD {
		t.Fatalf("substring pagination or total: %+v %v", page, err)
	}
	missing, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "no-match"})
	if err != nil || missing.Total != 0 || *missing.TotalUSD != *full.TotalUSD || missing.Completeness != full.Completeness {
		t.Fatalf("empty filter changed boundary totals: %+v %v", missing, err)
	}
	balances, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Limit: 500})
	if err != nil || balances.Total == 0 {
		t.Fatalf("balances: %+v %v", balances, err)
	}
	sum := decimal.Zero()
	for _, row := range balances.Items {
		if row.AssetSymbol != "USDT" || row.EquityUSD == nil {
			t.Fatalf("unexpected filtered balance: %+v", row)
		}
		sum = decimal.Add(sum, decimal.ParseOrZero(*row.EquityUSD))
	}
	if balances.TotalUSD == nil || decimal.ParseOrZero(*balances.TotalUSD).Cmp(sum) != 0 {
		t.Fatal("Balance total did not follow the filtered rows")
	}
	pageBalances, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Offset: 1, Limit: 1})
	if err != nil || len(pageBalances.Items) != 1 || pageBalances.Total != balances.Total || *pageBalances.TotalUSD != *balances.TotalUSD {
		t.Fatalf("Balance search pagination: %+v %v", pageBalances, err)
	}
	duals, err := h.financial.FlatDualPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Limit: 1})
	if err != nil || duals.Total == 0 || len(duals.Items) != 1 {
		t.Fatalf("Dual quote substring: %+v %v", duals, err)
	}
	for _, account := range h.accountsByType(job, item.ID) {
		if _, err := h.financial.AccountBalances(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.financial.AccountPositions(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.financial.AccountDualPositions(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
	}
	if overview, err := h.financial.Overview(h.ctx, pctx); err != nil || len(overview.ByPlatform) != 1 || len(overview.ByIdentity) != 1 {
		t.Fatalf("Overview groups: %+v %v", overview, err)
	}
}
