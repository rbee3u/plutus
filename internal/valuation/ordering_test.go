package valuation

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestPositionsGroupBaseBeforeContractKindAndExpiry(t *testing.T) {
	btc, eth, blank := "BTC", "ETH", " "
	near := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	far := near.AddDate(0, 3, 0)
	want := []PositionRow{
		{BaseSymbol: &btc, Kind: adapters.KindLinearPerp, Symbol: "BTCUSDC", Side: "long", PlatformPositionRef: "usdc"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearPerp, Symbol: "BTCUSDT", Side: "long", PlatformPositionRef: "long-a"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearPerp, Symbol: "BTCUSDT", Side: "long", PlatformPositionRef: "long-b"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearPerp, Symbol: "BTCUSDT", Side: "short", PlatformPositionRef: "short"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearDeliv, Symbol: "Z_NEAR", ExpiryAt: &near, PlatformPositionRef: "near"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearDeliv, Symbol: "A_FAR", ExpiryAt: &far, PlatformPositionRef: "far"},
		{BaseSymbol: &btc, Kind: adapters.KindLinearDeliv, Symbol: "A_UNKNOWN", PlatformPositionRef: "unknown-expiry"},
		{BaseSymbol: &btc, Kind: adapters.KindInversePerp, Symbol: "BTCUSD_PERP", PlatformPositionRef: "inverse"},
		{BaseSymbol: &btc, Kind: adapters.KindInverseDlv, Symbol: "BTCUSD_DELIVERY", ExpiryAt: &near, PlatformPositionRef: "inverse-delivery"},
		{BaseSymbol: &eth, Kind: adapters.KindLinearPerp, Symbol: "A_ETH", PlatformPositionRef: "eth"},
		{BaseSymbol: &blank, Kind: adapters.KindInverseDlv, Symbol: "A_NO_BASE", PlatformPositionRef: "missing-a"},
		{Symbol: "B_NO_BASE", PlatformPositionRef: "missing-b"},
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
		got[i].Freshness = FreshnessStale
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
	balances := []BalanceRow{{AccountID: second, AssetSymbol: "AAA"}, {AccountID: first, AssetSymbol: "ZZZ"}}
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
	positions := []PositionRow{{AccountID: second, Symbol: "AAA"}, {AccountID: first, Symbol: "ZZZ"}}
	sortPositions(positions, ranks)
	if positions[0].AccountID != first {
		t.Fatal("Position Symbol sort crossed the Account hierarchy")
	}
}
