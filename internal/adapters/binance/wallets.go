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

// dualInvestmentStates says, for every purchaseStatus Binance documents, whether the position is
// running, its principal still in Earn (ADR 0029). Only running states are queried; a status
// Binance adds later fails the Earn Account instead of being guessed.
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

// simpleEarnQuery and dualInvestmentQuery are the queries of one page of the paged Earn
// endpoints; walletValueQuery asks for wallet values in USDT, which values at 1 USD (ADR 0017).
// The recorder sends the same queries.
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
}

// earn maps Simple Earn flexible and locked positions and the principal of open dual investment
// positions, one Balance per coin, then checks the result against Binance's own valuation of its
// Earn wallet (ADR 0029).
func (a *Adapter) earn(ctx context.Context, c *client, prices lastPrices) (adapters.AccountReport, error) {
	totals := adapters.Totals{}
	if err := simpleEarn(ctx, c, totals); err != nil {
		return adapters.AccountReport{}, err
	}
	if err := dualInvestment(ctx, c, totals); err != nil {
		return adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := checkEarn(ctx, c, balances, prices); err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
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
func dualInvestment(ctx context.Context, c *client, totals adapters.Totals) error {
	counted := map[string]bool{}
	for _, status := range runningDualInvestmentStates() {
		for index, seen := 1, 0; ; index++ {
			var page dualInvestmentPage
			if err := c.call(ctx, epDualInvest, dualInvestmentQuery(status, index), &page); err != nil {
				return err
			}
			for i := range page.List {
				if err := countDualInvestment(status, &page.List[i], counted, totals); err != nil {
					return err
				}
			}
			seen += len(page.List)
			if len(page.List) == 0 || seen >= page.Total {
				break
			}
		}
	}
	return nil
}

// countDualInvestment adds one position of the query for the running state queried. A status
// Binance has not documented, or any other than the one queried, fails the Earn Account: the
// answer no longer shows which positions are running.
func countDualInvestment(queried string, row *dualInvestmentPosition, counted map[string]bool, totals adapters.Totals) error {
	where := epDualInvest.String()
	if _, documented := dualInvestmentStates[row.PurchaseStatus]; !documented {
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: dual investment status %q is not documented.", where, row.PurchaseStatus))
	}
	if row.PurchaseStatus != queried {
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the query for status %s returned a %s position.", where, queried, row.PurchaseStatus))
	}
	id := string(row.ID)
	if id == "" || id == "null" || id == `""` {
		return adapters.Malformed(where, "the id of a running dual investment position")
	}
	if counted[id] {
		return nil
	}
	counted[id] = true
	amount, err := adapters.Decimal(where, "subscriptionAmount", row.SubscriptionAmount)
	if err != nil {
		return err
	}
	return totals.Add(where, row.InvestCoin, amount)
}

// checkEarn compares the itemized Earn Account with Binance's valuation of its Earn wallet. A
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
