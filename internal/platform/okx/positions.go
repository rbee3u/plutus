package okx

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type openPosition struct {
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	PosSide  string `json:"posSide"`
	Pos      string `json:"pos"`
	MgnMode  string `json:"mgnMode"`
	AvgPx    string `json:"avgPx"`
	MarkPx   string `json:"markPx"`
	LiqPx    string `json:"liqPx"`
	Upl      string `json:"upl"`
	Lever    string `json:"lever"`
	Margin   string `json:"margin"`
	IMR      string `json:"imr"`
	Ccy      string `json:"ccy"`
}

// positions maps the open contract positions of the trading account. MARGIN rows are skipped:
// they are borrow-backed spot exposure whose value is already in the Balances' equity. An
// contract kind Plutus cannot store fails the Account instead of disappearing.
func (a *Adapter) positions(ctx context.Context, c *client, mode adapters.MarginMode) ([]adapters.Position, error) {
	var rows []openPosition
	if err := c.call(ctx, epPositions, &rows); err != nil {
		return nil, err
	}
	out := make([]adapters.Position, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		instType := strings.ToUpper(strings.TrimSpace(row.InstType))
		if instType == instTypeMargin {
			continue
		}
		market, ok := markets[instType]
		if !ok {
			return nil, adapters.NewError(adapters.ClassNormalization,
				fmt.Sprintf("OKX reports an open %s position in %s, which Plutus cannot store.", instType, row.InstID))
		}
		amount, err := adapters.Decimal(epPositions.String(), "pos of "+row.InstID, row.Pos)
		if err != nil {
			return nil, err
		}
		// OKX emits zero-size placeholder rows when a position was moved out of isolated margin.
		if amount.Sign() == 0 {
			continue
		}
		futuresContract, err := a.futuresContracts.lookup(ctx, c, market, row.InstID)
		if err != nil {
			return nil, err
		}
		position, err := mapPosition(row, amount, &futuresContract)
		if err != nil {
			return nil, err
		}
		if mode == adapters.MarginOKXPM && position.MarginSetting == "cross" {
			position.Margin, position.MarginBasis, position.MarginAssetSymbol = nil, "", ""
		}
		out = append(out, position)
	}
	return out, nil
}

func mapPosition(row *openPosition, amount *big.Rat, futuresContract *adapters.FuturesContract) (adapters.Position, error) {
	if _, err := optionalFields(epPositions.String(), row.InstID,
		field{"avgPx", row.AvgPx}, field{"markPx", row.MarkPx}, field{"liqPx", row.LiqPx},
		field{"upl", row.Upl}, field{"lever", row.Lever}, field{"margin", row.Margin}, field{"imr", row.IMR}); err != nil {
		return adapters.Position{}, err
	}
	// In net mode one row carries the netted position and its direction is the sign of pos; in
	// long/short mode OKX reports one row per side with a positive size.
	side := adapters.SideLong
	switch strings.ToLower(strings.TrimSpace(row.PosSide)) {
	case "short":
		side = adapters.SideShort
	case "long":
	case "net":
		if amount.Sign() < 0 {
			side = adapters.SideShort
		}
	default:
		return adapters.Position{}, adapters.Malformed(epPositions.String(), "posSide of "+row.InstID)
	}
	mode := "hedged"
	if strings.EqualFold(strings.TrimSpace(row.PosSide), "net") {
		mode = "one_way"
	}
	out := adapters.Position{
		PositionKey:        fmt.Sprintf("%s:%s", row.InstID, strings.ToLower(strings.TrimSpace(row.PosSide))),
		FuturesContract:    *futuresContract,
		Side:               side,
		PositionMode:       mode,
		SourceQuantity:     decimal.Format(decimal.Abs(amount)),
		SourceQuantityUnit: adapters.UnitContracts,
		EntryPrice:         adapters.Plain(row.AvgPx),
		MarkPrice:          adapters.Plain(row.MarkPx),
		LiquidationPrice:   adapters.NonZero(row.LiqPx),
		UnrealizedPnl:      adapters.Plain(row.Upl),
		// Portfolio margin reports no leverage for cross positions and fills the field with "0".
		Leverage: adapters.NonZero(row.Lever),
	}
	switch row.MgnMode {
	case "cross":
		out.MarginSetting, out.MarginBasis, out.MarginAssetSymbol, out.Margin = "cross", "initial", strings.TrimSpace(row.Ccy), adapters.Plain(row.IMR)
	case "isolated":
		out.MarginSetting, out.MarginBasis, out.MarginAssetSymbol, out.Margin = "isolated", "allocated", strings.TrimSpace(row.Ccy), adapters.Plain(row.Margin)
	}
	return out, nil
}
