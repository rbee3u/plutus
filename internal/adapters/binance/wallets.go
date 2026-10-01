package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
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
// the earn Account and real LD-prefixed coins such as LDO are kept.
func (a *Adapter) wallet(ctx context.Context, c *client, accountType adapters.AccountType, ep endpoint) (adapters.AccountReport, error) {
	var rows []walletAsset
	if err := c.call(ctx, ep, nil, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	balances, err := mapWallet(ep, rows)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: accountType, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
}

func mapWallet(ep endpoint, rows []walletAsset) ([]adapters.Balance, error) {
	where := ep.String()
	out := make([]adapters.Balance, 0, len(rows))
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
		out = append(out, adapters.Balance{
			Symbol:              symbol,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: decimal.FormatPtr(parsed["free"]),
			AvailableToTrade:    decimal.FormatPtr(parsed["free"]),
		})
	}
	return out, nil
}

// earnWallet is the walletName of the Earn wallet in /sapi/v1/asset/wallet/balance.
const earnWallet = "Earn"

// dualInvestmentStates says, for every purchaseStatus Binance documents,
// whether the position is running, its principal still held in Dual Investment
// (ADR 0005). Only running states are queried; an undocumented status fails
// the selected Earn and Dual Accounts instead of being guessed.
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
	for _, status := range adapters.SortedKeys(dualInvestmentStates) {
		if dualInvestmentStates[status] {
			out = append(out, status)
		}
	}
	return out
}

// simpleEarnQuery and dualInvestmentQuery are the queries of one page of the
// paged Earn endpoints; walletValueQuery asks for wallet values in USDT, which
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
	Total int `json:"total"`
	Rows  []struct {
		Asset       string `json:"asset"`
		TotalAmount string `json:"totalAmount"`
		Amount      string `json:"amount"`
	} `json:"rows"`
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

// earn returns separate savings and Dual reports, then checks their combined
// principal against Binance's Earn wallet valuation (ADR 0005).
func (a *Adapter) earn(ctx context.Context, c *client, prices lastPrices) (adapters.AccountReport, adapters.AccountReport, error) {
	totals := adapters.Totals{}
	if err := simpleEarn(ctx, c, totals); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	positions, err := dualInvestment(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	dualTotals := adapters.Totals{}
	for i := range positions {
		p := &positions[i]
		if err := dualTotals.Add("Dual Investment", p.InvestSymbol, decimal.MustParse(p.Principal)); err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
	}
	dualBalances := dualTotals.Balances()
	combined := append(append([]adapters.Balance{}, balances...), dualBalances...)
	if err := checkEarn(ctx, c, combined, prices); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, adapters.AccountReport{AccountType: TypeDualInvestment, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
}

// simpleEarn adds flexible (totalAmount) and locked (amount) positions, paged by 100.
func simpleEarn(ctx context.Context, c *client, totals adapters.Totals) error {
	for _, product := range []struct {
		ep    endpoint
		field string
	}{{epFlexibleEarn, "totalAmount"}, {epLockedEarn, "amount"}} {
		where := product.ep.String()
		for current, seen := 1, 0; ; current++ {
			var page earnPage
			if err := c.call(ctx, product.ep, simpleEarnQuery(current), &page); err != nil {
				return err
			}
			for _, row := range page.Rows {
				value := row.TotalAmount
				if product.field == "amount" {
					value = row.Amount
				}
				amount, err := adapters.Decimal(where, product.field, value)
				if err != nil {
					return err
				}
				if amount.Sign() == 0 {
					continue
				}
				if err := totals.Add(where, row.Asset, amount); err != nil {
					return err
				}
			}
			seen += len(page.Rows)
			if len(page.Rows) == 0 || seen >= page.Total {
				break
			}
		}
	}
	return nil
}

// dualInvestment adds the principal of every running dual investment position: subscriptionAmount
// in investCoin, whatever the settlement will pay out. Each running state is queried on its own,
// so the read is bounded by the open positions rather than by the account's history. A position
// that moves between running states while they are read is counted once, by id.
func dualInvestment(ctx context.Context, c *client) ([]adapters.DualPosition, error) {
	counted := map[string]int{}
	var out []adapters.DualPosition
	for _, status := range runningDualInvestmentStates() {
		for index, seen := 1, 0; ; index++ {
			var page dualInvestmentPage
			if err := c.call(ctx, epDualInvest, dualInvestmentQuery(status, index), &page); err != nil {
				return nil, err
			}
			for i := range page.List {
				row := &page.List[i]
				if _, ok := dualInvestmentStates[row.PurchaseStatus]; !ok || row.PurchaseStatus != status {
					return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: unexpected dual investment status %q for %s.", epDualInvest.String(), row.PurchaseStatus, status))
				}
				p, err := mapDualPosition(row)
				if err != nil {
					return nil, err
				}
				if previous, exists := counted[p.PlatformPositionRef]; exists {
					if !sameDualTerms(&out[previous], &p) {
						return nil, adapters.Malformed(epDualInvest.String(), "conflicting dual subscription terms")
					}
					continue
				}
				counted[p.PlatformPositionRef] = len(out)
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
func sameDualTerms(a, b *adapters.DualPosition) bool {
	if a.InvestSymbol != b.InvestSymbol || a.Principal != b.Principal || (a.ExpiryAt != nil && b.ExpiryAt != nil && !a.ExpiryAt.Equal(*b.ExpiryAt)) || (a.SubscribedAt != nil && b.SubscribedAt != nil && !a.SubscribedAt.Equal(*b.SubscribedAt)) {
		return false
	}
	for _, pair := range [][2]*string{{a.ProductRef, b.ProductRef}, {a.OrderRef, b.OrderRef}, {a.BaseSymbol, b.BaseSymbol}, {a.QuoteSymbol, b.QuoteSymbol}, {a.Direction, b.Direction}, {a.TargetPrice, b.TargetPrice}, {a.AnnualizedRate, b.AnnualizedRate}, {a.RateKind, b.RateKind}} {
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

func mapDualPosition(row *dualInvestmentPosition) (adapters.DualPosition, error) {
	where := epDualInvest.String()
	expiry, err := adapters.DualExpiry(where, dualID(row.SettleDate))
	if err != nil {
		return adapters.DualPosition{}, err
	}
	subscribed, err := adapters.DualTime(where, "dual subscription time", dualID(row.SubscriptionTime))
	if err != nil {
		return adapters.DualPosition{}, err
	}
	p := adapters.DualPosition{PlatformPositionRef: dualID(row.ID), OrderRef: adapters.DualText(dualID(row.OrderID)), InvestSymbol: row.InvestCoin, Principal: row.SubscriptionAmount, TargetPrice: adapters.DualText(row.StrikePrice), AnnualizedRate: adapters.DualText(row.APR), RateKind: adapters.DualText("apr"), ExpiryAt: expiry, SubscribedAt: subscribed}
	switch row.OptionType {
	case "PUT":
		p.Direction = adapters.DualText("buy_low")
		p.BaseSymbol = adapters.DualText(row.ExercisedCoin)
		p.QuoteSymbol = adapters.DualText(row.InvestCoin)
	case "CALL":
		p.Direction = adapters.DualText("sell_high")
		p.BaseSymbol = adapters.DualText(row.InvestCoin)
		p.QuoteSymbol = adapters.DualText(row.ExercisedCoin)
	case "":
	default:
		return p, adapters.Malformed(where, "optionType")
	}
	p.Status = map[string]string{"PENDING": "pending", "PURCHASE_SUCCESS": "active", "SETTLING": "settling", "REFUNDING": "refunding"}[row.PurchaseStatus]
	return adapters.NormalizeDualPosition(where, &p)
}

// checkEarn compares combined Earn and Dual Balances with Binance's Earn wallet valuation. A
// wallet list without the Earn wallet is a normalization failure.
func checkEarn(ctx context.Context, c *client, balances []adapters.Balance, prices lastPrices) error {
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
		value, err := adapters.Decimal(epWalletValue.String(), "balance", wallet.Balance)
		if err != nil {
			return err
		}
		valuation = value
	}
	return adapters.CheckEarnComplete(adapters.PlatformBinance, valuation, balances, prices.usd)
}
