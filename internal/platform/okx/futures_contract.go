package okx

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// market is an OKX contract market, listed by its own public/instruments call.
type market string

const (
	marketSwap    market = "SWAP"    // perpetual swaps
	marketFutures market = "FUTURES" // delivery futures
	// instTypeMargin is a borrow-backed spot position, valued through the balances instead.
	instTypeMargin = "MARGIN"
)

// markets maps the instType of a position row to the market that lists its contract. OPTION and
// EVENTS positions have no Plutus futures contract kind and are refused by name.
var markets = map[string]market{
	string(marketSwap):    marketSwap,
	string(marketFutures): marketFutures,
}

const (
	// futuresContractTTL bounds how long a futures contract list is reused before a refresh.
	futuresContractTTL = time.Hour
	// futuresContractRetry keeps an unknown contract from refetching the list on every position.
	futuresContractRetry = time.Minute
)

type nativeInstrumentRow struct {
	InstID    string `json:"instId"`
	Uly       string `json:"uly"`
	CtType    string `json:"ctType"`
	CtVal     string `json:"ctVal"`
	CtMult    string `json:"ctMult"`
	SettleCcy string `json:"settleCcy"`
	ExpTime   string `json:"expTime"`
}

type futuresContractSet struct {
	bySymbol  map[string]adapter.FuturesContract
	fetchedAt time.Time
}

// futuresContractCache holds the SWAP and FUTURES lists in process for futuresContractTTL. A list is
// fetched only when a position needs it.
type futuresContractCache struct {
	mu   sync.Mutex
	now  func() time.Time
	sets map[market]*futuresContractSet
}

func newFuturesContractCache(now func() time.Time) *futuresContractCache {
	return &futuresContractCache{now: now, sets: map[market]*futuresContractSet{}}
}

// lookup returns the futures contract of a position. A contract missing from a fresh list is a
// normalization failure, so the position is never dropped silently.
func (ic *futuresContractCache) lookup(ctx context.Context, c *client, m market, instID string) (adapter.FuturesContract, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	set := ic.sets[m]
	age := time.Duration(0)
	if set != nil {
		age = ic.now().Sub(set.fetchedAt)
		if futuresContract, ok := set.bySymbol[instID]; ok && age < futuresContractTTL {
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
	if futuresContract, ok := set.bySymbol[instID]; ok {
		return futuresContract, nil
	}
	return adapter.FuturesContract{}, adapter.NewError(adapter.ClassNormalization,
		fmt.Sprintf("OKX %s market has no contract %s for an open position.", m, instID))
}

func fetchFuturesContracts(ctx context.Context, c *client, m market) (*futuresContractSet, error) {
	ep := epSwaps
	if m == marketFutures {
		ep = epFutures
	}
	var rows []nativeInstrumentRow
	if err := c.call(ctx, ep, &rows); err != nil {
		return nil, err
	}
	set := &futuresContractSet{bySymbol: make(map[string]adapter.FuturesContract, len(rows))}
	for i := range rows {
		if futuresContract, ok := mapFuturesContract(m, &rows[i]); ok {
			set.bySymbol[futuresContract.Symbol] = futuresContract
		}
	}
	return set, nil
}

// mapFuturesContract converts one listed contract. Rows that cannot be stored are skipped: pre-open
// listings without an id or contract type, and futures without a delivery time. A position in
// such a contract then fails its account through lookup.
func mapFuturesContract(m market, row *nativeInstrumentRow) (adapter.FuturesContract, bool) {
	instID := strings.TrimSpace(row.InstID)
	kind, delivery, ok := contractKind(m, row.CtType)
	if instID == "" || !ok {
		return adapter.FuturesContract{}, false
	}
	// The underlying is BASE-QUOTE for every swap and future ("DOGE-USDT", "BTC-USD"); the
	// instrument family carries OKX's settlement suffixes such as "BTC-USD_UM".
	base, quote, _ := strings.Cut(strings.TrimSpace(row.Uly), "-")
	futuresContract := adapter.FuturesContract{
		Symbol:                instID,
		Kind:                  kind,
		BaseAssetSymbol:       base,
		QuoteAssetSymbol:      quote,
		SettlementAssetSymbol: strings.TrimSpace(row.SettleCcy),
		ContractSize:          contractSize(row.CtVal, row.CtMult),
	}
	if delivery {
		millis, err := strconv.ParseInt(strings.TrimSpace(row.ExpTime), 10, 64)
		if err != nil || millis <= 0 {
			return adapter.FuturesContract{}, false
		}
		expiry := time.UnixMilli(millis).UTC()
		futuresContract.ExpiryAt = &expiry
	}
	return futuresContract, true
}

// contractKind maps an OKX market and contract type. A linear contract is margined and settled
// in the quote coin; an inverse one in the base coin, with a face value in USD.
func contractKind(m market, ctType string) (kind string, delivery, ok bool) {
	linear := strings.EqualFold(strings.TrimSpace(ctType), "linear")
	if !linear && !strings.EqualFold(strings.TrimSpace(ctType), "inverse") {
		return "", false, false
	}
	switch {
	case m == marketSwap && linear:
		return adapter.KindLinearPerpetual, false, true
	case m == marketSwap:
		return adapter.KindInversePerpetual, false, true
	case linear:
		return adapter.KindLinearDelivery, true, true
	default:
		return adapter.KindInverseDelivery, true, true
	}
}

// contractSize is the face value of one contract: ctVal in the base coin for linear contracts
// and in USD for inverse ones, times the contract multiplier (1 for every swap and future so far).
func contractSize(ctVal, ctMult string) *string {
	value, ok := decimal.Parse(ctVal)
	if !ok || value.Sign() <= 0 {
		return nil
	}
	if multiplier, ok := decimal.Parse(ctMult); ok && multiplier.Sign() > 0 {
		value = new(big.Rat).Mul(value, multiplier)
	}
	return new(decimal.Format(value))
}
