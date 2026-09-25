package bybit

import (
	"context"
	"fmt"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

// earnBucket is the asset-overview account type that carries Bybit's own valuation of Earn.
const earnBucket = "Earn"

// dualAssetStatuses are the Dual Asset position states Bybit documents. Both hold principal that
// has not been settled yet, so both count (ADR 0029).
var dualAssetStatuses = map[string]bool{"Active": true, "Redeeming": true}

type savingsPosition struct {
	Coin           string `json:"coin"`
	Amount         string `json:"amount"`
	ClaimableYield string `json:"claimableYield"`
	Category       string `json:"category"`
}

type dualAssetPosition struct {
	Category   string `json:"category"`
	InvestCoin string `json:"investCoin"`
	Amount     string `json:"amount"`
	Status     string `json:"status"`
}

// earn maps the Earn account: the principal of Flexible Savings, Fixed-Term Savings, and Dual
// Asset positions, one Balance per coin. The result is then checked against Bybit's own USD
// valuation of Earn, so value held in a product Plutus does not read keeps the Account stale
// instead of vanishing under the Complete Set rule.
func (a *Adapter) earn(ctx context.Context, c *client, prices spotPrices) (adapters.AccountReport, error) {
	totals := adapters.Totals{}
	if err := savings(ctx, c, epFlexible, "FlexibleSaving", totals); err != nil {
		return adapters.AccountReport{}, err
	}
	if err := savings(ctx, c, epFixedTerm, "FixedTermSaving", totals); err != nil {
		return adapters.AccountReport{}, err
	}
	if err := dualAssets(ctx, c, totals); err != nil {
		return adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := checkEarn(ctx, c, balances, prices); err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
}

// savings adds the principal of one savings category to totals. `amount` is the principal,
// pending and early redemptions included. Flexible `claimableYield` is not part of the holding:
// on the recorded account Bybit's own Earn value matched the summed amounts, not amounts plus
// yield. Fixed-term `effectiveAmount` lags the principal by a day and is not used either. Neither
// endpoint pages, so a page cursor means a list Plutus cannot read to its end.
func savings(ctx context.Context, c *client, ep endpoint, category string, totals adapters.Totals) error {
	var result struct {
		List   []savingsPosition `json:"list"`
		Cursor string            `json:"nextPageCursor"`
	}
	if err := c.call(ctx, ep, nil, &result); err != nil {
		return err
	}
	where := ep.String()
	if result.List == nil || result.Cursor != "" {
		return adapters.Malformed(where, "a complete position list")
	}
	for i := range result.List {
		row := &result.List[i]
		if row.Category != "" && row.Category != category {
			return adapters.Malformed(where, "the category of "+row.Coin)
		}
		amount, err := adapters.Decimal(where, "amount of "+row.Coin, row.Amount)
		if err != nil {
			return err
		}
		if _, err := adapters.OptionalDecimal(where, "claimableYield of "+row.Coin, row.ClaimableYield); err != nil {
			return err
		}
		if err := totals.Add(where, row.Coin, amount); err != nil {
			return err
		}
	}
	return nil
}

// dualAssets adds the principal of every open Dual Asset position to totals, in the coin invested:
// the quote coin of a BuyLow position and the base coin of a SellHigh one. A state Bybit does not
// document fails the Account instead of guessing whether its principal is still held.
func dualAssets(ctx context.Context, c *client, totals adapters.Totals) error {
	rows, err := pages[dualAssetPosition](ctx, c, epDualAssets, nil)
	if err != nil {
		return err
	}
	where := epDualAssets.String()
	for i := range rows {
		row := &rows[i]
		if row.Category != "" && row.Category != "DualAssets" {
			return adapters.Malformed(where, "the category of a "+row.InvestCoin+" position")
		}
		if !dualAssetStatuses[row.Status] {
			return adapters.NewError(adapters.ClassNormalization,
				fmt.Sprintf("%s: Bybit reports a Dual Asset position in state %q, which Plutus does not know.", where, row.Status))
		}
		amount, err := adapters.Decimal(where, "amount of "+row.InvestCoin, row.Amount)
		if err != nil {
			return err
		}
		if err := totals.Add(where, row.InvestCoin, amount); err != nil {
			return err
		}
	}
	return nil
}

// checkEarn compares the Earn Balances with Bybit's own USD valuation of Earn in asset-overview
// (ADR 0029). A list without an Earn entry counts as zero: the check only ever fails when Bybit
// values Earn higher than the Balances.
func checkEarn(ctx context.Context, c *client, balances []adapters.Balance, prices spotPrices) error {
	var overview struct {
		List []struct {
			AccountType       string `json:"accountType"`
			TotalEquity       string `json:"totalEquity"`
			ValuationCurrency string `json:"valuationCurrency"`
		} `json:"list"`
	}
	if err := c.call(ctx, epAssetOverview, nil, &overview); err != nil {
		return err
	}
	where := epAssetOverview.String()
	if overview.List == nil {
		return adapters.Malformed(where, "the account list")
	}
	valued := decimal.Zero()
	for _, bucket := range overview.List {
		if bucket.AccountType != earnBucket {
			continue
		}
		if bucket.ValuationCurrency != "USD" {
			return adapters.Malformed(where, "the USD valuation of Earn")
		}
		equity, err := adapters.Decimal(where, "totalEquity of Earn", bucket.TotalEquity)
		if err != nil {
			return err
		}
		valued = decimal.Add(valued, equity)
	}
	return adapters.CheckEarnComplete(adapters.PlatformBybit, valued, balances, prices.usd)
}
