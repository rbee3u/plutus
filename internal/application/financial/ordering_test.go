package financial

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestPositionsGroupBaseBeforeContractKindAndExpiry(t *testing.T) {
	btc, eth, blank := "BTC", "ETH", " "
	near := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	far := near.AddDate(0, 3, 0)
	want := []PositionRow{
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearPerpetual, FuturesSymbol: "BTCUSDC", Side: "long", PlatformPositionRef: "usdc"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearPerpetual, FuturesSymbol: "BTCUSDT", Side: "long", PlatformPositionRef: "long-a"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearPerpetual, FuturesSymbol: "BTCUSDT", Side: "long", PlatformPositionRef: "long-b"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearPerpetual, FuturesSymbol: "BTCUSDT", Side: "short", PlatformPositionRef: "short"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearDelivery, FuturesSymbol: "Z_NEAR", ExpiryAt: &near, PlatformPositionRef: "near"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearDelivery, FuturesSymbol: "A_FAR", ExpiryAt: &far, PlatformPositionRef: "far"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindLinearDelivery, FuturesSymbol: "A_UNKNOWN", PlatformPositionRef: "unknown-expiry"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindInversePerpetual, FuturesSymbol: "BTCUSD_PERP", PlatformPositionRef: "inverse"},
		{BaseAssetSymbol: &btc, Kind: adapters.KindInverseDelivery, FuturesSymbol: "BTCUSD_DELIVERY", ExpiryAt: &near, PlatformPositionRef: "inverse-delivery"},
		{BaseAssetSymbol: &eth, Kind: adapters.KindLinearPerpetual, FuturesSymbol: "A_ETH", PlatformPositionRef: "eth"},
		{BaseAssetSymbol: &blank, Kind: adapters.KindInverseDelivery, FuturesSymbol: "A_NO_BASE", PlatformPositionRef: "missing-a"},
		{FuturesSymbol: "B_NO_BASE", PlatformPositionRef: "missing-b"},
	}
	got := slices.Clone(want)
	slices.Reverse(got)
	sortPositions(got, nil)
	for i := range want {
		if got[i].PlatformPositionRef != want[i].PlatformPositionRef {
			t.Fatalf("Position %d = %s, want %s", i, got[i].PlatformPositionRef, want[i].PlatformPositionRef)
		}
	}
	// Changed financial measures and arrival order must not move rows.
	for i := range got {
		got[i].Quantity = "999"
		got[i].Freshness = valuation.FreshnessStale
		got[i].UnrealizedPnl = new("-123")
	}
	slices.Reverse(got)
	sortPositions(got, nil)
	for i := range want {
		if got[i].PlatformPositionRef != want[i].PlatformPositionRef {
			t.Fatalf("financial changes moved Position %d", i)
		}
	}
}

func TestHoldingsHierarchyPrecedesAssetAndExpiry(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	ranks := map[uuid.UUID]int{first: 0, second: 1}
	balances := []BalanceRow{{AccountID: second, CanonicalAssetSymbol: "AAA"}, {AccountID: first, CanonicalAssetSymbol: "ZZZ"}}
	sortBalances(balances, ranks)
	if balances[0].AccountID != first {
		t.Fatal("Asset sort crossed the Account hierarchy")
	}
	near := time.Now()
	far := near.Add(time.Hour)
	duals := []DualPositionRow{
		{AccountID: second, DualPosition: adapters.DualPosition{ExpiryAt: &near, PlatformPositionRef: "second"}},
		{AccountID: first, DualPosition: adapters.DualPosition{PlatformPositionRef: "missing"}},
		{AccountID: first, DualPosition: adapters.DualPosition{ExpiryAt: &far, PlatformPositionRef: "far"}},
	}
	sortDualPositions(duals, ranks)
	if duals[0].PlatformPositionRef != "far" || duals[1].PlatformPositionRef != "missing" || duals[2].PlatformPositionRef != "second" {
		t.Fatalf("Dual Positions crossed hierarchy or lost missing-expiry order: %+v", duals)
	}
	positions := []PositionRow{{AccountID: second, FuturesSymbol: "AAA"}, {AccountID: first, FuturesSymbol: "ZZZ"}}
	sortPositions(positions, ranks)
	if positions[0].AccountID != first {
		t.Fatal("Position Symbol sort crossed the Account hierarchy")
	}
}

func TestHoldingsOrderUsesAssetBeforeCanonicalAsset(t *testing.T) {
	balances := []BalanceRow{
		{AssetSymbol: "ZZZ", CanonicalAssetSymbol: "AAA"},
		{AssetSymbol: "AAA", CanonicalAssetSymbol: "ZZZ"},
	}
	sortBalances(balances, nil)
	if balances[0].AssetSymbol != "AAA" {
		t.Fatal("Canonical Asset displaced the displayed Asset")
	}
	rows := []EarnPositionRow{
		{EarnPosition: adapters.EarnPosition{PlatformPositionRef: "z", Assets: []adapters.EarnAmount{{AssetSymbol: "ZZZ", CanonicalAssetSymbol: new("AAA")}}}},
		{EarnPosition: adapters.EarnPosition{PlatformPositionRef: "a", Assets: []adapters.EarnAmount{{AssetSymbol: "AAA", CanonicalAssetSymbol: new("ZZZ")}, {AssetSymbol: "BBB"}}}},
	}
	sortEarnPositions(rows, nil)
	if rows[0].PlatformPositionRef != "a" || len(rows[0].Assets) != 2 {
		t.Fatal("Earn ordering lost Asset priority or split a multi-Asset Position")
	}
}
