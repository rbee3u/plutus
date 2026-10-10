package kraken

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Futures instrument types Kraken documents.
const (
	// typeFlexible is a multi-collateral linear contract margined and settled in USD.
	typeFlexible = "flexible_futures"
	// typeInverse is a single-collateral inverse contract settled in its base coin.
	typeInverse = "futures_inverse"
	// typeVanilla is a single-collateral linear contract settled in its quote.
	typeVanilla = "futures_vanilla"
	typeOptions = "options"
)

// openPosition is one row of Futures openpositions.
type openPosition struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Size          number `json:"size"`
	Price         number `json:"price"`
	UnrealizedPnl number `json:"unrealizedPnl"`
	PnlCurrency   string `json:"pnlCurrency"`
}

// nativeInstrument is the contract metadata Futures instruments reports.
type nativeInstrument struct {
	Symbol          string     `json:"symbol"`
	Type            string     `json:"type"`
	Pair            string     `json:"pair"`
	Base            string     `json:"base"`
	Quote           string     `json:"quote"`
	ContractSize    number     `json:"contractSize"`
	LastTradingTime *time.Time `json:"lastTradingTime"`
}

// futuresTicker is one row of Futures tickers: the mark price and the contract's tag.
type futuresTicker struct {
	Symbol    string `json:"symbol"`
	MarkPrice number `json:"markPrice"`
	Tag       string `json:"tag"`
}

// openBook is the open positions of one run with the metadata and mark prices of their contracts.
type openBook struct {
	positions         []openPosition
	nativeInstruments map[string]nativeInstrument
	tickers           map[string]futuresTicker
}

// book reads the open positions once per run; the contract lists are fetched only when a position
// is open.
func (r *run) book(ctx context.Context) (*openBook, error) {
	if r.open != nil {
		return r.open, nil
	}
	var open struct {
		Positions []openPosition `json:"openPositions"`
	}
	if err := r.c.call(ctx, epOpenPositions, nil, &open); err != nil {
		return nil, err
	}
	if open.Positions == nil {
		return nil, adapters.Malformed("Kraken "+epOpenPositions.String(), "openPositions")
	}
	book := &openBook{positions: open.Positions}
	if len(open.Positions) > 0 {
		var listed struct {
			Instruments []nativeInstrument `json:"instruments"`
		}
		if err := r.c.call(ctx, epInstruments, url.Values{"contractType": {"all"}}, &listed); err != nil {
			return nil, err
		}
		if listed.Instruments == nil {
			return nil, adapters.Malformed("Kraken "+epInstruments.String(), "instruments")
		}
		var market struct {
			Tickers []futuresTicker `json:"tickers"`
		}
		if err := r.c.call(ctx, epFuturesTickers, nil, &market); err != nil {
			return nil, err
		}
		book.nativeInstruments = make(map[string]nativeInstrument, len(listed.Instruments))
		for _, row := range listed.Instruments {
			book.nativeInstruments[row.Symbol] = row
		}
		book.tickers = make(map[string]futuresTicker, len(market.Tickers))
		for _, row := range market.Tickers {
			book.tickers[row.Symbol] = row
		}
	}
	r.open = book
	return book, nil
}

// positions maps the open Futures positions of one wallet family: flexible futures belong to the
// multi-collateral wallet (Multi-M, or Main in Unified Wallet), inverse and vanilla futures to
// Coin-M. An option position fails the multi-collateral report, because Plutus has no Futures Contract
// kind for it. A contract missing from the instrument list fails the report instead of
// disappearing.
func (r *run) positions(ctx context.Context, multiCollateral bool) ([]adapters.Position, error) {
	book, err := r.book(ctx)
	if err != nil {
		return nil, err
	}
	where := "Kraken " + epOpenPositions.String()
	var out []adapters.Position
	seen := map[string]bool{}
	for i := range book.positions {
		row := &book.positions[i]
		size, err := adapters.Decimal(where, "size of "+row.Symbol, string(row.Size))
		if err != nil {
			return nil, err
		}
		if size.Sign() == 0 {
			continue
		}
		contract, ok := book.nativeInstruments[row.Symbol]
		if !ok {
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: contract %s is missing from the instrument list.", where, row.Symbol))
		}
		switch contract.Type {
		case typeOptions:
			if multiCollateral {
				return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: an open option position in %s cannot be stored.", where, row.Symbol))
			}
			continue
		case typeFlexible, typeInverse, typeVanilla:
		default:
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: contract %s has the undocumented type %q.", where, row.Symbol, contract.Type))
		}
		if (contract.Type == typeFlexible) != multiCollateral {
			continue
		}
		ticker := book.tickers[row.Symbol]
		position, err := mapPosition(row, &contract, &ticker, r.assets)
		if err != nil {
			return nil, err
		}
		if multiCollateral {
			if isolated, exists := r.snapshot.Flex.Isolated[row.Symbol]; exists {
				position.MarginMode, position.MarginBasis, position.MarginAssetSymbol = "isolated", "allocated", "USD"
				position.Margin = adapters.Plain(string(isolated.InitialWithoutOrders))
			} else if r.snapshot.Flex.Isolated != nil {
				position.MarginMode = "cross"
			}
		} else {
			position.MarginMode = "cross"
		}
		if seen[position.PlatformPositionRef] {
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: position %s appears twice.", where, position.PlatformPositionRef))
		}
		seen[position.PlatformPositionRef] = true
		out = append(out, position)
	}
	return out, nil
}

// mapPosition maps one position. Kraken sizes are contracts; the contract size is base coins per
// contract for linear contracts and quote units (USD) per contract for inverse ones. Unrealized
// profit is kept only when its reported currency is the settlement coin, where a flexible
// position without pnlCurrency reports USD, Kraken's documented default.
func mapPosition(row *openPosition, contract *nativeInstrument, ticker *futuresTicker, assets assetNames) (adapters.Position, error) {
	where := "Kraken " + epInstruments.String()
	base, quote := contract.Base, contract.Quote
	if base == "" || quote == "" {
		pairBase, pairQuote, ok := strings.Cut(contract.Pair, ":")
		if !ok {
			return adapters.Position{}, adapters.Malformed(where, "the currencies of "+contract.Symbol)
		}
		base, quote = pairBase, pairQuote
	}
	base, err := assets.symbol(where, base)
	if err != nil {
		return adapters.Position{}, err
	}
	quote, err = assets.symbol(where, quote)
	if err != nil {
		return adapters.Position{}, err
	}
	kind, settle := adapters.KindLinearPerpetual, "USD"
	switch contract.Type {
	case typeInverse:
		kind, settle = adapters.KindInversePerpetual, base
	case typeVanilla:
		settle = quote
	}
	expiry := contract.LastTradingTime
	switch {
	case expiry != nil && !expiry.IsZero():
		if contract.Type == typeInverse {
			kind = adapters.KindInverseDelivery
		} else {
			kind = adapters.KindLinearDelivery
		}
		at := expiry.UTC()
		expiry = &at
	case ticker.Tag != "" && ticker.Tag != "perpetual":
		return adapters.Position{}, adapters.Malformed(where, "the expiry of "+contract.Symbol)
	default:
		expiry = nil
	}
	side := adapters.SideLong
	switch row.Side {
	case "long":
	case "short":
		side = adapters.SideShort
	default:
		return adapters.Position{}, adapters.Malformed("Kraken "+epOpenPositions.String(), "the side of "+row.Symbol)
	}
	contractSize := adapters.Plain(string(contract.ContractSize))
	if contractSize == nil {
		return adapters.Position{}, adapters.Malformed(where, "the contract size of "+contract.Symbol)
	}
	size, err := adapters.Decimal("Kraken "+epOpenPositions.String(), "size of "+row.Symbol, string(row.Size))
	if err != nil {
		return adapters.Position{}, err
	}
	out := adapters.Position{
		PlatformPositionRef: row.Symbol + ":" + row.Side,
		FuturesContract: adapters.FuturesContract{
			Symbol:                row.Symbol,
			Kind:                  kind,
			BaseAssetSymbol:       base,
			QuoteAssetSymbol:      quote,
			SettlementAssetSymbol: settle,
			ExpiryAt:              expiry,
			ContractSize:          contractSize,
		},
		Side:         side,
		PositionMode: "one_way",
		Quantity:     decimal.Format(decimal.Abs(size)),
		QuantityUnit: adapters.UnitContracts,
		EntryPrice:   adapters.Plain(string(row.Price)),
		MarkPrice:    adapters.Plain(string(ticker.MarkPrice)),
	}
	pnlCurrency := row.PnlCurrency
	if pnlCurrency == "" && contract.Type == typeFlexible {
		pnlCurrency = "USD"
	}
	if pnlCurrency != "" {
		currency, err := assets.symbol(where, pnlCurrency)
		if err != nil {
			return adapters.Position{}, err
		}
		if currency == settle || (settle == "USD" && adapters.USDEquivalent(currency)) {
			out.UnrealizedPnl = adapters.Plain(string(row.UnrealizedPnl))
		}
	}
	return out, nil
}
