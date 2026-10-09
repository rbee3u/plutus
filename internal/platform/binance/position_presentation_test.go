package binance

import (
	"math/big"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestPositionMarginSourceAndMode(t *testing.T) {
	futuresContract := adapters.FuturesContract{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerpetual, SettlementSymbol: "USDT"}
	row := positionRisk{Symbol: "BTCUSDT", PositionSide: "BOTH", PositionAmt: "-2", MarginType: "cross", MarginAsset: "USDT", PositionInitialMargin: "100", IsolatedWallet: "300"}
	cross, err := mapPosition(marketUM, epUMPositionRisk, &row, big.NewRat(-2, 1), &futuresContract)
	if err != nil || cross.PositionMode != "one_way" || cross.Side != "short" || cross.Margin == nil || *cross.Margin != "100" || cross.MarginBasis != "initial" {
		t.Fatalf("cross %+v %v", cross, err)
	}
	row.MarginType, row.PositionSide = "isolated", "SHORT"
	isolated, err := mapPosition(marketUM, epUMPositionRisk, &row, big.NewRat(-2, 1), &futuresContract)
	if err != nil || isolated.PositionMode != "hedged" || isolated.Margin == nil || *isolated.Margin != "300" || isolated.MarginBasis != "allocated" {
		t.Fatalf("isolated %+v %v", isolated, err)
	}
	portfolio, err := mapPosition(marketUM, epPMUMPositionRisk, &row, big.NewRat(-2, 1), &futuresContract)
	if err != nil || portfolio.Margin != nil || portfolio.MarginMode != "cross" {
		t.Fatalf("portfolio %+v %v", portfolio, err)
	}
}

func TestCoinMarginJoinRefusesDifferentQuantityAndDuplicates(t *testing.T) {
	rows := []positionRisk{{Symbol: "BTCUSD_PERP", PositionSide: "BOTH", PositionAmt: "2", PositionInitialMargin: "999"}}
	source := []futuresAccountPosition{{Symbol: "BTCUSD_PERP", PositionSide: "BOTH", PositionAmt: "3", PositionInitialMargin: "1"}}
	withCMRequirements(rows, source)
	if rows[0].PositionInitialMargin != "" {
		t.Fatal("non-atomic quantities were joined")
	}
	source[0].PositionAmt = "2"
	withCMRequirements(rows, append(source, source...))
	if rows[0].PositionInitialMargin != "" {
		t.Fatal("ambiguous positions were joined")
	}
	withCMRequirements(rows, source)
	if rows[0].PositionInitialMargin != "1" {
		t.Fatal("matching position requirement lost")
	}
	isolated := true
	source[0].Isolated, source[0].IsolatedWallet = &isolated, "0.003"
	rows[0].MarginType = ""
	withCMRequirements(rows, source)
	futuresContract := adapters.FuturesContract{Symbol: rows[0].Symbol, Kind: adapters.KindInversePerpetual, SettlementSymbol: "BTC"}
	position, err := mapPosition(marketCM, epCMPositionRisk, &rows[0], big.NewRat(2, 1), &futuresContract)
	if err != nil || position.Margin == nil || *position.Margin != "0.003" || position.MarginBasis != "allocated" {
		t.Fatalf("actual account isolatedWallet lost: %+v %v", position, err)
	}
}
