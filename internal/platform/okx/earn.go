package okx

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
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
func (a *Adapter) earn(ctx context.Context, c *client, usdPrice func(symbol string) (*big.Rat, bool)) (adapters.AccountReport, adapters.AccountReport, error) {
	totals := adapters.Totals{}
	earnPositions := []adapters.EarnPosition{}
	var savings []savingsBalance
	if err := c.call(ctx, epSavings, &savings); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	for i := range savings {
		amount, err := adapters.Decimal(epSavings.String(), "amt of "+savings[i].Ccy, savings[i].Amt)
		if err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
		if err := totals.Add(epSavings.String(), savings[i].Ccy, amount); err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
		if amount.Sign() != 0 {
			earnPositions = append(earnPositions, savings[i].position(decimal.Format(amount)))
		}
	}
	var staking []stakingOrder
	if err := c.call(ctx, epStaking, &staking); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	for i := range staking {
		p := staking[i].position()
		for _, invested := range staking[i].InvestData {
			amount, err := adapters.Decimal(epStaking.String(), "investData.amt of "+invested.Ccy, invested.Amt)
			if err != nil {
				return adapters.AccountReport{}, adapters.AccountReport{}, err
			}
			if err := totals.Add(epStaking.String(), invested.Ccy, amount); err != nil {
				return adapters.AccountReport{}, adapters.AccountReport{}, err
			}
			if amount.Sign() != 0 {
				p.Assets = append(p.Assets, adapters.EarnAmount{AssetSymbol: invested.Ccy, Amount: decimal.Format(amount)})
			}
		}
		if len(p.Assets) > 0 {
			earnPositions = append(earnPositions, p)
		}
	}
	orders, err := a.dualInvestmentOrders(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	positions, err := dualPositions(orders)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	if err := adapters.CheckEarnPositions(earnPositions, balances); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	dualTotals := adapters.Totals{}
	for i := range positions {
		p := &positions[i]
		if err := dualTotals.Add("Dual investment", p.PrincipalAssetSymbol, decimal.ParseOrZero(p.Principal)); err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
	}
	dualBalances := dualTotals.Balances()
	combined := append(append([]adapters.Balance{}, balances...), dualBalances...)
	valuation, err := readEarnValuation(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	if err := adapters.CheckEarnComplete(adapters.PlatformOKX, valuation, combined, usdPrice); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusSucceeded, CollectedAt: a.now(), Balances: balances, EarnPositions: earnPositions}, adapters.AccountReport{AccountType: TypeDualInvestment, Status: adapters.StatusSucceeded, CollectedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
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
		return nil, adapters.Malformed(epValuation.String(), "the earn valuation")
	}
	return adapters.Decimal(epValuation.String(), "details.earn", rows[0].Details.Earn)
}
