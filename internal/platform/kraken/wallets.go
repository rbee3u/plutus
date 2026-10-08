package kraken

import (
	"context"
	"maps"
	"math/big"
	"net/url"
	"slices"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// snapshot is the balances_snapshot message of the Futures balances feed.
type snapshot struct {
	Timestamp int64                   `json:"timestamp"`
	Holding   map[string]number       `json:"holding"`
	Futures   map[string]singleWallet `json:"futures"`
	Flex      multiWallet             `json:"flex_futures"`
	// FuturesUPnLCashValue is the USD value of every single-collateral wallet with its
	// unrealized profit.
	FuturesUPnLCashValue number `json:"futures_upnl_cash_value"`
}

// observedAt is Kraken's own time of the balance state, or now when the snapshot has none.
func (s *snapshot) observedAt(now time.Time) time.Time {
	if s.Timestamp > 0 {
		return time.UnixMilli(s.Timestamp).UTC()
	}
	return now
}

// multiWallet is the multi-collateral wallet: Multi-M in Separate wallets, Main in Unified Wallet.
type multiWallet struct {
	Isolated          map[string]isolatedWallet     `json:"isolated"`
	UnifiedBalances   *bool                         `json:"unified_balances"`
	Currencies        map[string]collateralCurrency `json:"currencies"`
	PortfolioValue    number                        `json:"portfolio_value"`
	MarginEquity      number                        `json:"margin_equity"`
	AvailableMargin   number                        `json:"available_margin"`
	InitialMargin     number                        `json:"initial_margin"`
	MaintenanceMargin number                        `json:"maintenance_margin"`
}

type isolatedWallet struct {
	InitialWithoutOrders number `json:"initial_margin_without_orders"`
}

// collateralCurrency is one currency of the multi-collateral wallet.
type collateralCurrency struct {
	Quantity  number `json:"quantity"`
	Available number `json:"available"`
}

// singleWallet is one single-collateral (Coin-M) wallet, valued in its own unit.
type singleWallet struct {
	Unit           string `json:"unit"`
	PortfolioValue number `json:"portfolio_value"`
}

// multiCollateral maps the multi-collateral wallet: Main in Unified Wallet, Multi-M in Separate
// wallets. Its equity is portfolio_value, which already holds unrealized profit and funding;
// margin_equity is the haircut-adjusted collateral Kraken margins against. Each currency is a
// Balance in coin units; the wallet's derivative profit is not split among currencies.
func (r *run) multiCollateral(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	wallet := &r.snapshot.Flex
	if _, err := adapters.Decimal(where, "flex_futures.portfolio_value", string(wallet.PortfolioValue)); err != nil {
		return adapters.AccountReport{}, err
	}
	if wallet.Currencies == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "flex_futures.currencies")
	}
	figures := map[string]number{"margin_equity": wallet.MarginEquity, "initial_margin": wallet.InitialMargin, "maintenance_margin": wallet.MaintenanceMargin}
	parsed := map[string]*big.Rat{}
	for name, value := range figures {
		rat, err := adapters.OptionalDecimal(where, "flex_futures."+name, string(value))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		parsed[name] = rat
	}
	report := adapters.AccountReport{
		AccountType:     accountType,
		Status:          adapters.StatusOK,
		ObservedAt:      r.snapshot.observedAt(r.a.now()),
		Equity:          adapters.Plain(string(wallet.PortfolioValue)),
		AdjustedEquity:  adapters.Plain(string(wallet.MarginEquity)),
		AvailableEquity: adapters.Plain(string(wallet.AvailableMargin)),
	}
	// Kraken reports the margin requirements as USD amounts, like Binance and OKX, so the stored
	// ratios are derived against the adjusted equity.
	if adjusted := parsed["margin_equity"]; adjusted.Sign() > 0 {
		if wallet.InitialMargin != "" {
			report.InitialMarginRatio = adapters.Ratio(parsed["initial_margin"], adjusted)
		}
		if wallet.MaintenanceMargin != "" {
			report.MaintenanceMarginRatio = adapters.Ratio(parsed["maintenance_margin"], adjusted)
		}
	}
	for _, code := range slices.Sorted(maps.Keys(wallet.Currencies)) {
		row := wallet.Currencies[code]
		quantity, err := adapters.Decimal(where, "flex_futures.currencies."+code+".quantity", string(row.Quantity))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if quantity.Sign() == 0 {
			continue
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		report.Balances = append(report.Balances, adapters.Balance{
			Symbol:           symbol,
			Equity:           decimal.Format(quantity),
			AvailableToTrade: adapters.Plain(string(row.Available)),
		})
	}
	positions, err := r.positions(ctx, true)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.Positions = positions
	return report, nil
}

// coinM maps the single-collateral wallets: one Balance per settlement unit in coin units, and
// futures_upnl_cash_value, already in USD with unrealized profit, as the equity.
func (r *run) coinM(ctx context.Context) (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Futures == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "futures")
	}
	if _, err := adapters.Decimal(where, "futures_upnl_cash_value", string(r.snapshot.FuturesUPnLCashValue)); err != nil {
		return adapters.AccountReport{}, err
	}
	totals := adapters.Totals{}
	for _, name := range slices.Sorted(maps.Keys(r.snapshot.Futures)) {
		wallet := r.snapshot.Futures[name]
		value, err := adapters.Decimal(where, "futures."+name+".portfolio_value", string(wallet.PortfolioValue))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, wallet.Unit)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, value); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	positions, err := r.positions(ctx, false)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{
		AccountType: TypeCoinM,
		Status:      adapters.StatusOK,
		ObservedAt:  r.snapshot.observedAt(r.a.now()),
		Equity:      adapters.Plain(string(r.snapshot.FuturesUPnLCashValue)),
		Balances:    totals.Balances(),
		Positions:   positions,
	}, nil
}

// derivativesHolding maps the holding wallet, which only stores coins for the derivatives
// wallets and never trades.
func (r *run) derivativesHolding() (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Holding == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "holding")
	}
	totals := adapters.Totals{}
	for _, code := range slices.Sorted(maps.Keys(r.snapshot.Holding)) {
		quantity, err := adapters.Decimal(where, "holding."+code, string(r.snapshot.Holding[code]))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, quantity); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	return adapters.AccountReport{
		AccountType: TypeDerivativesHolding,
		Status:      adapters.StatusOK,
		ObservedAt:  r.snapshot.observedAt(r.a.now()),
		Balances:    totals.Balances(),
	}, nil
}

// extendedBalance is one BalanceEx row.
type extendedBalance struct {
	Balance    number `json:"balance"`
	CreditUsed number `json:"credit_used"`
}

// allocatedSuffixes mark BalanceEx rows of funds allocated to Earn: .B yield-bearing products,
// .S staking, .M opt-in rewards. Earn reports them, so Main leaves them out. .F (Kraken Rewards,
// shown as Auto Earn) earns on Main balances and stays in Main.
var allocatedSuffixes = []string{".B", ".S", ".M"}

// separateMain maps Main in Separate wallets: its BalanceEx coins, and an equity that adds the
// net result of open Spot Margin positions (TradeBalance `n`) to the coins valued at their USD
// prices. TradeBalance `e` is never used, because it values the whole balance with Kraken's own
// rules. Spot Margin positions are not itemized, as with OKX.
func (r *run) separateMain(ctx context.Context) (adapters.AccountReport, error) {
	where := "Kraken " + epBalanceEx.String()
	var rows map[string]extendedBalance
	if err := r.c.call(ctx, epBalanceEx, nil, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	if rows == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "the balance list")
	}
	observedAt := r.a.now()
	totals := adapters.Totals{}
	for _, code := range slices.Sorted(maps.Keys(rows)) {
		row := rows[code]
		used, err := adapters.OptionalDecimal(where, "credit_used of "+code, string(row.CreditUsed))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if used.Sign() != 0 {
			return adapters.AccountReport{}, adapters.NewError(adapters.ClassUnsupported, "Kraken Main uses credit, which Plutus does not support.")
		}
		amount, err := adapters.Decimal(where, "balance of "+code, string(row.Balance))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, inMain, err := r.assets.mainSymbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if inMain {
			if err := totals.Add(where, symbol, amount); err != nil {
				return adapters.AccountReport{}, err
			}
		}
	}
	var trade struct {
		Net number `json:"n"`
	}
	if err := r.c.call(ctx, epTradeBalance, url.Values{"asset": {"ZUSD"}}, &trade); err != nil {
		return adapters.AccountReport{}, err
	}
	net, err := adapters.Decimal("Kraken "+epTradeBalance.String(), "n", string(trade.Net))
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{AccountType: TypeMain, Status: adapters.StatusOK, ObservedAt: observedAt, Balances: totals.Balances()}
	equity, complete := net, true
	for _, balance := range report.Balances {
		price, ok := adapters.USDPrice(balance.Symbol, priceQuotes, r.prices.lookup)
		if !ok {
			complete = false
			continue
		}
		equity = decimal.Add(equity, decimal.Mul(decimal.ParseOrZero(balance.Equity), price))
	}
	switch {
	case complete:
		report.Equity = decimal.FormatPtr(equity)
	case net.Sign() != 0:
		// Without an equity the value is calculated from the Balances, which cannot hold the
		// margin result; valued incomplete, it would silently lose that profit or loss.
		return adapters.AccountReport{}, adapters.NewError(adapters.ClassNormalization, "Kraken Main has an open Spot Margin result and a balance without a price, so its net equity cannot be determined.")
	}
	return report, nil
}
