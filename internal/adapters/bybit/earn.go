package bybit

import (
	"context"
	"fmt"
	"math/big"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

// earnBucket is the asset-overview account type that carries Bybit's own valuation of Earn.
const earnBucket = "Earn"

// dualAssetStatuses are the Dual Asset position states Bybit documents. Both
// hold principal that has not been settled yet, so both count (ADR 0005).
var dualAssetStatuses = map[string]bool{"Active": true, "Redeeming": true}

type dualAssetPosition struct {
	Category       string `json:"category"`
	InvestCoin     string `json:"investCoin"`
	Amount         string `json:"amount"`
	Status         string `json:"status"`
	PositionID     string `json:"positionId"`
	ProductID      string `json:"productId"`
	OrderID        string `json:"orderId"`
	BaseCoin       string `json:"baseCoin"`
	QuoteCoin      string `json:"quoteCoin"`
	Direction      string `json:"direction"`
	TargetPrice    string `json:"targetPrice"`
	APYE8          string `json:"apyE8"`
	SettlementTime string `json:"settlementTime"`
	YieldStartAt   string `json:"yieldStartAt"`
}

// earn returns separate savings and Dual reports. Their combined principal is checked
// against Bybit's Earn valuation so unsupported holdings fail selected dependent Accounts.
func (a *Adapter) earn(ctx context.Context, c *client, prices spotPrices, enrich bool) (adapters.AccountReport, adapters.AccountReport, error) {
	totals := adapters.Totals{}
	earnPositions := []adapters.EarnPosition{}
	if err := savings(ctx, c, epFlexible, "FlexibleSaving", totals, &earnPositions); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	if err := savings(ctx, c, epFixedTerm, "FixedTermSaving", totals, &earnPositions); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	positions, err := dualAssets(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := adapters.CheckEarnPositions(earnPositions, balances); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	if enrich {
		enrichFlexible(ctx, c, earnPositions)
	}
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
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances, EarnDetailsCollected: true, EarnPositions: earnPositions}, adapters.AccountReport{AccountType: TypeDualInvestment, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
}

// savings adds the principal of one savings category to totals. `amount` is the principal,
// pending and early redemptions included. Flexible `claimableYield` is not part of the holding:
// on the recorded account Bybit's own Earn value matched the summed amounts, not amounts plus
// yield. Fixed-term `effectiveAmount` lags the principal by a day and is not used either. Neither
// endpoint pages, so a page cursor means a list Plutus cannot read to its end.
func savings(ctx context.Context, c *client, ep endpoint, category string, totals adapters.Totals, positions *[]adapters.EarnPosition) error {
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
		if err := totals.Add(where, row.Coin, amount); err != nil {
			return err
		}
		if amount.Sign() != 0 {
			p := row.position(category, decimal.Format(amount))
			*positions = append(*positions, p)
		}
	}
	return nil
}

// dualAssets preserves each open Dual Asset position and its principal in the coin invested:
// the quote coin of a BuyLow position and the base coin of a SellHigh one. A state Bybit does not
// document fails the Account instead of guessing whether its principal is still held.
func dualAssets(ctx context.Context, c *client) ([]adapters.DualPosition, error) {
	rows, err := pages[dualAssetPosition](ctx, c, epDualAssets, nil)
	if err != nil {
		return nil, err
	}
	where := epDualAssets.String()
	var out []adapters.DualPosition
	seen := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		if row.Category != "" && row.Category != "DualAssets" {
			return nil, adapters.Malformed(where, "dual category")
		}
		if !dualAssetStatuses[row.Status] {
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: undocumented Dual Asset status %q.", where, row.Status))
		}
		expiry, err := adapters.DualExpiry(where, row.SettlementTime)
		if err != nil {
			return nil, err
		}
		subscribed, err := adapters.DualTime(where, "dual subscription time", row.YieldStartAt)
		if err != nil {
			return nil, err
		}
		var rate *string
		if row.APYE8 != "" {
			parsed, err := adapters.Decimal(where, "apyE8", row.APYE8)
			if err != nil {
				return nil, err
			}
			rate = decimal.FormatPtr(new(big.Rat).Quo(parsed, big.NewRat(100000000, 1)))
		}
		var direction *string
		switch row.Direction {
		case "BuyLow":
			direction = adapters.DualText("buy_low")
		case "SellHigh":
			direction = adapters.DualText("sell_high")
		case "":
		default:
			return nil, adapters.Malformed(where, "dual direction")
		}
		state := "active"
		if row.Status == "Redeeming" {
			state = "redeeming"
		}
		p, err := adapters.NormalizeDualPosition(where, &adapters.DualPosition{PlatformPositionRef: row.PositionID, ProductRef: adapters.DualText(row.ProductID), OrderRef: adapters.DualText(row.OrderID), BaseSymbol: adapters.DualText(row.BaseCoin), QuoteSymbol: adapters.DualText(row.QuoteCoin), InvestSymbol: row.InvestCoin, Principal: row.Amount, Direction: direction, TargetPrice: adapters.DualText(row.TargetPrice), ExpiryAt: expiry, SubscribedAt: subscribed, AnnualizedRate: rate, RateKind: adapters.DualText("apy"), Status: state})
		if err != nil {
			return nil, err
		}
		if seen[p.PlatformPositionRef] {
			return nil, adapters.Malformed(where, "duplicate dual position id")
		}
		seen[p.PlatformPositionRef] = true
		out = append(out, p)
	}
	return out, nil
}

// checkEarn compares combined Earn and Dual Balances with Bybit's own USD
// valuation of Earn in asset-overview (ADR 0005). A list without an Earn entry
// counts as zero: the check only ever fails when Bybit values Earn higher than
// the Balances.
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
