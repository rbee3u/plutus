package bybit

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapter"
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

// unified maps the unified trading account: its account-wide figures, its per-coin equities, and
// its open contract positions.
func (a *Adapter) unified(ctx context.Context, c *client, mode adapter.MarginMode) (adapter.AccountReport, error) {
	var result struct {
		List []walletAccount `json:"list"`
	}
	if err := c.call(ctx, epWallet, nil, &result); err != nil {
		return adapter.AccountReport{}, err
	}
	if len(result.List) != 1 || result.List[0].AccountType != "UNIFIED" || result.List[0].Coins == nil {
		return adapter.AccountReport{}, adapter.Malformed(epWallet.String(), "the UNIFIED account and its coin list")
	}
	account := &result.List[0]
	report, err := mapUnifiedAccount(account, mode)
	if err != nil {
		return adapter.AccountReport{}, err
	}
	report.CollectedAt = a.now()
	if report.Balances, err = mapUnifiedBalances(account.Coins); err != nil {
		return adapter.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, mode); err != nil {
		return adapter.AccountReport{}, err
	}
	return report, nil
}

// mapUnifiedAccount reads the account-wide figures, all in USD. Bybit documents them as not
// applicable to isolated margin, so there they are ignored and the value is calculated from the
// balances. In cross and portfolio margin an empty field stays unknown rather than becoming zero.
// totalMarginBalance is not haircut-adjusted equity and is not mapped.
func mapUnifiedAccount(in *walletAccount, mode adapter.MarginMode) (adapter.AccountReport, error) {
	report := adapter.AccountReport{AccountType: TypeUnified, Status: adapter.StatusSucceeded}
	if mode == adapter.MarginBybitIsolated {
		return report, nil
	}
	for _, field := range []struct {
		name, value string
		dst         **string
	}{
		{"totalEquity", in.TotalEquity, &report.Equity},
		{"totalAvailableBalance", in.TotalAvailableBalance, &report.AvailableEquity},
		{"accountIMRate", in.AccountIMRate, &report.ReportedInitialMarginRatio},
		{"accountMMRate", in.AccountMMRate, &report.ReportedMaintenanceMarginRatio},
	} {
		if _, err := adapter.OptionalDecimal(epWallet.String(), field.name, field.value); err != nil {
			return adapter.AccountReport{}, err
		}
		*field.dst = adapter.Plain(field.value)
	}
	return report, nil
}

// mapUnifiedBalances maps coin[] rows. Equity is Bybit's own per-coin `equity`, which already
// nets spot borrowing and includes unrealized PnL, so neither is added or subtracted again. An
// empty `equity` fails the account: read as zero, it would drop the coin under the complete set
// rule. Bybit reports no full coin name, so Name stays empty.
func mapUnifiedBalances(rows []walletCoin) ([]adapter.Balance, error) {
	where := epWallet.String()
	out := make([]adapter.Balance, 0, len(rows))
	seen := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return nil, adapter.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		equity, err := adapter.Decimal(where, "equity of "+symbol, row.Equity)
		if err != nil {
			return nil, err
		}
		parsed := map[string]*big.Rat{}
		nonzero := equity.Sign() != 0
		for name, value := range map[string]string{
			"borrowAmount": row.BorrowAmount, "accruedInterest": row.AccruedInterest, "unrealisedPnl": row.UnrealisedPnl,
		} {
			rat, err := adapter.OptionalDecimal(where, name+" of "+symbol, value)
			if err != nil {
				return nil, err
			}
			parsed[name] = rat
			nonzero = nonzero || rat.Sign() != 0
		}
		if !nonzero {
			continue
		}
		out = append(out, adapter.Balance{
			AssetSymbol:   symbol,
			Equity:        decimal.Format(equity),
			Liability:     adapter.Reported(row.BorrowAmount, decimal.Abs(parsed["borrowAmount"])),
			Interest:      adapter.Reported(row.AccruedInterest, parsed["accruedInterest"]),
			UnrealizedPnl: adapter.Reported(row.UnrealisedPnl, parsed["unrealisedPnl"]),
		})
	}
	return out, nil
}

type fundingCoin struct {
	Coin            string `json:"coin"`
	WalletBalance   string `json:"walletBalance"`
	TransferBalance string `json:"transferBalance"`
}

// funding maps the funding account, which Bybit reports without any USD valuation, so the value
// is calculated from the balances. Like `equity`, an empty `walletBalance` fails the account
// instead of reading as zero.
func (a *Adapter) funding(ctx context.Context, c *client) (adapter.AccountReport, error) {
	var result struct {
		AccountType string        `json:"accountType"`
		Balance     []fundingCoin `json:"balance"`
	}
	if err := c.call(ctx, epFunding, nil, &result); err != nil {
		return adapter.AccountReport{}, err
	}
	where := epFunding.String()
	if result.AccountType != "FUND" || result.Balance == nil {
		return adapter.AccountReport{}, adapter.Malformed(where, "the FUND balance list")
	}
	balances := make([]adapter.Balance, 0, len(result.Balance))
	seen := map[string]bool{}
	for i := range result.Balance {
		row := &result.Balance[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return adapter.AccountReport{}, adapter.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		total, err := adapter.Decimal(where, "walletBalance of "+symbol, row.WalletBalance)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		transferable, err := adapter.OptionalDecimal(where, "transferBalance of "+symbol, row.TransferBalance)
		if err != nil {
			return adapter.AccountReport{}, err
		}
		if total.Sign() == 0 {
			continue
		}
		balances = append(balances, adapter.Balance{
			AssetSymbol:         symbol,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: adapter.Reported(row.TransferBalance, transferable),
		})
	}
	return adapter.AccountReport{AccountType: TypeFunding, Status: adapter.StatusSucceeded, CollectedAt: a.now(), Balances: balances}, nil
}
