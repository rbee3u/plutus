package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

// market is a Binance futures market, traded in the portfolio_margin account under portfolio margin
// and in the um_futures and cm_futures accounts under classic trading.
type market string

const (
	marketUM market = "um" // USDⓈ-M futures: linear, quantity in base units
	marketCM market = "cm" // COIN-M futures: inverse, quantity in contracts
)

const (
	// futuresContractTTL bounds how long exchangeInfo is reused before a refresh.
	futuresContractTTL = time.Hour
	// futuresContractRetry keeps an unknown symbol from refetching exchangeInfo on every position.
	futuresContractRetry = time.Minute
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

type futuresContractSet struct {
	bySymbol  map[string]adapter.FuturesContract
	fetchedAt time.Time
}

// futuresContractCache holds UM and CM exchangeInfo in process for futuresContractTTL. It is fetched only
// when a position needs it.
type futuresContractCache struct {
	mu   sync.Mutex
	now  func() time.Time
	sets map[market]*futuresContractSet
}

func newFuturesContractCache(now func() time.Time) *futuresContractCache {
	return &futuresContractCache{now: now, sets: map[market]*futuresContractSet{}}
}

// lookup returns the futures contract of a position symbol. A symbol missing from a fresh exchangeInfo
// is a normalization failure, so the position is never dropped silently.
func (ic *futuresContractCache) lookup(ctx context.Context, c *client, m market, symbol string) (adapter.FuturesContract, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	set := ic.sets[m]
	age := time.Duration(0)
	if set != nil {
		age = ic.now().Sub(set.fetchedAt)
		if futuresContract, ok := set.bySymbol[symbol]; ok && age < futuresContractTTL {
			return futuresContract, nil
		}
	}
	if set == nil || age >= futuresContractRetry {
		fresh, err := fetchFuturesContracts(ctx, c, m)
		if err != nil {
			return adapter.FuturesContract{}, err
		}
		fresh.fetchedAt = ic.now()
		ic.sets[m] = fresh
		set = fresh
	}
	if futuresContract, ok := set.bySymbol[symbol]; ok {
		return futuresContract, nil
	}
	return adapter.FuturesContract{}, adapter.NewError(adapter.ClassNormalization,
		fmt.Sprintf("Binance %s exchangeInfo has no contract %s for an open position.", strings.ToUpper(string(m)), symbol))
}

func fetchFuturesContracts(ctx context.Context, c *client, m market) (*futuresContractSet, error) {
	ep := epUMExchangeInfo
	if m == marketCM {
		ep = epCMExchangeInfo
	}
	var info exchangeInfo
	if err := c.call(ctx, ep, nil, &info); err != nil {
		return nil, err
	}
	set := &futuresContractSet{bySymbol: make(map[string]adapter.FuturesContract, len(info.Symbols))}
	for _, s := range info.Symbols {
		kind, delivery, ok := contractKind(m, s.ContractType)
		if !ok {
			continue
		}
		futuresContract := adapter.FuturesContract{
			Symbol:                s.Symbol,
			Kind:                  kind,
			BaseAssetSymbol:       s.BaseAsset,
			QuoteAssetSymbol:      s.QuoteAsset,
			SettlementAssetSymbol: s.MarginAsset,
		}
		if delivery && s.DeliveryDate > 0 {
			expiry := time.UnixMilli(s.DeliveryDate).UTC()
			futuresContract.ExpiryAt = &expiry
		}
		if m == marketCM && s.ContractSize > 0 {
			size := strconv.FormatInt(s.ContractSize, 10)
			futuresContract.ContractSize = &size
		}
		set.bySymbol[s.Symbol] = futuresContract
	}
	return set, nil
}

// contractKind maps a Binance contractType. PERPETUAL, TRADIFI_PERPETUAL, and
// PERPETUAL_DELIVERING are perpetual; the MONTH and QUARTER types are delivery contracts.
func contractKind(m market, contractType string) (kind string, delivery, ok bool) {
	switch {
	case strings.Contains(contractType, "PERPETUAL"):
		if m == marketCM {
			return adapter.KindInversePerpetual, false, true
		}
		return adapter.KindLinearPerpetual, false, true
	case strings.Contains(contractType, "QUARTER"), strings.Contains(contractType, "MONTH"):
		if m == marketCM {
			return adapter.KindInverseDelivery, true, true
		}
		return adapter.KindLinearDelivery, true, true
	default:
		return "", false, false
	}
}
