package bybit

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

// positionLists are the position/list reads that together cover every contract position of the
// Unified account: Bybit lists linear positions per settlement coin, and inverse and option
// positions in one list each.
var positionLists = []struct{ category, settleCoin string }{
	{"linear", "USDT"},
	{"linear", "USDC"},
	{"inverse", ""},
	{"option", ""},
}

// contractKinds maps Bybit's contractType to the Plutus Instrument kind.
var contractKinds = map[string]string{
	"LinearPerpetual":  adapters.KindLinearPerp,
	"InversePerpetual": adapters.KindInversePerp,
	"LinearFutures":    adapters.KindLinearDeliv,
	"InverseFutures":   adapters.KindInverseDlv,
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

// instrument is one row of market/instruments-info.
type instrument struct {
	Symbol       string `json:"symbol"`
	ContractType string `json:"contractType"`
	BaseCoin     string `json:"baseCoin"`
	QuoteCoin    string `json:"quoteCoin"`
	SettleCoin   string `json:"settleCoin"`
	DeliveryTime string `json:"deliveryTime"`
}

// positions maps the open contract positions of the Unified account. Every list is read to its
// last page before any position is kept, and instruments are read only for categories that hold a
// position. An open option position fails the Account instead of disappearing: Plutus has no
// Instrument kind for it.
func (a *Adapter) positions(ctx context.Context, c *client, mode adapters.MarginMode) ([]adapters.Position, error) {
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
			size, err := adapters.Decimal(epPositions.String(), "size of "+rows[i].Symbol, rows[i].Size)
			if err != nil {
				return nil, err
			}
			if size.Sign() == 0 {
				continue
			}
			if list.category == "option" {
				return nil, adapters.NewError(adapters.ClassNormalization,
					fmt.Sprintf("Bybit reports an open option position in %s, which Plutus cannot store.", rows[i].Symbol))
			}
			open[list.category] = append(open[list.category], rows[i])
			sizes[list.category] = append(sizes[list.category], size)
		}
	}
	var out []adapters.Position
	seen := map[string]bool{}
	for _, category := range []string{"linear", "inverse"} {
		if len(open[category]) == 0 {
			continue
		}
		instruments, err := a.instruments(ctx, c, category)
		if err != nil {
			return nil, err
		}
		for i := range open[category] {
			row := &open[category][i]
			info, ok := instruments[row.Symbol]
			if !ok {
				return nil, adapters.NewError(adapters.ClassNormalization,
					fmt.Sprintf("%s: Bybit lists no %s contract %s for an open position.", epInstruments, category, row.Symbol))
			}
			position, err := mapPosition(row, sizes[category][i], &info, mode)
			if err != nil {
				return nil, err
			}
			if seen[position.PlatformPositionRef] {
				return nil, adapters.Malformed(epPositions.String(), "a unique position "+position.PlatformPositionRef)
			}
			seen[position.PlatformPositionRef] = true
			out = append(out, position)
		}
	}
	return out, nil
}

// instruments reads every contract of one category by symbol.
func (a *Adapter) instruments(ctx context.Context, c *client, category string) (map[string]instrument, error) {
	rows, err := pages[instrument](ctx, c, epInstruments, url.Values{"category": {category}})
	if err != nil {
		return nil, err
	}
	out := make(map[string]instrument, len(rows))
	for _, row := range rows {
		out[row.Symbol] = row
	}
	return out, nil
}

// mapPosition maps one open position. Bybit reports `size` positive and the direction in `side`;
// linear sizes are in the base coin and inverse sizes in USD. Portfolio Margin positions have no
// leverage, margin, or liquidation price of their own, which Bybit leaves empty, so they stay null.
// In Cross Margin the liquidation price is Bybit's estimate for the account as a whole.
func mapPosition(row *openPosition, size *big.Rat, info *instrument, mode adapters.MarginMode) (adapters.Position, error) {
	kind, ok := contractKinds[info.ContractType]
	if !ok || strings.TrimSpace(info.BaseCoin) == "" || strings.TrimSpace(info.QuoteCoin) == "" || strings.TrimSpace(info.SettleCoin) == "" {
		return adapters.Position{}, adapters.Malformed(epInstruments.String(), "the contract type and coins of "+info.Symbol)
	}
	var side string
	switch row.Side {
	case "Buy":
		side = adapters.SideLong
	case "Sell":
		side = adapters.SideShort
	default:
		return adapters.Position{}, adapters.Malformed(epPositions.String(), "the side of "+row.Symbol)
	}
	for name, value := range map[string]string{
		"avgPrice": row.AvgPrice, "markPrice": row.MarkPrice, "liqPrice": row.LiqPrice,
		"unrealisedPnl": row.UnrealisedPnl, "leverage": row.Leverage, "positionIM": row.PositionIM,
	} {
		if _, err := adapters.OptionalDecimal(epPositions.String(), name+" of "+row.Symbol, value); err != nil {
			return adapters.Position{}, err
		}
	}
	if row.PositionIdx == nil {
		return adapters.Position{}, adapters.Malformed(epPositions.String(), "positionIdx of "+row.Symbol)
	}
	unit := adapters.UnitBase
	if kind == adapters.KindInversePerp || kind == adapters.KindInverseDlv {
		unit = adapters.UnitQuote
	}
	position := adapters.Position{
		PlatformPositionRef: fmt.Sprintf("%s:%d", row.Symbol, *row.PositionIdx),
		Instrument: adapters.Instrument{
			Symbol:           info.Symbol,
			Kind:             kind,
			BaseSymbol:       info.BaseCoin,
			QuoteSymbol:      info.QuoteCoin,
			SettlementSymbol: info.SettleCoin,
		},
		Side:          side,
		PositionMode:  "one_way",
		Quantity:      decimal.Format(decimal.Abs(size)),
		QuantityUnit:  unit,
		EntryPrice:    adapters.Plain(row.AvgPrice),
		MarkPrice:     adapters.Plain(row.MarkPrice),
		UnrealizedPnl: adapters.Plain(row.UnrealisedPnl),
	}
	if *row.PositionIdx != 0 {
		if (*row.PositionIdx != 1 || side != adapters.SideLong) && (*row.PositionIdx != 2 || side != adapters.SideShort) {
			return adapters.Position{}, adapters.Malformed(epPositions.String(), "positionIdx and direction of "+row.Symbol)
		}
		position.PositionMode = "hedged"
	}
	if kind == adapters.KindLinearDeliv || kind == adapters.KindInverseDlv {
		expiry, ok := parseMillis(info.DeliveryTime)
		if !ok {
			return adapters.Position{}, adapters.Malformed(epInstruments.String(), "the deliveryTime of "+info.Symbol)
		}
		position.Instrument.ExpiryAt = &expiry
	}
	if mode != adapters.MarginBybitPM {
		position.LiquidationPrice = adapters.NonZero(row.LiqPrice)
		position.Leverage = adapters.NonZero(row.Leverage)
		position.Margin = adapters.Plain(row.PositionIM)
		position.MarginCurrency, position.MarginBasis = info.SettleCoin, "initial"
		position.MarginMode = "cross"
		if mode == adapters.MarginBybitIsolated {
			position.MarginMode = "isolated"
		}
	}
	return position, nil
}
