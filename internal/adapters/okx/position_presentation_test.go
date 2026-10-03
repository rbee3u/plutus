package okx

import (
	"math/big"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestPositionMarginUnitAndMode(t *testing.T) {
	instrument := adapters.Instrument{Symbol: "BTC-USD-SWAP", Kind: adapters.KindInversePerp, SettlementSymbol: "BTC"}
	row := openPosition{InstID: instrument.Symbol, PosSide: " NET ", MgnMode: "cross", IMR: "100", Margin: "0.3", Ccy: "BTC"}
	cross, err := mapPosition(&row, big.NewRat(-2, 1), &instrument)
	if err != nil || cross.PositionMode != "one_way" || cross.Side != "short" || cross.Margin == nil || *cross.Margin != "100" || cross.MarginCurrency != "USD" || cross.MarginBasis != "initial" {
		t.Fatalf("cross %+v %v", cross, err)
	}
	row.MgnMode, row.PosSide = "isolated", "long"
	isolated, err := mapPosition(&row, big.NewRat(2, 1), &instrument)
	if err != nil || isolated.PositionMode != "hedged" || isolated.Margin == nil || *isolated.Margin != "0.3" || isolated.MarginCurrency != "BTC" || isolated.MarginBasis != "allocated" {
		t.Fatalf("isolated %+v %v", isolated, err)
	}
}
