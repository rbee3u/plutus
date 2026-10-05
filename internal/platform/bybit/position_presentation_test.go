package bybit

import (
	"math/big"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestModeAndInitialMarginProvenance(t *testing.T) {
	info := instrument{Symbol: "BTCUSD", ContractType: "InversePerpetual", BaseCoin: "BTC", QuoteCoin: "USD", SettleCoin: "BTC"}
	index := 0
	row := openPosition{Symbol: info.Symbol, PositionIdx: &index, Side: "Sell", PositionIM: "0.002"}
	for _, mode := range []adapters.MarginMode{adapters.MarginBybitCross, adapters.MarginBybitIsolated, adapters.MarginBybitPM} {
		position, err := mapPosition(&row, big.NewRat(200, 1), &info, mode)
		if err != nil || position.PositionMode != "one_way" || position.QuantityUnit != "quote" {
			t.Fatalf("position %+v %v", position, err)
		}
		if mode == adapters.MarginBybitPM {
			if position.Margin != nil {
				t.Fatal("Portfolio amount guessed")
			}
			continue
		}
		if position.MarginBasis != "initial" || position.MarginCurrency != "BTC" {
			t.Fatal("positionIM mislabeled as allocation")
		}
	}
	index = 1
	if _, err := mapPosition(&row, big.NewRat(200, 1), &info, adapters.MarginBybitCross); err == nil {
		t.Fatal("hedge index contradicting direction accepted")
	}
	index = 2
	position, err := mapPosition(&row, big.NewRat(200, 1), &info, adapters.MarginBybitCross)
	if err != nil || position.PositionMode != "hedged" {
		t.Fatalf("hedged %+v %v", position, err)
	}
	row.PositionIdx = nil
	if _, err := mapPosition(&row, big.NewRat(200, 1), &info, adapters.MarginBybitCross); err == nil {
		t.Fatal("missing positionIdx treated as one-way")
	}
}
