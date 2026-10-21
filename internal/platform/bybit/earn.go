package bybit

import (
	"context"
	"fmt"
	"math/big"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// earnBucket is the asset-overview account type that carries Bybit's own valuation of earn.
const earnBucket = "Earn"

// dualAssetStatuses are the dual asset position states Bybit documents. Both
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

// earn returns separate savings and dual reports. Their combined principal is checked
// against Bybit's earn valuation so unsupported holdings fail selected dependent accounts.
func (a *Adapter) earn(ctx context.Context, c *client, prices spotPrices, enrich bool) (adapter.AccountReport, adapter.AccountReport, error) {
	totals := adapter.Totals{}
	earnPositions := []adapter.EarnPosition{}
	if err := savings(ctx, c, epFlexible, "FlexibleSaving", totals, &earnPositions); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	if err := savings(ctx, c, epFixedTerm, "FixedTermSaving", totals, &earnPositions); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	positions, err := dualAssets(ctx, c)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := adapter.CheckEarnPositions(earnPositions, balances); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	if enrich {
		enrichFlexible(ctx, c, earnPositions)
	}
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

// savings adds the principal of one savings category to totals. `amount` is the principal,
// pending and early redemptions included. Flexible `claimableYield` is not part of the holding:
// on the recorded account Bybit's own earn value matched the summed amounts, not amounts plus
// yield. Fixed-term `effectiveAmount` lags the principal by a day and is not used either. Neither
// endpoint pages, so a page cursor means a list Plutus cannot read to its end.
func savings(ctx context.Context, c *client, ep endpoint, category string, totals adapter.Totals, positions *[]adapter.EarnPosition) error {
	var result struct {
		List   []savingsPosition `json:"list"`
		Cursor string            `json:"nextPageCursor"`
	}
	if err := c.call(ctx, ep, nil, &result); err != nil {
		return err
	}
	where := ep.String()
	if result.List == nil || result.Cursor != "" {
		return adapter.Malformed(where, "a complete position list")
	}
	for i := range result.List {
		row := &result.List[i]
		if row.Category != "" && row.Category != category {
			return adapter.Malformed(where, "the category of "+row.Coin)
		}
		amount, err := adapter.Decimal(where, "amount of "+row.Coin, row.Amount)
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

// dualAssets preserves each open dual asset position and its principal in the coin invested:
// the quote coin of a BuyLow position and the base coin of a SellHigh one. A state Bybit does not
// document fails the account instead of guessing whether its principal is still held.
func dualAssets(ctx context.Context, c *client) ([]adapter.DualPosition, error) {
	rows, err := pages[dualAssetPosition](ctx, c, epDualAssets, nil)
	if err != nil {
		return nil, err
	}
	where := epDualAssets.String()
	var out []adapter.DualPosition
	seen := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		if row.Category != "" && row.Category != "DualAssets" {
			return nil, adapter.Malformed(where, "dual category")
		}
		if !dualAssetStatuses[row.Status] {
			return nil, adapter.NewError(adapter.ClassNormalization, fmt.Sprintf("%s: undocumented dual asset status %q.", where, row.Status))
		}
		expiry, err := adapter.DualExpiry(where, row.SettlementTime)
		if err != nil {
			return nil, err
		}
		subscribed, err := adapter.OptionalMillis(where, "dual subscription time", row.YieldStartAt)
		if err != nil {
			return nil, err
		}
		var rate *string
		if row.APYE8 != "" {
			parsed, err := adapter.Decimal(where, "apyE8", row.APYE8)
			if err != nil {
				return nil, err
			}
			rate = decimal.FormatPtr(new(big.Rat).Quo(parsed, big.NewRat(100000000, 1)))
		}
		var direction *string
		switch row.Direction {
		case "BuyLow":
			direction = adapter.OptionalText("buy_low")
		case "SellHigh":
			direction = adapter.OptionalText("sell_high")
		case "":
		default:
			return nil, adapter.Malformed(where, "dual direction")
		}
		state := "active"
		if row.Status == "Redeeming" {
			state = "redeeming"
		}
		p, err := adapter.NormalizeDualPosition(where, &adapter.DualPosition{PositionKey: row.PositionID, ProductRef: adapter.OptionalText(row.ProductID), OrderRef: adapter.OptionalText(row.OrderID), BaseAssetSymbol: adapter.OptionalText(row.BaseCoin), QuoteAssetSymbol: adapter.OptionalText(row.QuoteCoin), PrincipalAssetSymbol: row.InvestCoin, Principal: row.Amount, Direction: direction, TargetPrice: adapter.OptionalText(row.TargetPrice), ExpiryAt: expiry, SubscribedAt: subscribed, AnnualizedRate: rate, RateKind: adapter.OptionalText("apy"), Status: state})
		if err != nil {
			return nil, err
		}
		if seen[p.PositionKey] {
			return nil, adapter.Malformed(where, "duplicate dual position id")
		}
		seen[p.PositionKey] = true
		out = append(out, p)
	}
	return out, nil
}

// checkEarn compares combined earn and dual balances with Bybit's own USD
// valuation of earn in asset-overview (ADR 0005). A list without an earn entry
// counts as zero: the check only ever fails when Bybit values earn higher than
// the balances.
func checkEarn(ctx context.Context, c *client, balances []adapter.Balance, prices spotPrices) error {
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
		return adapter.Malformed(where, "the account list")
	}
	valued := decimal.Zero()
	for _, bucket := range overview.List {
		if bucket.AccountType != earnBucket {
			continue
		}
		if bucket.ValuationCurrency != "USD" {
			return adapter.Malformed(where, "the USD valuation of earn")
		}
		equity, err := adapter.Decimal(where, "totalEquity of earn", bucket.TotalEquity)
		if err != nil {
			return err
		}
		valued = decimal.Add(valued, equity)
	}
	return adapter.CheckEarnComplete(adapter.PlatformBybit, valued, balances, prices.usd)
}
