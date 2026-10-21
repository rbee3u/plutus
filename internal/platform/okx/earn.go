package okx

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// earn maps the earn bucket, one balance per coin: Simple Earn flexible
// balances, the principal of active on-chain earn orders, and the principal of
// dual investment orders still invested. An empty amount fails the account.
// ETH and SOL staking are deliberately not collected: OKX reports those
// balances account-wide, and the BETH and OKSOL coins they produce are already
// held in the trading or funding account. Last, the earn check compares the
// balances with OKX's own valuation of earn (ADR 0005), so unitemized value
// fails selected earn and dual accounts instead of vanishing.
func (a *Adapter) earn(ctx context.Context, c *client, usdPrice func(symbol string) (*big.Rat, bool)) (adapter.AccountReport, adapter.AccountReport, error) {
	totals := adapter.Totals{}
	earnPositions := []adapter.EarnPosition{}
	var savings []savingsBalance
	if err := c.call(ctx, epSavings, &savings); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	for i := range savings {
		amount, err := adapter.Decimal(epSavings.String(), "amt of "+savings[i].Ccy, savings[i].Amt)
		if err != nil {
			return adapter.AccountReport{}, adapter.AccountReport{}, err
		}
		if err := totals.Add(epSavings.String(), savings[i].Ccy, amount); err != nil {
			return adapter.AccountReport{}, adapter.AccountReport{}, err
		}
		if amount.Sign() != 0 {
			earnPositions = append(earnPositions, savings[i].position(decimal.Format(amount)))
		}
	}
	var staking []stakingOrder
	if err := c.call(ctx, epStaking, &staking); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	for i := range staking {
		p := staking[i].position()
		for _, invested := range staking[i].InvestData {
			amount, err := adapter.Decimal(epStaking.String(), "investData.amt of "+invested.Ccy, invested.Amt)
			if err != nil {
				return adapter.AccountReport{}, adapter.AccountReport{}, err
			}
			if err := totals.Add(epStaking.String(), invested.Ccy, amount); err != nil {
				return adapter.AccountReport{}, adapter.AccountReport{}, err
			}
			if amount.Sign() != 0 {
				p.Assets = append(p.Assets, adapter.EarnAmount{AssetSymbol: invested.Ccy, Amount: decimal.Format(amount)})
			}
		}
		if len(p.Assets) > 0 {
			earnPositions = append(earnPositions, p)
		}
	}
	orders, err := a.dualInvestmentOrders(ctx, c)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	positions, err := dualPositions(orders)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := adapter.CheckEarnPositions(earnPositions, balances); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
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
	valuation, err := readEarnValuation(ctx, c)
	if err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	if err := adapter.CheckEarnComplete(adapter.PlatformOKX, valuation, combined, usdPrice); err != nil {
		return adapter.AccountReport{}, adapter.AccountReport{}, err
	}
	return adapter.AccountReport{AccountType: TypeEarn, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: balances, EarnPositions: earnPositions}, adapter.AccountReport{AccountType: TypeDualInvestment, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
}

// readEarnValuation reads OKX's own valuation of the earn bucket in USDT, which Plutus counts as USD.
func readEarnValuation(ctx context.Context, c *client) (*big.Rat, error) {
	var rows []struct {
		Details struct {
			Earn string `json:"earn"`
		} `json:"details"`
	}
	if err := c.call(ctx, epValuation, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 || strings.TrimSpace(rows[0].Details.Earn) == "" {
		return nil, adapter.Malformed(epValuation.String(), "the earn valuation")
	}
	return adapter.Decimal(epValuation.String(), "details.earn", rows[0].Details.Earn)
}
