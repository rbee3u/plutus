package okx

import (
	"context"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type tradingAccount struct {
	TotalEq  string           `json:"totalEq"`
	AdjEq    string           `json:"adjEq"`
	AvailEq  string           `json:"availEq"`
	IMR      string           `json:"imr"`
	MMR      string           `json:"mmr"`
	MgnRatio string           `json:"mgnRatio"`
	Details  []tradingBalance `json:"details"`
}

type tradingBalance struct {
	Ccy      string `json:"ccy"`
	Eq       string `json:"eq"`
	AvailEq  string `json:"availEq"`
	AvailBal string `json:"availBal"`
	Liab     string `json:"liab"`
	Interest string `json:"interest"`
	Upl      string `json:"upl"`
}

// trading maps the trading account: its USD equity and margin figures, its
// per-coin equities, and its open positions. One mapping serves all four
// margin modes: OKX returns "" for every field that does not apply to the
// account's level (docs/platform/okx/mapping.md lists them per mode), and an
// empty field stays unknown rather than becoming zero.
func (a *Adapter) trading(ctx context.Context, c *client, mode adapters.MarginMode) (adapters.AccountReport, error) {
	var rows []tradingAccount
	if err := c.call(ctx, epBalance, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	if len(rows) == 0 {
		return adapters.AccountReport{}, adapters.Malformed(epBalance.String(), "the trading account")
	}
	account := &rows[0]
	report, err := mapTradingAccount(account)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.CollectedAt = a.now()
	if report.Balances, err = mapTradingBalances(account.Details); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, mode); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapTradingAccount reads the account-level figures. Futures mode reports none of them but
// totalEq, and Spot mode no availEq.
func mapTradingAccount(in *tradingAccount) (adapters.AccountReport, error) {
	where := epBalance.String()
	if _, err := adapters.Decimal(where, "totalEq", in.TotalEq); err != nil {
		return adapters.AccountReport{}, err
	}
	_, err := optionalFields(where, "",
		field{"adjEq", in.AdjEq}, field{"availEq", in.AvailEq}, field{"imr", in.IMR}, field{"mmr", in.MMR}, field{"mgnRatio", in.MgnRatio})
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{
		AccountType:         TypeTrading,
		Status:              adapters.StatusSucceeded,
		Equity:              adapters.Plain(in.TotalEq),
		AdjustedEquity:      adapters.Plain(in.AdjEq),
		AvailableEquity:     adapters.Plain(in.AvailEq),
		ReportedMarginRatio: adapters.Plain(in.MgnRatio),
	}
	report.InitialMarginRequirementUSD = adapters.Plain(in.IMR)
	report.MaintenanceMarginRequirementUSD = adapters.Plain(in.MMR)

	return report, nil
}

// mapTradingBalances maps details[] rows. Equity is OKX's own `eq`, never recomputed: the sum of
// the rows' USD equities is exactly the account's totalEq, while the composition of `eq` is not
// documented. An empty `eq` is unknown and fails the account; read as zero, it would drop the coin
// under the complete set rule. Liabilities are reported positive and stored positive.
func mapTradingBalances(rows []tradingBalance) ([]adapters.Balance, error) {
	where := epBalance.String()
	out := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		equity, err := adapters.Decimal(where, "eq of "+row.Ccy, row.Eq)
		if err != nil {
			return nil, err
		}
		parsed, err := optionalFields(where, row.Ccy,
			field{"availEq", row.AvailEq}, field{"availBal", row.AvailBal}, field{"liab", row.Liab},
			field{"interest", row.Interest}, field{"upl", row.Upl})
		if err != nil {
			return nil, err
		}
		nonzero := equity.Sign() != 0
		for _, value := range parsed {
			nonzero = nonzero || value.Sign() != 0
		}
		if !nonzero {
			continue
		}
		if strings.TrimSpace(row.Ccy) == "" {
			return nil, adapters.Malformed(where, "the currency of a nonzero balance")
		}
		out = append(out, adapters.Balance{
			AssetSymbol:         row.Ccy,
			Equity:              decimal.Format(equity),
			AvailableToTrade:    adapters.Reported(row.AvailEq, parsed["availEq"]),
			AvailableToWithdraw: adapters.Reported(row.AvailBal, parsed["availBal"]),
			Liability:           adapters.Reported(row.Liab, decimal.Abs(parsed["liab"])),
			Interest:            adapters.Reported(row.Interest, parsed["interest"]),
			UnrealizedPnl:       adapters.Reported(row.Upl, parsed["upl"]),
		})
	}
	return out, nil
}

// field is one named value of a response row.
type field struct{ name, value string }

// optionalFields parses fields that OKX may leave empty, by name; an empty one reads as zero and a
// present non-decimal fails. coin, when set, names the row in the error.
func optionalFields(where, coin string, fields ...field) (map[string]*big.Rat, error) {
	out := make(map[string]*big.Rat, len(fields))
	for _, f := range fields {
		name := f.name
		if coin != "" {
			name += " of " + coin
		}
		value, err := adapters.OptionalDecimal(where, name, f.value)
		if err != nil {
			return nil, err
		}
		out[f.name] = value
	}
	return out, nil
}

type fundingBalance struct {
	Ccy      string `json:"ccy"`
	Bal      string `json:"bal"`
	AvailBal string `json:"availBal"`
}

// funding maps the funding account, which OKX reports without any USD valuation. Like `eq`, an
// empty `bal` fails the account instead of reading as zero.
func (a *Adapter) funding(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var rows []fundingBalance
	if err := c.call(ctx, epFunding, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	where := epFunding.String()
	balances := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		total, err := adapters.Decimal(where, "bal of "+row.Ccy, row.Bal)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		available, err := adapters.OptionalDecimal(where, "availBal of "+row.Ccy, row.AvailBal)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if total.Sign() == 0 && available.Sign() == 0 {
			continue
		}
		if strings.TrimSpace(row.Ccy) == "" {
			return adapters.AccountReport{}, adapters.Malformed(where, "the currency of a nonzero balance")
		}
		balances = append(balances, adapters.Balance{
			AssetSymbol:         row.Ccy,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: adapters.Reported(row.AvailBal, available),
		})
	}
	return adapters.AccountReport{AccountType: TypeFunding, Status: adapters.StatusSucceeded, CollectedAt: a.now(), Balances: balances}, nil
}
