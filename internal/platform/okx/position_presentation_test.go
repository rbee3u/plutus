package okx

import (
	"math/big"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

func TestPositionMarginUnitAndMode(t *testing.T) {
	futuresContract := adapter.FuturesContract{Symbol: "BTC-USD-SWAP", Kind: adapter.KindInversePerpetual, SettlementAssetSymbol: "BTC"}
	row := openPosition{InstID: futuresContract.Symbol, PosSide: " NET ", MgnMode: "cross", IMR: "100", Margin: "0.3", Ccy: "BTC"}
	cross, err := mapPosition(&row, big.NewRat(-2, 1), &futuresContract)
	if err != nil || cross.PositionMode != "one_way" || cross.Side != "short" || cross.Margin == nil || *cross.Margin != "100" || cross.MarginAssetSymbol != "BTC" || cross.MarginBasis != "initial" {
		t.Fatalf("cross %+v %v", cross, err)
	}
	row.MgnMode, row.PosSide = "isolated", "long"
	isolated, err := mapPosition(&row, big.NewRat(2, 1), &futuresContract)
	if err != nil || isolated.PositionMode != "hedged" || isolated.Margin == nil || *isolated.Margin != "0.3" || isolated.MarginAssetSymbol != "BTC" || isolated.MarginBasis != "allocated" {
		t.Fatalf("isolated %+v %v", isolated, err)
	}
}
