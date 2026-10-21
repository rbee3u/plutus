package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"net/url"
	"slices"
	"strconv"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type walletAsset struct {
	Asset       string `json:"asset"`
	Free        string `json:"free"`
	Locked      string `json:"locked"`
	Freeze      string `json:"freeze"`
	Withdrawing string `json:"withdrawing"`
}

// wallet maps the spot (getUserAsset) and funding (get-funding-asset) wallets. getUserAsset does
// not list Simple Earn flexible receipts (LD* assets), so flexible positions are counted only in
// the earn account and real LD-prefixed coins such as LDO are kept.
func (a *Adapter) wallet(ctx context.Context, c *client, accountType adapter.AccountType, ep endpoint) (adapter.AccountReport, error) {
	var rows []walletAsset
	if err := c.call(ctx, ep, nil, &rows); err != nil {
		return adapter.AccountReport{}, err
	}
	balances, err := mapWallet(ep, rows)
	if err != nil {
		return adapter.AccountReport{}, err
	}
	return adapter.AccountReport{AccountType: accountType, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: balances}, nil
}

func mapWallet(ep endpoint, rows []walletAsset) ([]adapter.Balance, error) {
	where := ep.String()
	out := make([]adapter.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		parsed, _, err := amounts(where, map[string]string{"free": row.Free, "locked": row.Locked, "freeze": row.Freeze, "withdrawing": row.Withdrawing})
		if err != nil {
			return nil, err
		}
		total := decimal.Add(decimal.Add(parsed["free"], parsed["locked"]), decimal.Add(parsed["freeze"], parsed["withdrawing"]))
		if total.Sign() == 0 {
			continue
		}
		symbol, err := coin(where, row.Asset)
		if err != nil {
			return nil, err
		}
		out = append(out, adapter.Balance{
			AssetSymbol:         symbol,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: decimal.FormatPtr(parsed["free"]),
			AvailableToTrade:    decimal.FormatPtr(parsed["free"]),
		})
	}
	return out, nil
}

// earnWallet is the walletName of the earn wallet in /sapi/v1/asset/wallet/balance.
const earnWallet = "Earn"

// dualInvestmentStates says, for every purchaseStatus Binance documents,
// whether the position is running, its principal still held in dual investment
// (ADR 0005). Only running states are queried; an undocumented status fails
// the selected earn and dual accounts instead of being guessed.
var dualInvestmentStates = map[string]bool{
	"PENDING":          true,  // subscribed, awaiting the purchase result
	"PURCHASE_SUCCESS": true,  // running until the settlement date
	"SETTLING":         true,  // settled at 08:00 UTC; the payout reaches Spot within six hours
	"REFUNDING":        true,  // the purchase failed and the refund is on its way
	"SETTLED":          false, // paid out to Spot
	"PURCHASE_FAIL":    false,
	"REFUND_SUCCESS":   false, // refunded to Spot
}

// runningDualInvestmentStates lists the running states of dualInvestmentStates in a fixed order.
func runningDualInvestmentStates() []string {
	var out []string
	for _, status := range slices.Sorted(maps.Keys(dualInvestmentStates)) {
		if dualInvestmentStates[status] {
			out = append(out, status)
		}
	}
	return out
}

// simpleEarnQuery and dualInvestmentQuery are the queries of one page of the
// paged earn endpoints; walletValueQuery asks for wallet values in USDT, which
// values at 1 USD (ADR 0003). The recorder sends the same queries.
func simpleEarnQuery(current int) url.Values {
	return url.Values{"current": {strconv.Itoa(current)}, "size": {strconv.Itoa(pageSize)}}
}

func dualInvestmentQuery(status string, index int) url.Values {
	return url.Values{"status": {status}, "pageIndex": {strconv.Itoa(index)}, "pageSize": {strconv.Itoa(pageSize)}}
}

func walletValueQuery() url.Values {
	return url.Values{"quoteAsset": {"USDT"}}
}

type earnPage struct {
	Total int           `json:"total"`
	Rows  []earnHolding `json:"rows"`
}

type dualInvestmentPage struct {
	Total int                      `json:"total"`
	List  []dualInvestmentPosition `json:"list"`
}

type dualInvestmentPosition struct {
	ID                 json.RawMessage `json:"id"`
	InvestCoin         string          `json:"investCoin"`
	SubscriptionAmount string          `json:"subscriptionAmount"`
	PurchaseStatus     string          `json:"purchaseStatus"`
	OrderID            json.RawMessage `json:"orderId"`
	ExercisedCoin      string          `json:"exercisedCoin"`
	OptionType         string          `json:"optionType"`
	StrikePrice        string          `json:"strikePrice"`
	APR                string          `json:"apr"`
	SettleDate         json.RawMessage `json:"settleDate"`
	SubscriptionTime   json.RawMessage `json:"subscriptionTime"`
}

// earn returns separate savings and dual reports, then checks their combined
// principal against Binance's earn wallet valuation (ADR 0005).
func (a *Adapter) earn(ctx context.Context, c *client, prices lastPrices) (adapter.AccountReport, adapter.AccountReport, error) {
	totals := adapter.Totals{}
	earnPositions, err := simpleEarn(ctx, c, totals)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	positions, err := dualInvestment(ctx, c)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	balances := totals.Balances()
	dualTotals := adapter.Totals{}
	for i := range positions {
		p := &positions[i]
		if err := dualTotals.Add("Dual investment", p.PrincipalAssetSymbol, decimal.ParseOrZero(p.Principal)); err != nil {
			return adapter.AccountReport{}, adapter.AccountReport{}, err
		}
	}
	dualBalances := dualTotals.Balances()
	combined := append(append([]adapter.Balance{}, balances...), dualBalances...)
	if err := checkEarn(ctx, c, combined, prices); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	return adapter.AccountReport{AccountType: TypeEarn, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: balances, EarnPositions: earnPositions}, adapter.AccountReport{AccountType: TypeDualInvestment, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
}

// simpleEarn adds flexible (totalAmount) and locked (amount) positions, paged by 100.
func simpleEarn(ctx context.Context, c *client, totals adapter.Totals) ([]adapter.EarnPosition, error) {
	out := []adapter.EarnPosition{}
	for _, product := range []struct {
		ep    endpoint
		field string
	}{{epFlexibleEarn, "totalAmount"}, {epLockedEarn, "amount"}} {
		where := product.ep.String()
		for current, seen := 1, 0; ; current++ {
			var page earnPage
			if err := c.call(ctx, product.ep, simpleEarnQuery(current), &page); err != nil {
				return nil, err
			}
			for _, row := range page.Rows {
				value := row.TotalAmount
				if product.field == "amount" {
					value = row.Amount
				}
				amount, err := adapter.Decimal(where, product.field, value)
				if err != nil {
					return nil, err
				}
				if amount.Sign() == 0 {
					continue
				}
				if err := totals.Add(where, row.Asset, amount); err != nil {
					return nil, err
				}
				p, err := row.position(product.field, decimal.Format(amount))
				if err != nil {
					return nil, err
				}
				out = append(out, p)
			}
			seen += len(page.Rows)
			if len(page.Rows) == 0 || seen >= page.Total {
				break
			}
		}
	}
	return out, adapter.CheckEarnPositions(out, totals.Balances())
}

// dualInvestment adds the principal of every running dual investment position: subscriptionAmount
// in investCoin, whatever the settlement will pay out. Each running state is queried on its own,
// so the read is bounded by the open positions rather than by the account's history. A position
// that moves between running states while they are read is counted once, by id.
func dualInvestment(ctx context.Context, c *client) ([]adapter.DualPosition, error) {
	counted := map[string]int{}
	var out []adapter.DualPosition
	for _, status := range runningDualInvestmentStates() {
		for index, seen := 1, 0; ; index++ {
			var page dualInvestmentPage
			if err := c.call(ctx, epDualInvest, dualInvestmentQuery(status, index), &page); err != nil {
				return nil, err
			}
			for i := range page.List {
				row := &page.List[i]
				if _, ok := dualInvestmentStates[row.PurchaseStatus]; !ok || row.PurchaseStatus != status {
					return nil, adapter.NewError(adapter.ClassNormalization, fmt.Sprintf("%s: unexpected dual investment status %q for %s.", epDualInvest.String(), row.PurchaseStatus, status))
				}
				p, err := mapDualPosition(row)
				if err != nil {
					return nil, err
				}
				if previous, exists := counted[p.PositionKey]; exists {
					if !sameDualTerms(&out[previous], &p) {
						return nil, adapter.Malformed(epDualInvest.String(), "conflicting dual subscription terms")
					}
					continue
				}
				counted[p.PositionKey] = len(out)
				out = append(out, p)
			}
			seen += len(page.List)
			if len(page.List) == 0 || seen >= page.Total {
				break
			}
		}
	}
	return out, nil
}

// Running states may overlap while reads are in progress, but subscription terms must agree.
func sameDualTerms(a, b *adapter.DualPosition) bool {
	if a.PrincipalAssetSymbol != b.PrincipalAssetSymbol || a.Principal != b.Principal || (a.ExpiryAt != nil && b.ExpiryAt != nil && !a.ExpiryAt.Equal(*b.ExpiryAt)) || (a.SubscribedAt != nil && b.SubscribedAt != nil && !a.SubscribedAt.Equal(*b.SubscribedAt)) {
		return false
	}
	for _, pair := range [][2]*string{{a.ProductRef, b.ProductRef}, {a.OrderRef, b.OrderRef}, {a.BaseAssetSymbol, b.BaseAssetSymbol}, {a.QuoteAssetSymbol, b.QuoteAssetSymbol}, {a.Direction, b.Direction}, {a.TargetPrice, b.TargetPrice}, {a.AnnualizedRate, b.AnnualizedRate}, {a.RateKind, b.RateKind}} {
		if pair[0] != nil && pair[1] != nil && *pair[0] != *pair[1] {
			return false
		}
	}
	return true
}

func dualID(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	if string(value) == "null" {
		return ""
	}
	return string(value)
}

func mapDualPosition(row *dualInvestmentPosition) (adapter.DualPosition, error) {
	where := epDualInvest.String()
	expiry, err := adapter.DualExpiry(where, dualID(row.SettleDate))
	if err != nil {
		return adapter.DualPosition{}, err
	}
	subscribed, err := adapter.OptionalMillis(where, "dual subscription time", dualID(row.SubscriptionTime))
	if err != nil {
		return adapter.DualPosition{}, err
	}
	p := adapter.DualPosition{PositionKey: dualID(row.ID), OrderRef: adapter.OptionalText(dualID(row.OrderID)), PrincipalAssetSymbol: row.InvestCoin, Principal: row.SubscriptionAmount, TargetPrice: adapter.OptionalText(row.StrikePrice), AnnualizedRate: adapter.OptionalText(row.APR), RateKind: adapter.OptionalText("apr"), ExpiryAt: expiry, SubscribedAt: subscribed}
	switch row.OptionType {
	case "PUT":
		p.Direction = adapter.OptionalText("buy_low")
		p.BaseAssetSymbol = adapter.OptionalText(row.ExercisedCoin)
		p.QuoteAssetSymbol = adapter.OptionalText(row.InvestCoin)
	case "CALL":
		p.Direction = adapter.OptionalText("sell_high")
		p.BaseAssetSymbol = adapter.OptionalText(row.InvestCoin)
		p.QuoteAssetSymbol = adapter.OptionalText(row.ExercisedCoin)
	case "":
	default:
		return p, adapter.Malformed(where, "optionType")
	}
	p.Status = map[string]string{"PENDING": "pending", "PURCHASE_SUCCESS": "active", "SETTLING": "settling", "REFUNDING": "refunding"}[row.PurchaseStatus]
	return adapter.NormalizeDualPosition(where, &p)
}

// checkEarn compares combined earn and dual balances with Binance's earn wallet valuation. A
// wallet list without the earn wallet is a normalization failure.
func checkEarn(ctx context.Context, c *client, balances []adapter.Balance, prices lastPrices) error {
	var wallets []struct {
		Balance    string `json:"balance"`
		WalletName string `json:"walletName"`
	}
	if err := c.call(ctx, epWalletValue, walletValueQuery(), &wallets); err != nil {
		return err
	}
	var valuation *big.Rat
	for _, wallet := range wallets {
		if wallet.WalletName != earnWallet {
			continue
		}
		value, err := adapter.Decimal(epWalletValue.String(), "balance", wallet.Balance)
		if err != nil {
			return err
		}
		valuation = value
	}
	return adapter.CheckEarnComplete(adapter.PlatformBinance, valuation, balances, prices.usd)
}
