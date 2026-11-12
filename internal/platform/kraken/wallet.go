package kraken

import (
	"context"
	"maps"
	"math/big"
	"net/url"
	"slices"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// snapshot is the balances_snapshot message of the futures balances feed.
type snapshot struct {
	Timestamp int64                   `json:"timestamp"`
	Holding   map[string]number       `json:"holding"`
	Futures   map[string]singleWallet `json:"futures"`
	Flex      multiWallet             `json:"flex_futures"`
	// FuturesUPnLCashValue is the USD value of every single-collateral wallet with its
	// unrealized profit.
	FuturesUPnLCashValue number `json:"futures_upnl_cash_value"`
}

// observedAt is the optional source timestamp, never a local substitute.
func (s *snapshot) observedAt() *time.Time {
	if s.Timestamp <= 0 {
		return nil
	}
	at := time.UnixMilli(s.Timestamp).UTC()
	return &at
}

// multiWallet is the multi-collateral wallet: Multi-M in separate wallets, main in unified wallet.
type multiWallet struct {
	Isolated          map[string]isolatedWallet     `json:"isolated"`
	UnifiedBalances   *bool                         `json:"unified_balances"`
	Currencies        map[string]collateralCurrency `json:"currencies"`
	PortfolioValue    number                        `json:"portfolio_value"`
	MarginEquity      number                        `json:"margin_equity"`
	AvailableMargin   number                        `json:"available_margin"`
	InitialMargin     number                        `json:"initial_margin"`
	MaintenanceMargin number                        `json:"maintenance_margin"`
	// TotalUnrealized is the wallet's unrealized profit and funding in USD, which Kraken adds to
	// the collateral value to report PortfolioValue.
	TotalUnrealized number `json:"total_unrealized"`
}

type isolatedWallet struct {
	InitialWithoutOrders number `json:"initial_margin_without_orders"`
}

// collateralCurrency is one currency of the multi-collateral wallet.
type collateralCurrency struct {
	Quantity  number `json:"quantity"`
	Available number `json:"available"`
}

// singleWallet is one single-collateral (Coin-M) wallet, valued in its own unit. PortfolioValue
// already holds Pnl and UnrealizedFunding, as Kraken documents for the wallet.
type singleWallet struct {
	Unit              string `json:"unit"`
	PortfolioValue    number `json:"portfolio_value"`
	Pnl               number `json:"pnl"`
	UnrealizedFunding number `json:"unrealized_funding"`
}

// multiCollateral maps the multi-collateral wallet: main in unified wallet, Multi-M in separate
// wallets. Its equity is portfolio_value, which already holds unrealized profit and funding;
// margin_equity is the haircut-adjusted collateral Kraken margins against. Each currency is a
// balance in coin units. Only flexible linear contracts belong to this wallet and they settle in
// USD, so Kraken reports one USD unrealized result: the settlement currency's row carries it in
// its equity and in its UnrealizedPnl, and every collateral currency's own result is zero.
func (r *run) multiCollateral(ctx context.Context, accountType adapter.AccountType) (adapter.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	wallet := &r.snapshot.Flex
	if _, err := adapter.Decimal(where, "flex_futures.portfolio_value", string(wallet.PortfolioValue)); err != nil {
		return adapter.AccountReport{}, err
	}
	unrealized, err := adapter.Decimal(where, "flex_futures.total_unrealized", string(wallet.TotalUnrealized))
	if err != nil {
		return adapter.AccountReport{}, err
	}
	if wallet.Currencies == nil {
		return adapter.AccountReport{}, adapter.Malformed(where, "flex_futures.currencies")
	}
	figures := map[string]number{"margin_equity": wallet.MarginEquity, "initial_margin": wallet.InitialMargin, "maintenance_margin": wallet.MaintenanceMargin}
	for name, value := range figures {
		_, err := adapter.OptionalDecimal(where, "flex_futures."+name, string(value))
		if err != nil {
			return adapter.AccountReport{}, err
		}
	}
	settlement, err := r.assets.symbol(where, "USD")
	if err != nil {
		return adapter.AccountReport{}, err
	}
	report := adapter.AccountReport{
		AccountType:      accountType,
		Status:           adapter.StatusSucceeded,
		CollectedAt:      r.a.now(),
		SourceObservedAt: r.snapshot.observedAt(),
		Equity:           adapter.Plain(string(wallet.PortfolioValue)),
		AdjustedEquity:   adapter.Plain(string(wallet.MarginEquity)),
		AvailableEquity:  adapter.Plain(string(wallet.AvailableMargin)),
	}
	report.InitialMarginRequirementUSD = adapter.Plain(string(wallet.InitialMargin))
	report.MaintenanceMarginRequirementUSD = adapter.Plain(string(wallet.MaintenanceMargin))

	settled := false
	for _, code := range slices.Sorted(maps.Keys(wallet.Currencies)) {
		row := wallet.Currencies[code]
		quantity, err := adapter.Decimal(where, "flex_futures.currencies."+code+".quantity", string(row.Quantity))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if symbol == settlement {
			settled = true
			if quantity.Sign() == 0 && unrealized.Sign() == 0 {
				continue
			}
			report.Balances = append(report.Balances, adapter.Balance{
				AssetSymbol:      symbol,
				Equity:           decimal.Format(decimal.Add(quantity, unrealized)),
				AvailableToTrade: adapter.Plain(string(row.Available)),
				UnrealizedPnl:    decimal.FormatPtr(unrealized),
			})
			continue
		}
		if quantity.Sign() == 0 {
			continue
		}
		report.Balances = append(report.Balances, adapter.Balance{
			AssetSymbol:      symbol,
			Equity:           decimal.Format(quantity),
			AvailableToTrade: adapter.Plain(string(row.Available)),
			UnrealizedPnl:    decimal.FormatPtr(decimal.Zero()),
		})
	}
	// A wallet whose settlement currency holds nothing still reports a USD result. Binance lists a
	// coin whose margin balance is nonzero while its wallet balance is zero, so this row is created.
	if !settled && unrealized.Sign() != 0 {
		report.Balances = append(report.Balances, adapter.Balance{
			AssetSymbol:   settlement,
			Equity:        decimal.Format(unrealized),
			UnrealizedPnl: decimal.FormatPtr(unrealized),
		})
	}
	positions, err := r.positions(ctx, true)
	if err != nil {
		return adapter.AccountReport{}, err
	}
	report.Positions = positions
	return report, nil
}

// walletTotal aggregates the single-collateral wallets of one settlement unit.
type walletTotal struct {
	value, unrealized *big.Rat
}

// coinM maps the single-collateral wallets: one balance per settlement unit in coin units, and
// futures_upnl_cash_value, already in USD with unrealized profit, as the equity. Each wallet's
// portfolio_value already holds its unrealized result, so its row reports that result without
// adding it again.
func (r *run) coinM(ctx context.Context) (adapter.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Futures == nil {
		return adapter.AccountReport{}, adapter.Malformed(where, "futures")
	}
	if _, err := adapter.Decimal(where, "futures_upnl_cash_value", string(r.snapshot.FuturesUPnLCashValue)); err != nil {
		return adapter.AccountReport{}, err
	}
	bySymbol := map[string]*walletTotal{}
	for _, name := range slices.Sorted(maps.Keys(r.snapshot.Futures)) {
		wallet := r.snapshot.Futures[name]
		value, err := adapter.Decimal(where, "futures."+name+".portfolio_value", string(wallet.PortfolioValue))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		pnl, err := adapter.Decimal(where, "futures."+name+".pnl", string(wallet.Pnl))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		funding, err := adapter.Decimal(where, "futures."+name+".unrealized_funding", string(wallet.UnrealizedFunding))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, wallet.Unit)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if bySymbol[symbol] == nil {
			bySymbol[symbol] = &walletTotal{value: decimal.Zero(), unrealized: decimal.Zero()}
		}
		bySymbol[symbol].value = decimal.Add(bySymbol[symbol].value, value)
		bySymbol[symbol].unrealized = decimal.Add(bySymbol[symbol].unrealized, decimal.Add(pnl, funding))
	}
	balances := make([]adapter.Balance, 0, len(bySymbol))
	for _, symbol := range slices.Sorted(maps.Keys(bySymbol)) {
		if bySymbol[symbol].value.Sign() == 0 {
			continue
		}
		balances = append(balances, adapter.Balance{
			AssetSymbol:   symbol,
			Equity:        decimal.Format(bySymbol[symbol].value),
			UnrealizedPnl: decimal.FormatPtr(bySymbol[symbol].unrealized),
		})
	}
	positions, err := r.positions(ctx, false)
	if err != nil {
		return adapter.AccountReport{}, err
	}
	return adapter.AccountReport{
		AccountType:      TypeCoinM,
		Status:           adapter.StatusSucceeded,
		CollectedAt:      r.a.now(),
		SourceObservedAt: r.snapshot.observedAt(),
		Equity:           adapter.Plain(string(r.snapshot.FuturesUPnLCashValue)),
		Balances:         balances,
		Positions:        positions,
	}, nil
}

// derivativesHolding maps the holding wallet, which only stores coins for the derivatives
// wallets and never trades. Its coins therefore carry no unrealized result: zero, not unknown.
func (r *run) derivativesHolding() (adapter.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Holding == nil {
		return adapter.AccountReport{}, adapter.Malformed(where, "holding")
	}
	totals := adapter.Totals{}
	for _, code := range slices.Sorted(maps.Keys(r.snapshot.Holding)) {
		quantity, err := adapter.Decimal(where, "holding."+code, string(r.snapshot.Holding[code]))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, quantity); err != nil {
			return adapter.AccountReport{}, err
		}
	}
	balances := totals.Balances()
	for i := range balances {
		balances[i].UnrealizedPnl = decimal.FormatPtr(decimal.Zero())
	}
	return adapter.AccountReport{
		AccountType:      TypeDerivativesHolding,
		Status:           adapter.StatusSucceeded,
		CollectedAt:      r.a.now(),
		SourceObservedAt: r.snapshot.observedAt(),
		Balances:         balances,
	}, nil
}

// extendedBalance is one BalanceEx row.
type extendedBalance struct {
	Balance    number `json:"balance"`
	CreditUsed number `json:"credit_used"`
}

// allocatedSuffixes mark BalanceEx rows of funds allocated to earn: .B yield-bearing products,
// .S staking, .M opt-in rewards. Earn reports them, so main leaves them out. .F (Kraken Rewards,
// shown as Auto Earn) earns on main balances and stays in main.
var allocatedSuffixes = []string{".B", ".S", ".M"}

// separateMain maps main in separate wallets: its BalanceEx coins, and the net result of open Spot
// margin positions (TradeBalance `n`) folded into the USD row, as the settlement currency of that
// result. TradeBalance `e` is never used, because it values the whole balance with Kraken's own
// rules. Spot margin positions are not itemized, as with OKX, so other coins' own result is
// unknown and stays null.
func (r *run) separateMain(ctx context.Context) (adapter.AccountReport, error) {
	where := "Kraken " + epBalanceEx.String()
	var rows map[string]extendedBalance
	if err := r.c.call(ctx, epBalanceEx, nil, &rows); err != nil {
		return adapter.AccountReport{}, err
	}
	if rows == nil {
		return adapter.AccountReport{}, adapter.Malformed(where, "the balance list")
	}
	totals := adapter.Totals{}
	for _, code := range slices.Sorted(maps.Keys(rows)) {
		row := rows[code]
		used, err := adapter.OptionalDecimal(where, "credit_used of "+code, string(row.CreditUsed))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if used.Sign() != 0 {
			return adapter.AccountReport{}, adapter.NewError(adapter.ClassUnsupported, "The Kraken main account uses credit, which Plutus does not support.")
		}
		amount, err := adapter.Decimal(where, "balance of "+code, string(row.Balance))
		if err != nil {
			return adapter.AccountReport{}, err
		}
		symbol, inMain, err := r.assets.mainSymbol(where, code)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if inMain {
			if err := totals.Add(where, symbol, amount); err != nil {
				return adapter.AccountReport{}, err
			}
		}
	}
	var trade struct {
		Net number `json:"n"`
	}
	if err := r.c.call(ctx, epTradeBalance, url.Values{"asset": {"ZUSD"}}, &trade); err != nil {
		return adapter.AccountReport{}, err
	}
	net, err := adapter.Decimal("Kraken "+epTradeBalance.String(), "n", string(trade.Net))
	if err != nil {
		return adapter.AccountReport{}, err
	}
	settlement, err := r.assets.symbol(where, "USD")
	if err != nil {
		return adapter.AccountReport{}, err
	}
	if net.Sign() != 0 || totals[settlement] != nil {
		if err := totals.Add(where, settlement, net); err != nil {
			return adapter.AccountReport{}, err
		}
	}
	balances := totals.Balances()
	for i := range balances {
		if balances[i].AssetSymbol == settlement {
			balances[i].UnrealizedPnl = decimal.FormatPtr(net)
		}
	}
	return adapter.AccountReport{AccountType: TypeMain, Status: adapter.StatusSucceeded, CollectedAt: r.a.now(), Balances: balances}, nil
}
