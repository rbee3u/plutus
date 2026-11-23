package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// The leverage-warning demo has a USDT balance and a BTCUSDT short, but no BTC
// balance. Its BTC row must therefore be synthesized from the contract, with
// the USDT-settled position result kept off the BTC row.
func TestRiskExposureSynthesizesContractOnlyAsset(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("main", adapter.PlatformBinance, adapter.StructureBinancePM, "portfolio_margin")
	anchor := time.Now().UTC().Unix()
	item = h.storeCredentials(item, "plutus-demo-v1:leverage-warning:"+strconv.FormatInt(anchor, 10))
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID, RequestID: "test"}); err != nil || !outcome.OK {
		t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
	}
	_, cycleID := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	accountID := h.accountsByType(cycleID, item.ID)["portfolio_margin"].AccountID
	view, err := h.financial.AccountReport(h.ctx, pctx, accountID, &cycleID, true)
	if err != nil {
		t.Fatal(err)
	}

	var btc, usdt *financial.BalanceRow
	for i := range view.Balances {
		switch view.Balances[i].AssetSymbol {
		case "BTC":
			btc = &view.Balances[i]
		case "USDT":
			usdt = &view.Balances[i]
		}
	}
	if btc == nil || usdt == nil {
		t.Fatalf("balances = %+v, want BTC and USDT", view.Balances)
	}
	if !btc.ContractOnly {
		t.Fatal("BTC row was not marked contract-only")
	}
	if btc.Equity != "0" || btc.Liability == nil || *btc.Liability != "0" || btc.EquityUSD == nil || *btc.EquityUSD != "0" {
		t.Fatalf("BTC spot columns = %+v", btc)
	}
	if btc.UnrealizedPnl == nil || *btc.UnrealizedPnl != "0" {
		t.Fatalf("BTC PnL = %v, want 0 because the position settles in USDT", btc.UnrealizedPnl)
	}
	if btc.PriceSource != "contract" || btc.PriceSourceSymbol == nil || *btc.PriceSourceSymbol == "" {
		t.Fatalf("BTC price provenance = %q %v", btc.PriceSource, btc.PriceSourceSymbol)
	}
	if btc.PriceUSD == nil || btc.RiskExposure == nil {
		t.Fatalf("BTC source price or exposure missing: %+v", btc)
	}
	if usdt.RiskExposure == nil || *usdt.RiskExposure != "1000" {
		t.Fatalf("USDT exposure = %v, want 1000", usdt.RiskExposure)
	}

	var position *financial.PositionRow
	for i := range view.Positions {
		if view.Positions[i].BaseAssetSymbol != nil && *view.Positions[i].BaseAssetSymbol == "BTC" {
			position = &view.Positions[i]
			break
		}
	}
	if position == nil || position.Size == nil {
		t.Fatalf("positions = %+v", view.Positions)
	}
	size, _ := decimal.Parse(*position.Size)
	want := decimal.Format(decimal.Abs(size))
	if *btc.RiskExposure != "-"+want {
		t.Fatalf("BTC exposure = %s, want -%s", *btc.RiskExposure, want)
	}
	if btc.PriceUSD == nil || position.MarkPrice == nil || *btc.PriceUSD != *position.MarkPrice {
		t.Fatalf("fallback price = %v, want mark %v", btc.PriceUSD, position.MarkPrice)
	}

	flat, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Platform: "binance", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range flat.Items {
		if row.AccountID == accountID && row.AssetSymbol == "BTC" && row.ContractOnly {
			found = true
			if row.RiskExposure == nil || *row.RiskExposure != *btc.RiskExposure {
				t.Fatalf("flat BTC exposure = %v, want %s", row.RiskExposure, *btc.RiskExposure)
			}
		}
	}
	if !found {
		t.Fatalf("flat balances missing the contract-only BTC row: %+v", flat.Items)
	}
	if flat.TotalUSD == nil || *flat.TotalUSD != "1000" {
		t.Fatalf("filtered total = %v, want 1000; synthetic zero equity must not change it", flat.TotalUSD)
	}
}
