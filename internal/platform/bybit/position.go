package bybit

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Linear positions require one read per settlement coin; inverse and options
// each use one list. Include options so unsupported holdings fail explicitly.
var positionLists = []struct{ category, settleCoin string }{
	{"linear", "USDT"},
	{"linear", "USDC"},
	{"inverse", ""},
	{"option", ""},
}

var contractKinds = map[string]string{
	"LinearPerpetual":  adapter.KindLinearPerpetual,
	"InversePerpetual": adapter.KindInversePerpetual,
	"LinearFutures":    adapter.KindLinearDelivery,
	"InverseFutures":   adapter.KindInverseDelivery,
}

type openPosition struct {
	Symbol        string `json:"symbol"`
	PositionIdx   *int   `json:"positionIdx"`
	Side          string `json:"side"`
	Size          string `json:"size"`
	AvgPrice      string `json:"avgPrice"`
	MarkPrice     string `json:"markPrice"`
	LiqPrice      string `json:"liqPrice"`
	UnrealisedPnl string `json:"unrealisedPnl"`
	Leverage      string `json:"leverage"`
	PositionIM    string `json:"positionIM"`
}

type nativeInstrument struct {
	Symbol       string `json:"symbol"`
	ContractType string `json:"contractType"`
	BaseCoin     string `json:"baseCoin"`
	QuoteCoin    string `json:"quoteCoin"`
	SettleCoin   string `json:"settleCoin"`
	DeliveryTime string `json:"deliveryTime"`
}

// Read complete lists before retaining positions; fetch metadata only for held
// categories. Unsupported options fail the account instead of disappearing.
func (a *Adapter) positions(ctx context.Context, c *client, mode adapter.MarginMode) ([]adapter.Position, error) {
	open := map[string][]openPosition{}
	sizes := map[string][]*big.Rat{}
	for _, list := range positionLists {
		params := url.Values{"category": {list.category}}
		if list.settleCoin != "" {
			params.Set("settleCoin", list.settleCoin)
		}
		rows, err := pages[openPosition](ctx, c, epPositions, params)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			size, err := adapter.Decimal(epPositions.String(), "size of "+rows[i].Symbol, rows[i].Size)
			if err != nil {
				return nil, err
			}
			if size.Sign() == 0 {
				continue
			}
			if list.category == "option" {
				return nil, adapter.NewError(adapter.ClassNormalization,
					fmt.Sprintf("Bybit reports an open option position in %s, which Plutus cannot store.", rows[i].Symbol))
			}
			open[list.category] = append(open[list.category], rows[i])
			sizes[list.category] = append(sizes[list.category], size)
		}
	}
	var out []adapter.Position
	seen := map[string]bool{}
	for _, category := range []string{"linear", "inverse"} {
		if len(open[category]) == 0 {
			continue
		}
		nativeInstruments, err := readNativeInstruments(ctx, c, category)
		if err != nil {
			return nil, err
		}
		for i := range open[category] {
			row := &open[category][i]
			info, ok := nativeInstruments[row.Symbol]
			if !ok {
				return nil, adapter.NewError(adapter.ClassNormalization,
					fmt.Sprintf("%s: Bybit lists no %s contract %s for an open position.", epInstruments, category, row.Symbol))
			}
			position, err := mapPosition(row, sizes[category][i], &info, mode)
			if err != nil {
				return nil, err
			}
			if seen[position.PositionKey] {
				return nil, adapter.Malformed(epPositions.String(), "a unique position "+position.PositionKey)
			}
			seen[position.PositionKey] = true
			out = append(out, position)
		}
	}
	return out, nil
}

func readNativeInstruments(ctx context.Context, c *client, category string) (map[string]nativeInstrument, error) {
	rows, err := pages[nativeInstrument](ctx, c, epInstruments, url.Values{"category": {category}})
	if err != nil {
		return nil, err
	}
	out := make(map[string]nativeInstrument, len(rows))
	for _, row := range rows {
		out[row.Symbol] = row
	}
	return out, nil
}

// Bybit size is positive: base units for linear, USD face value for inverse.
// PM has no position-level leverage, margin, or liquidation price. Cross
// liquidation price estimates liquidation of the whole account.
func mapPosition(row *openPosition, size *big.Rat, info *nativeInstrument, mode adapter.MarginMode) (adapter.Position, error) {
	kind, ok := contractKinds[info.ContractType]
	if !ok || strings.TrimSpace(info.BaseCoin) == "" || strings.TrimSpace(info.QuoteCoin) == "" || strings.TrimSpace(info.SettleCoin) == "" {
		return adapter.Position{}, adapter.Malformed(epInstruments.String(), "the contract type and coins of "+info.Symbol)
	}
	var side string
	switch row.Side {
	case "Buy":
		side = adapter.SideLong
	case "Sell":
		side = adapter.SideShort
	default:
		return adapter.Position{}, adapter.Malformed(epPositions.String(), "the side of "+row.Symbol)
	}
	for name, value := range map[string]string{
		"avgPrice": row.AvgPrice, "markPrice": row.MarkPrice, "liqPrice": row.LiqPrice,
		"unrealisedPnl": row.UnrealisedPnl, "leverage": row.Leverage, "positionIM": row.PositionIM,
	} {
		if _, err := adapter.OptionalDecimal(epPositions.String(), name+" of "+row.Symbol, value); err != nil {
			return adapter.Position{}, err
		}
	}
	if row.PositionIdx == nil {
		return adapter.Position{}, adapter.Malformed(epPositions.String(), "positionIdx of "+row.Symbol)
	}
	unit := adapter.UnitBase
	if kind == adapter.KindInversePerpetual || kind == adapter.KindInverseDelivery {
		unit = adapter.UnitQuote
	}
	position := adapter.Position{
		PositionKey: fmt.Sprintf("%s:%d", row.Symbol, *row.PositionIdx),
		FuturesContract: adapter.FuturesContract{
			Symbol:                info.Symbol,
			Kind:                  kind,
			BaseAssetSymbol:       info.BaseCoin,
			QuoteAssetSymbol:      info.QuoteCoin,
			SettlementAssetSymbol: info.SettleCoin,
		},
		Side:               side,
		PositionMode:       "one_way",
		SourceQuantity:     decimal.Format(decimal.Abs(size)),
		SourceQuantityUnit: unit,
		EntryPrice:         adapter.Plain(row.AvgPrice),
		MarkPrice:          adapter.Plain(row.MarkPrice),
		UnrealizedPnl:      adapter.Plain(row.UnrealisedPnl),
	}
	if *row.PositionIdx != 0 {
		if (*row.PositionIdx != 1 || side != adapter.SideLong) && (*row.PositionIdx != 2 || side != adapter.SideShort) {
			return adapter.Position{}, adapter.Malformed(epPositions.String(), "positionIdx and direction of "+row.Symbol)
		}
		position.PositionMode = "hedged"
	}
	if kind == adapter.KindLinearDelivery || kind == adapter.KindInverseDelivery {
		expiry, ok := parseMillis(info.DeliveryTime)
		if !ok {
			return adapter.Position{}, adapter.Malformed(epInstruments.String(), "the deliveryTime of "+info.Symbol)
		}
		position.FuturesContract.ExpiryAt = &expiry
	}
	if mode != adapter.MarginBybitPM {
		position.LiquidationPrice = adapter.NonZero(row.LiqPrice)
		position.Leverage = adapter.NonZero(row.Leverage)
		position.Margin = adapter.Plain(row.PositionIM)
		position.MarginAssetSymbol, position.MarginBasis = info.SettleCoin, "initial"
		position.MarginSetting = "cross"
		if mode == adapter.MarginBybitIsolated {
			position.MarginSetting = "isolated"
		}
	}
	return position, nil
}

func parseMillis(value string) (time.Time, bool) {
	millis, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis).UTC(), true
}
