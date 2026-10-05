package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// market is a Binance futures market, traded in the portfolio_margin Account under Portfolio Margin
// and in the um_futures and cm_futures Accounts under Classic Trading.
type market string

const (
	marketUM market = "um" // USDⓈ-M futures: linear, quantity in base units
	marketCM market = "cm" // COIN-M futures: inverse, quantity in contracts
)

const (
	// instrumentTTL bounds how long exchangeInfo is reused before a refresh.
	instrumentTTL = time.Hour
	// instrumentRetry keeps an unknown symbol from refetching exchangeInfo on every position.
	instrumentRetry = time.Minute
)

type exchangeInfo struct {
	Symbols []struct {
		Symbol       string `json:"symbol"`
		ContractType string `json:"contractType"`
		BaseAsset    string `json:"baseAsset"`
		QuoteAsset   string `json:"quoteAsset"`
		MarginAsset  string `json:"marginAsset"`
		DeliveryDate int64  `json:"deliveryDate"`
		ContractSize int64  `json:"contractSize"`
	} `json:"symbols"`
}

type instrumentSet struct {
	bySymbol  map[string]adapters.Instrument
	fetchedAt time.Time
}

// instrumentCache holds UM and CM exchangeInfo in process for instrumentTTL. It is fetched only
// when a position needs it.
type instrumentCache struct {
	mu   sync.Mutex
	now  func() time.Time
	sets map[market]*instrumentSet
}

func newInstrumentCache(now func() time.Time) *instrumentCache {
	return &instrumentCache{now: now, sets: map[market]*instrumentSet{}}
}

// lookup returns the instrument of a position symbol. A symbol missing from a fresh exchangeInfo
// is a normalization failure, so the position is never dropped silently.
func (ic *instrumentCache) lookup(ctx context.Context, c *client, m market, symbol string) (adapters.Instrument, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	set := ic.sets[m]
	age := time.Duration(0)
	if set != nil {
		age = ic.now().Sub(set.fetchedAt)
		if instrument, ok := set.bySymbol[symbol]; ok && age < instrumentTTL {
			return instrument, nil
		}
	}
	if set == nil || age >= instrumentRetry {
		fresh, err := fetchInstruments(ctx, c, m)
		if err != nil {
			return adapters.Instrument{}, err
		}
		fresh.fetchedAt = ic.now()
		ic.sets[m] = fresh
		set = fresh
	}
	if instrument, ok := set.bySymbol[symbol]; ok {
		return instrument, nil
	}
	return adapters.Instrument{}, adapters.NewError(adapters.ClassNormalization,
		fmt.Sprintf("Binance %s exchangeInfo has no contract %s for an open position.", strings.ToUpper(string(m)), symbol))
}

func fetchInstruments(ctx context.Context, c *client, m market) (*instrumentSet, error) {
	ep := epUMExchangeInfo
	if m == marketCM {
		ep = epCMExchangeInfo
	}
	var info exchangeInfo
	if err := c.call(ctx, ep, nil, &info); err != nil {
		return nil, err
	}
	set := &instrumentSet{bySymbol: make(map[string]adapters.Instrument, len(info.Symbols))}
	for _, s := range info.Symbols {
		kind, delivery, ok := contractKind(m, s.ContractType)
		if !ok {
			continue
		}
		instrument := adapters.Instrument{
			Symbol:           s.Symbol,
			Kind:             kind,
			BaseSymbol:       s.BaseAsset,
			QuoteSymbol:      s.QuoteAsset,
			SettlementSymbol: s.MarginAsset,
		}
		if delivery && s.DeliveryDate > 0 {
			expiry := time.UnixMilli(s.DeliveryDate).UTC()
			instrument.ExpiryAt = &expiry
		}
		if m == marketCM && s.ContractSize > 0 {
			size := strconv.FormatInt(s.ContractSize, 10)
			instrument.ContractSize = &size
		}
		set.bySymbol[s.Symbol] = instrument
	}
	return set, nil
}

// contractKind maps a Binance contractType. PERPETUAL, TRADIFI_PERPETUAL, and
// PERPETUAL_DELIVERING are perpetual; the MONTH and QUARTER types are delivery contracts.
func contractKind(m market, contractType string) (kind string, delivery, ok bool) {
	switch {
	case strings.Contains(contractType, "PERPETUAL"):
		if m == marketCM {
			return adapters.KindInversePerp, false, true
		}
		return adapters.KindLinearPerp, false, true
	case strings.Contains(contractType, "QUARTER"), strings.Contains(contractType, "MONTH"):
		if m == marketCM {
			return adapters.KindInverseDlv, true, true
		}
		return adapters.KindLinearDeliv, true, true
	default:
		return "", false, false
	}
}
