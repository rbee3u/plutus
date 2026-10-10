package kraken

import (
	"context"
	"testing"
)

func TestIsolatedAllocationExcludesOrdersAndProfit(t *testing.T) {
	r := &run{
		assets:   assetNames{"BTC": "BTC", "USD": "USD"},
		snapshot: &snapshot{Flex: multiWallet{Isolated: map[string]isolatedWallet{"PF_BTCUSD": {InitialWithoutOrders: "100"}}}},
		open: &openBook{
			positions:         []openPosition{{Symbol: "PF_BTCUSD", Side: "short", Size: "2", UnrealizedPnl: "50", PnlCurrency: "USD"}},
			nativeInstruments: map[string]nativeInstrument{"PF_BTCUSD": {Symbol: "PF_BTCUSD", Type: typeFlexible, Base: "BTC", Quote: "USD", ContractSize: "0.01"}},
			tickers:           map[string]futuresTicker{"PF_BTCUSD": {MarkPrice: "50000", Tag: "perpetual"}},
		},
	}
	positions, err := r.positions(context.Background(), true)
	if err != nil || len(positions) != 1 {
		t.Fatalf("positions %v %v", positions, err)
	}
	p := positions[0]
	if p.PositionMode != "one_way" || p.MarginMode != "isolated" || p.MarginBasis != "allocated" || p.MarginAssetSymbol != "USD" || p.Margin == nil || *p.Margin != "100" {
		t.Fatalf("allocation %+v", p)
	}
	r.snapshot.Flex.Isolated = map[string]isolatedWallet{}
	positions, err = r.positions(context.Background(), true)
	if err != nil || positions[0].MarginMode != "cross" || positions[0].Margin != nil {
		t.Fatalf("cross %v %v", positions, err)
	}
	r.snapshot.Flex.Isolated = nil
	positions, err = r.positions(context.Background(), true)
	if err != nil || positions[0].MarginMode != "" {
		t.Fatal("absent isolated map was guessed Cross")
	}
}
