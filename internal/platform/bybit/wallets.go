package bybit

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type walletAccount struct {
	AccountType           string       `json:"accountType"`
	TotalEquity           string       `json:"totalEquity"`
	TotalAvailableBalance string       `json:"totalAvailableBalance"`
	AccountIMRate         string       `json:"accountIMRate"`
	AccountMMRate         string       `json:"accountMMRate"`
	Coins                 []walletCoin `json:"coin"`
}

type walletCoin struct {
	Coin            string `json:"coin"`
	Equity          string `json:"equity"`
	BorrowAmount    string `json:"borrowAmount"`
	AccruedInterest string `json:"accruedInterest"`
	UnrealisedPnl   string `json:"unrealisedPnl"`
}

// unified maps the Unified Trading Account: its account-wide figures, its per-coin equities, and
// its open contract positions.
func (a *Adapter) unified(ctx context.Context, c *client, mode adapters.MarginMode) (adapters.AccountReport, error) {
	var result struct {
		List []walletAccount `json:"list"`
	}
	if err := c.call(ctx, epWallet, nil, &result); err != nil {
		return adapters.AccountReport{}, err
	}
	if len(result.List) != 1 || result.List[0].AccountType != "UNIFIED" || result.List[0].Coins == nil {
		return adapters.AccountReport{}, adapters.Malformed(epWallet.String(), "the UNIFIED account and its coin list")
	}
	account := &result.List[0]
	report, err := mapUnifiedAccount(account, mode)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.ObservedAt = a.now()
	if report.Balances, err = mapUnifiedBalances(account.Coins); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, mode); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapUnifiedAccount reads the account-wide figures, all in USD. Bybit documents them as not
// applicable to Isolated Margin, so there they are ignored and the value is calculated from the
// Balances. In Cross and Portfolio Margin an empty field stays unknown rather than becoming zero.
// totalMarginBalance is not haircut-adjusted equity and is not mapped.
func mapUnifiedAccount(in *walletAccount, mode adapters.MarginMode) (adapters.AccountReport, error) {
	report := adapters.AccountReport{AccountType: TypeUnified, Status: adapters.StatusOK}
	if mode == adapters.MarginBybitIsolated {
		return report, nil
	}
	for _, field := range []struct {
		name, value string
		dst         **string
	}{
		{"totalEquity", in.TotalEquity, &report.Equity},
		{"totalAvailableBalance", in.TotalAvailableBalance, &report.AvailableEquity},
		{"accountIMRate", in.AccountIMRate, &report.InitialMarginRatio},
		{"accountMMRate", in.AccountMMRate, &report.MaintenanceMarginRatio},
	} {
		if _, err := adapters.OptionalDecimal(epWallet.String(), field.name, field.value); err != nil {
			return adapters.AccountReport{}, err
		}
		*field.dst = adapters.Plain(field.value)
	}
	return report, nil
}

// mapUnifiedBalances maps coin[] rows. Equity is Bybit's own per-coin `equity`, which already
// nets spot borrowing and includes unrealized PnL, so neither is added or subtracted again. An
// empty `equity` fails the Account: read as zero, it would drop the coin under the Complete Set
// rule. Bybit reports no full coin name, so Name stays empty.
func mapUnifiedBalances(rows []walletCoin) ([]adapters.Balance, error) {
	where := epWallet.String()
	out := make([]adapters.Balance, 0, len(rows))
	seen := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return nil, adapters.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		equity, err := adapters.Decimal(where, "equity of "+symbol, row.Equity)
		if err != nil {
			return nil, err
		}
		parsed := map[string]*big.Rat{}
		nonzero := equity.Sign() != 0
		for name, value := range map[string]string{
			"borrowAmount": row.BorrowAmount, "accruedInterest": row.AccruedInterest, "unrealisedPnl": row.UnrealisedPnl,
		} {
			rat, err := adapters.OptionalDecimal(where, name+" of "+symbol, value)
			if err != nil {
				return nil, err
			}
			parsed[name] = rat
			nonzero = nonzero || rat.Sign() != 0
		}
		if !nonzero {
			continue
		}
		out = append(out, adapters.Balance{
			PlatformAssetSymbol: symbol,
			Equity:              decimal.Format(equity),
			Liability:           adapters.Reported(row.BorrowAmount, decimal.Abs(parsed["borrowAmount"])),
			Interest:            adapters.Reported(row.AccruedInterest, parsed["accruedInterest"]),
			UnrealizedPnl:       adapters.Reported(row.UnrealisedPnl, parsed["unrealisedPnl"]),
		})
	}
	return out, nil
}

type fundingCoin struct {
	Coin            string `json:"coin"`
	WalletBalance   string `json:"walletBalance"`
	TransferBalance string `json:"transferBalance"`
}

// funding maps the Funding account, which Bybit reports without any USD valuation, so the value
// is calculated from the Balances. Like `equity`, an empty `walletBalance` fails the Account
// instead of reading as zero.
func (a *Adapter) funding(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var result struct {
		AccountType string        `json:"accountType"`
		Balance     []fundingCoin `json:"balance"`
	}
	if err := c.call(ctx, epFunding, nil, &result); err != nil {
		return adapters.AccountReport{}, err
	}
	where := epFunding.String()
	if result.AccountType != "FUND" || result.Balance == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "the FUND balance list")
	}
	balances := make([]adapters.Balance, 0, len(result.Balance))
	seen := map[string]bool{}
	for i := range result.Balance {
		row := &result.Balance[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return adapters.AccountReport{}, adapters.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		total, err := adapters.Decimal(where, "walletBalance of "+symbol, row.WalletBalance)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		transferable, err := adapters.OptionalDecimal(where, "transferBalance of "+symbol, row.TransferBalance)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if total.Sign() == 0 {
			continue
		}
		balances = append(balances, adapters.Balance{
			PlatformAssetSymbol: symbol,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: adapters.Reported(row.TransferBalance, transferable),
		})
	}
	return adapters.AccountReport{AccountType: TypeFunding, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
}
