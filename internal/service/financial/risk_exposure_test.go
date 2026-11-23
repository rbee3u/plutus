package financial

import (
	"math/big"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestRiskExposurePlatformGateCoversSupportedScopes(t *testing.T) {
	for _, platform := range []adapter.Platform{adapter.PlatformBinance, adapter.PlatformOKX, adapter.PlatformBybit, adapter.PlatformKraken} {
		if !riskExposurePlatforms[platform] {
			t.Errorf("%s is missing from the risk exposure rollout", platform)
		}
	}
}

func TestRiskAssetsNetsLinearAndInverseContracts(t *testing.T) {
	positions := []PositionRow{
		{
			FuturesSymbol:         "BTCUSDT",
			Kind:                  adapter.KindLinearPerpetual,
			Side:                  adapter.SideLong,
			BaseAssetSymbol:       new("BTC"),
			QuoteAssetSymbol:      new("USDT"),
			SettlementAssetSymbol: new("USDT"),
			Size:                  new("1"),
			MarkPrice:             new("60000"),
			NotionalUSD:           new("60000"),
			UnrealizedPnl:         new("5"),
		},
		{
			FuturesSymbol:         "BTCUSD_PERP",
			Kind:                  adapter.KindInversePerpetual,
			Side:                  adapter.SideShort,
			BaseAssetSymbol:       new("BTC"),
			QuoteAssetSymbol:      new("USD"),
			SettlementAssetSymbol: new("BTC"),
			Size:                  new("80000"),
			MarkPrice:             new("60000"),
			NotionalUSD:           new("80000"),
			UnrealizedPnl:         new("0.0125"),
		},
	}

	assets := riskAssets(positions)
	asset := assets["BTC"]
	if asset == nil {
		t.Fatal("BTC exposure was not aggregated")
	}
	want := new(big.Rat).SetFrac64(-1, 3)
	if asset.contract.Cmp(want) != 0 {
		t.Fatalf("contract = %s, want -1/3", decimal.Format(asset.contract))
	}
	if got := stringValue(riskPnl(asset)); got != "0.0125" {
		t.Fatalf("coin-settled PnL = %q, want 0.0125", got)
	}
	mark, symbol := riskPrice(asset)
	if mark == nil || mark.Cmp(big.NewRat(60000, 1)) != 0 || symbol != "BTCUSD_PERP" {
		t.Fatalf("fallback = %v %q, want the larger BTCUSD notional", mark, symbol)
	}

	rows := []BalanceRow{{AssetSymbol: "BTC", Equity: "0"}}
	applyRiskExposureToRows(rows, assets)
	if rows[0].RiskExposure == nil {
		t.Fatal("known exposure was not rendered")
	}
	if *rows[0].RiskExposure != decimal.Format(want) {
		t.Fatalf("row exposure = %s, want %s", *rows[0].RiskExposure, decimal.Format(want))
	}
	if !rows[0].ContractOnly {
		t.Fatal("a zero-spot asset with contracts was not marked contract-only")
	}
	if rows[0].PriceSource != "contract" || stringValue(rows[0].PriceSourceSymbol) != "BTCUSD_PERP" {
		t.Fatalf("price fallback = %q %v", rows[0].PriceSource, rows[0].PriceSourceSymbol)
	}
}

func TestRiskExposureUsesNetEquityAndKeepsUnknownWhole(t *testing.T) {
	assets := riskAssets([]PositionRow{{
		FuturesSymbol:         "ADAUSDT",
		Kind:                  adapter.KindLinearPerpetual,
		Side:                  adapter.SideShort,
		BaseAssetSymbol:       new("ADA"),
		QuoteAssetSymbol:      new("USDT"),
		SettlementAssetSymbol: new("USDT"),
		Size:                  new("100"),
		MarkPrice:             new("0.5"),
		NotionalUSD:           new("50"),
	}})
	rows := []BalanceRow{
		{AssetSymbol: "ADA", Equity: "150"},
		{AssetSymbol: "LTC", Equity: "2"},
	}
	applyRiskExposureToRows(rows, assets)
	if *rows[0].RiskExposure != "50" {
		t.Fatalf("net spot and contract = %q, want 50", *rows[0].RiskExposure)
	}
	if *rows[1].RiskExposure != "2" || rows[1].ContractOnly {
		t.Fatalf("spot-only row = %+v", rows[1])
	}

	unknown := riskAssets([]PositionRow{{
		FuturesSymbol:         "BTCUSD_PERP",
		Kind:                  adapter.KindInversePerpetual,
		Side:                  adapter.SideLong,
		BaseAssetSymbol:       new("BTC"),
		QuoteAssetSymbol:      new("USD"),
		SettlementAssetSymbol: new("BTC"),
		Size:                  new("100"),
		MarkPrice:             nil,
	}})
	rows = []BalanceRow{{AssetSymbol: "BTC", Equity: "0.5"}}
	applyRiskExposureToRows(rows, unknown)
	if rows[0].RiskExposure != nil || rows[0].RiskExposureUnavailableReason != RiskExposureUnknownPosition {
		t.Fatalf("inverse without mark = %+v, want unknown", rows[0])
	}
}

func TestRiskPnlOnlyIncludesPositionsSettledInTheRowCoin(t *testing.T) {
	assets := riskAssets([]PositionRow{
		{
			Kind:                  adapter.KindLinearPerpetual,
			Side:                  adapter.SideLong,
			BaseAssetSymbol:       new("ETH"),
			QuoteAssetSymbol:      new("USDT"),
			SettlementAssetSymbol: new("USDT"),
			Size:                  new("1"),
			MarkPrice:             new("2000"),
			UnrealizedPnl:         new("25"),
		},
		{
			Kind:                  adapter.KindInversePerpetual,
			Side:                  adapter.SideShort,
			BaseAssetSymbol:       new("ETH"),
			QuoteAssetSymbol:      new("USD"),
			SettlementAssetSymbol: new("ETH"),
			Size:                  new("4000"),
			MarkPrice:             new("2000"),
			UnrealizedPnl:         new("0.01"),
		},
	})
	if got := stringValue(riskPnl(assets["ETH"])); got != "0.01" {
		t.Fatalf("Pnl = %q, want the coin-settled 0.01", got)
	}

	linearOnly := riskAssets([]PositionRow{{
		Kind:                  adapter.KindLinearPerpetual,
		Side:                  adapter.SideLong,
		BaseAssetSymbol:       new("ETH"),
		QuoteAssetSymbol:      new("USDT"),
		SettlementAssetSymbol: new("USDT"),
		Size:                  new("1"),
		MarkPrice:             new("2000"),
		UnrealizedPnl:         new("25"),
	}})
	if got := stringValue(riskPnl(linearOnly["ETH"])); got != "0" {
		t.Fatalf("U-margined Pnl = %q, want 0 on the base row", got)
	}
}
