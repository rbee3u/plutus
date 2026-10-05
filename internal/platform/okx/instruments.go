package okx

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// market is an OKX contract market, listed by its own public/instruments call.
type market string

const (
	marketSwap    market = "SWAP"    // perpetual swaps
	marketFutures market = "FUTURES" // delivery futures
	// instTypeMargin is a borrow-backed spot position, valued through the Balances instead.
	instTypeMargin = "MARGIN"
)

// markets maps the instType of a position row to the market that lists its contract. OPTION and
// EVENTS positions have no Plutus instrument kind and are refused by name.
var markets = map[string]market{
	string(marketSwap):    marketSwap,
	string(marketFutures): marketFutures,
}

const (
	// instrumentTTL bounds how long an instrument list is reused before a refresh.
	instrumentTTL = time.Hour
	// instrumentRetry keeps an unknown contract from refetching the list on every position.
	instrumentRetry = time.Minute
)

type instrumentRow struct {
	InstID    string `json:"instId"`
	Uly       string `json:"uly"`
	CtType    string `json:"ctType"`
	CtVal     string `json:"ctVal"`
	CtMult    string `json:"ctMult"`
	SettleCcy string `json:"settleCcy"`
	ExpTime   string `json:"expTime"`
}

type instrumentSet struct {
	bySymbol  map[string]adapters.Instrument
	fetchedAt time.Time
}

// instrumentCache holds the SWAP and FUTURES lists in process for instrumentTTL. A list is
// fetched only when a position needs it.
type instrumentCache struct {
	mu   sync.Mutex
	now  func() time.Time
	sets map[market]*instrumentSet
}

func newInstrumentCache(now func() time.Time) *instrumentCache {
	return &instrumentCache{now: now, sets: map[market]*instrumentSet{}}
}

// lookup returns the instrument of a position. A contract missing from a fresh list is a
// normalization failure, so the position is never dropped silently.
func (ic *instrumentCache) lookup(ctx context.Context, c *client, m market, instID string) (adapters.Instrument, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	set := ic.sets[m]
	age := time.Duration(0)
	if set != nil {
		age = ic.now().Sub(set.fetchedAt)
		if instrument, ok := set.bySymbol[instID]; ok && age < instrumentTTL {
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
	if instrument, ok := set.bySymbol[instID]; ok {
		return instrument, nil
	}
	return adapters.Instrument{}, adapters.NewError(adapters.ClassNormalization,
		fmt.Sprintf("OKX %s instruments have no contract %s for an open position.", m, instID))
}

func fetchInstruments(ctx context.Context, c *client, m market) (*instrumentSet, error) {
	ep := epSwaps
	if m == marketFutures {
		ep = epFutures
	}
	var rows []instrumentRow
	if err := c.call(ctx, ep, &rows); err != nil {
		return nil, err
	}
	set := &instrumentSet{bySymbol: make(map[string]adapters.Instrument, len(rows))}
	for i := range rows {
		if instrument, ok := mapInstrument(m, &rows[i]); ok {
			set.bySymbol[instrument.Symbol] = instrument
		}
	}
	return set, nil
}

// mapInstrument converts one listed contract. Rows that cannot be stored are skipped: pre-open
// listings without an id or contract type, and futures without a delivery time. A position in
// such a contract then fails its Account through lookup.
func mapInstrument(m market, row *instrumentRow) (adapters.Instrument, bool) {
	instID := strings.TrimSpace(row.InstID)
	kind, delivery, ok := contractKind(m, row.CtType)
	if instID == "" || !ok {
		return adapters.Instrument{}, false
	}
	// The underlying is BASE-QUOTE for every swap and future ("DOGE-USDT", "BTC-USD"); the
	// instrument family carries OKX's settlement suffixes such as "BTC-USD_UM".
	base, quote, _ := strings.Cut(strings.TrimSpace(row.Uly), "-")
	instrument := adapters.Instrument{
		Symbol:           instID,
		Kind:             kind,
		BaseSymbol:       base,
		QuoteSymbol:      quote,
		SettlementSymbol: strings.TrimSpace(row.SettleCcy),
		ContractSize:     contractSize(row.CtVal, row.CtMult),
	}
	if delivery {
		millis, err := strconv.ParseInt(strings.TrimSpace(row.ExpTime), 10, 64)
		if err != nil || millis <= 0 {
			return adapters.Instrument{}, false
		}
		expiry := time.UnixMilli(millis).UTC()
		instrument.ExpiryAt = &expiry
	}
	return instrument, true
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
		return adapters.KindLinearPerp, false, true
	case m == marketSwap:
		return adapters.KindInversePerp, false, true
	case linear:
		return adapters.KindLinearDeliv, true, true
	default:
		return adapters.KindInverseDlv, true, true
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
	s := decimal.Format(value)
	return &s
}
