package binance

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Classic Trading keeps Cross Margin, Isolated Margin, USDⓈ-M Futures, and COIN-M Futures in
// separate wallets, each its own Account. Binance reports no USD equity for any of them, so every
// Account's value is calculated from its Balances and its equity stays null.

// marginAsset is one coin of a margin wallet: a Cross Margin userAssets row, or the base or quote
// side of an Isolated Margin pair.
type marginAsset struct {
	Asset    string `json:"asset"`
	Free     string `json:"free"`
	Locked   string `json:"locked"`
	Borrowed string `json:"borrowed"`
	Interest string `json:"interest"`
	NetAsset string `json:"netAsset"`
}

type crossMarginAccount struct {
	Created     *bool         `json:"created"`
	MarginLevel string        `json:"marginLevel"`
	UserAssets  []marginAsset `json:"userAssets"`
}

// crossMargin maps the Cross Margin wallet. Binance's margin level, total asset ÷ total liability
// where higher is safer, goes to the UniMMR column; Binance reports 999 when nothing is borrowed.
// A margin account that was never created is empty.
func (a *Adapter) crossMargin(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var account crossMarginAccount
	if err := c.call(ctx, epCrossMargin, nil, &account); err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{AccountType: TypeCrossMargin, Status: adapters.StatusOK, ObservedAt: a.now()}
	if account.Created != nil && !*account.Created {
		return report, nil
	}
	if _, err := adapters.OptionalDecimal(epCrossMargin.String(), "marginLevel", account.MarginLevel); err != nil {
		return adapters.AccountReport{}, err
	}
	report.UnifiedMarginRatio = adapters.Plain(account.MarginLevel)
	totals := newMarginTotals()
	for i := range account.UserAssets {
		if err := totals.add(epCrossMargin, &account.UserAssets[i]); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	report.Balances = totals.balances()
	return report, nil
}

type isolatedMarginAccount struct {
	Assets []struct {
		Base  marginAsset `json:"baseAsset"`
		Quote marginAsset `json:"quoteAsset"`
	} `json:"assets"`
}

// isolatedMargin maps the Isolated Margin wallet. Every pair holds its base and quote coins apart
// from the other pairs; the Account sums one coin across all pairs, so it appears once however
// many pairs hold it.
func (a *Adapter) isolatedMargin(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var account isolatedMarginAccount
	if err := c.call(ctx, epIsolatedMargin, nil, &account); err != nil {
		return adapters.AccountReport{}, err
	}
	totals := newMarginTotals()
	for i := range account.Assets {
		for _, side := range []*marginAsset{&account.Assets[i].Base, &account.Assets[i].Quote} {
			if err := totals.add(epIsolatedMargin, side); err != nil {
				return adapters.AccountReport{}, err
			}
		}
	}
	return adapters.AccountReport{AccountType: TypeIsolatedMargin, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: totals.balances()}, nil
}

// marginTotals sums margin wallet rows by coin. A coin's equity is its netAsset, the free and
// locked amount less the borrowed principal and interest, and may be negative.
type marginTotals struct {
	net, borrowed, interest adapters.Totals
}

func newMarginTotals() *marginTotals {
	return &marginTotals{net: adapters.Totals{}, borrowed: adapters.Totals{}, interest: adapters.Totals{}}
}

func (t *marginTotals) add(ep endpoint, row *marginAsset) error {
	where := ep.String()
	parsed, nonzero, err := amounts(where, map[string]string{
		"free": row.Free, "locked": row.Locked, "borrowed": row.Borrowed, "interest": row.Interest, "netAsset": row.NetAsset,
	})
	if err != nil || !nonzero {
		return err
	}
	net, err := adapters.Decimal(where, "netAsset", row.NetAsset)
	if err != nil {
		return err
	}
	if err := t.net.Add(where, row.Asset, net); err != nil {
		return err
	}
	if err := t.borrowed.Add(where, row.Asset, parsed["borrowed"]); err != nil {
		return err
	}
	return t.interest.Add(where, row.Asset, parsed["interest"])
}

// balances returns one Balance per coin with a nonzero equity, liability, or interest, ordered by
// coin.
func (t *marginTotals) balances() []adapters.Balance {
	out := make([]adapters.Balance, 0, len(t.net))
	for _, symbol := range adapters.SortedKeys(t.net) {
		if t.net[symbol].Sign() == 0 && t.borrowed[symbol].Sign() == 0 && t.interest[symbol].Sign() == 0 {
			continue
		}
		out = append(out, adapters.Balance{
			Symbol:    symbol,
			Equity:    decimal.Format(t.net[symbol]),
			Liability: decimal.FormatPtr(t.borrowed[symbol]),
			Interest:  decimal.FormatPtr(t.interest[symbol]),
		})
	}
	return out
}

// futuresAccount is /fapi/v3/account or /dapi/v1/account. Only USDⓈ-M reports the totals.
type futuresAccount struct {
	TotalInitialMargin string                   `json:"totalInitialMargin"`
	TotalMaintMargin   string                   `json:"totalMaintMargin"`
	TotalMarginBalance string                   `json:"totalMarginBalance"`
	AvailableBalance   string                   `json:"availableBalance"`
	Assets             []futuresAsset           `json:"assets"`
	Positions          []futuresAccountPosition `json:"positions"`
}

type futuresAccountPosition struct {
	Symbol                string `json:"symbol"`
	PositionSide          string `json:"positionSide"`
	PositionAmt           string `json:"positionAmt"`
	PositionInitialMargin string `json:"positionInitialMargin"`
	IsolatedWallet        string `json:"isolatedWallet"`
	Isolated              *bool  `json:"isolated"`
}

type futuresAsset struct {
	Asset             string `json:"asset"`
	WalletBalance     string `json:"walletBalance"`
	UnrealizedProfit  string `json:"unrealizedProfit"`
	MarginBalance     string `json:"marginBalance"`
	AvailableBalance  string `json:"availableBalance"`
	MaxWithdrawAmount string `json:"maxWithdrawAmount"`
}

// umFutures maps the USDⓈ-M Futures wallet. In Multi-Assets Mode its margin assets form one pool
// and Binance reports USD account figures for it; in Single-Asset Mode every margin asset stands
// alone and the totals cover USDT only, so no account figure is stored.
func (a *Adapter) umFutures(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var mode struct {
		MultiAssets *bool `json:"multiAssetsMargin"`
	}
	if err := c.call(ctx, epUMMultiAssets, nil, &mode); err != nil {
		return adapters.AccountReport{}, err
	}
	if mode.MultiAssets == nil {
		return adapters.AccountReport{}, adapters.Malformed(epUMMultiAssets.String(), "multiAssetsMargin")
	}
	var account futuresAccount
	if err := c.call(ctx, epUMAccount, nil, &account); err != nil {
		return adapters.AccountReport{}, err
	}
	var rows []positionRisk
	if err := c.call(ctx, epUMPositionRisk, nil, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	if err := withLeverage(ctx, c, rows); err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{AccountType: TypeUMFutures, Status: adapters.StatusOK, ObservedAt: a.now()}
	if *mode.MultiAssets {
		if err := mapMultiAssets(&account, &report); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	var err error
	if report.Balances, err = mapFuturesAssets(epUMAccount, account.Assets); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, positionSource{marketUM, epUMPositionRisk, rows}); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapMultiAssets maps the USD account figures of Multi-Assets Mode: the margin balance, which
// values collateral after Binance's haircuts, as adjusted equity, the available balance, and the
// initial and maintenance margin over the margin balance. The last is the Margin Ratio of
// Binance's futures page.
func mapMultiAssets(account *futuresAccount, report *adapters.AccountReport) error {
	where := epUMAccount.String()
	parsed := map[string]*big.Rat{}
	for name, value := range map[string]string{
		"totalMarginBalance": account.TotalMarginBalance, "totalInitialMargin": account.TotalInitialMargin,
		"totalMaintMargin": account.TotalMaintMargin, "availableBalance": account.AvailableBalance,
	} {
		rat, err := adapters.Decimal(where, name, value)
		if err != nil {
			return err
		}
		parsed[name] = rat
	}
	report.AdjustedEquity = adapters.Plain(account.TotalMarginBalance)
	report.AvailableEquity = adapters.Plain(account.AvailableBalance)
	if balance := parsed["totalMarginBalance"]; balance.Sign() > 0 {
		report.InitialMarginRatio = adapters.Ratio(parsed["totalInitialMargin"], balance)
		report.MaintenanceMarginRatio = adapters.Ratio(parsed["totalMaintMargin"], balance)
	}
	return nil
}

// cmFutures maps the COIN-M Futures wallet, where every margin coin stands alone.
func (a *Adapter) cmFutures(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var account futuresAccount
	if err := c.call(ctx, epCMAccount, nil, &account); err != nil {
		return adapters.AccountReport{}, err
	}
	var rows []positionRisk
	if err := c.call(ctx, epCMPositionRisk, nil, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	withCMRequirements(rows, account.Positions)
	report := adapters.AccountReport{AccountType: TypeCMFutures, Status: adapters.StatusOK, ObservedAt: a.now()}
	var err error
	if report.Balances, err = mapFuturesAssets(epCMAccount, account.Assets); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, positionSource{marketCM, epCMPositionRisk, rows}); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapFuturesAssets maps the margin assets of a futures wallet. A coin's equity is its margin
// balance, the wallet balance plus unrealized PnL, and may be negative. Binance lists every
// margin asset; rows that hold nothing are omitted.
func mapFuturesAssets(ep endpoint, rows []futuresAsset) ([]adapters.Balance, error) {
	where := ep.String()
	out := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		parsed, nonzero, err := amounts(where, map[string]string{
			"walletBalance": row.WalletBalance, "unrealizedProfit": row.UnrealizedProfit, "marginBalance": row.MarginBalance,
		})
		if err != nil {
			return nil, err
		}
		if !nonzero {
			continue
		}
		symbol, err := coin(where, row.Asset)
		if err != nil {
			return nil, err
		}
		equity, err := adapters.Decimal(where, "marginBalance", row.MarginBalance)
		if err != nil {
			return nil, err
		}
		if _, _, err := amounts(where, map[string]string{"availableBalance": row.AvailableBalance, "maxWithdrawAmount": row.MaxWithdrawAmount}); err != nil {
			return nil, err
		}
		out = append(out, adapters.Balance{
			Symbol:              symbol,
			Equity:              decimal.Format(equity),
			UnrealizedPnl:       decimal.FormatPtr(parsed["unrealizedProfit"]),
			AvailableToTrade:    adapters.Plain(row.AvailableBalance),
			AvailableToWithdraw: adapters.Plain(row.MaxWithdrawAmount),
		})
	}
	return out, nil
}

// withLeverage fills in the leverage /fapi/v3/positionRisk no longer reports from the account's
// symbol configuration, read once and only when a position is open. A symbol missing from the
// configuration keeps a null leverage.
func withLeverage(ctx context.Context, c *client, rows []positionRisk) error {
	open := false
	for i := range rows {
		if amount, ok := decimal.Parse(rows[i].PositionAmt); ok && amount.Sign() != 0 {
			open = true
			break
		}
	}
	if !open {
		return nil
	}
	var configs []struct {
		Symbol     string      `json:"symbol"`
		Leverage   json.Number `json:"leverage"`
		MarginType string      `json:"marginType"`
	}
	if err := c.call(ctx, epUMSymbolConfig, nil, &configs); err != nil {
		return err
	}
	leverage := make(map[string]string, len(configs))
	modes := make(map[string]string, len(configs))
	for _, config := range configs {
		leverage[config.Symbol] = config.Leverage.String()
		modes[config.Symbol] = config.MarginType
	}
	for i := range rows {
		rows[i].MarginType = modes[rows[i].Symbol]
		if rows[i].Leverage == "" {
			rows[i].Leverage = leverage[rows[i].Symbol]
		}
	}
	return nil
}

// withCMRequirements aligns only an unambiguous account row with the risk row.
// Different sizes may reflect a non-atomic read; they cannot supply this margin.
func withCMRequirements(rows []positionRisk, account []futuresAccountPosition) {
	for i := range rows {
		row := &rows[i]
		row.PositionInitialMargin = ""
		var match *futuresAccountPosition
		count := 0
		for j := range account {
			candidate := &account[j]
			if candidate.Symbol == row.Symbol && candidate.PositionSide == row.PositionSide {
				match = candidate
				count++
			}
		}
		if count != 1 {
			continue
		}
		qty, valid := decimal.Parse(row.PositionAmt)
		other, compatible := decimal.Parse(match.PositionAmt)
		if !valid || !compatible || qty.Cmp(other) != 0 {
			continue
		}
		row.PositionInitialMargin = match.PositionInitialMargin
		if row.IsolatedWallet == "" {
			row.IsolatedWallet = match.IsolatedWallet
		}
		if row.MarginType == "" && match.Isolated != nil {
			row.MarginType = "cross"
			if *match.Isolated {
				row.MarginType = "isolated"
			}
		}
	}
}
