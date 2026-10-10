package binance

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Portfolio Margin merges Cross Margin, USDⓈ-M Futures, and COIN-M Futures into one Account that
// Binance values itself: /papi/v1/account reports its USD equity and margin figures.

type pmAccount struct {
	AccountEquity        string `json:"accountEquity"`
	ActualEquity         string `json:"actualEquity"`
	AccountInitialMargin string `json:"accountInitialMargin"`
	AccountMaintMargin   string `json:"accountMaintMargin"`
	TotalAvailable       string `json:"totalAvailableBalance"`
	UniMMR               string `json:"uniMMR"`
}

type pmBalance struct {
	Asset               string `json:"asset"`
	TotalWalletBalance  string `json:"totalWalletBalance"`
	CrossMarginFree     string `json:"crossMarginFree"`
	CrossMarginLocked   string `json:"crossMarginLocked"`
	CrossMarginBorrowed string `json:"crossMarginBorrowed"`
	CrossMarginInterest string `json:"crossMarginInterest"`
	UMWalletBalance     string `json:"umWalletBalance"`
	UMUnrealizedPNL     string `json:"umUnrealizedPNL"`
	CMWalletBalance     string `json:"cmWalletBalance"`
	CMUnrealizedPNL     string `json:"cmUnrealizedPNL"`
}

func (a *Adapter) portfolioMargin(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var account pmAccount
	if err := c.call(ctx, epPMAccount, nil, &account); err != nil {
		return adapters.AccountReport{}, err
	}
	var balances []pmBalance
	if err := c.call(ctx, epPMBalance, nil, &balances); err != nil {
		return adapters.AccountReport{}, err
	}
	var um, cm []positionRisk
	if err := c.call(ctx, epPMUMPositionRisk, nil, &um); err != nil {
		return adapters.AccountReport{}, err
	}
	if err := c.call(ctx, epPMCMPositionRisk, nil, &cm); err != nil {
		return adapters.AccountReport{}, err
	}
	report, err := mapPMAccount(&account)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.ObservedAt = a.now()
	if report.Balances, err = mapPMBalances(balances); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c,
		positionSource{marketUM, epPMUMPositionRisk, um},
		positionSource{marketCM, epPMCMPositionRisk, cm}); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

func mapPMAccount(in *pmAccount) (adapters.AccountReport, error) {
	where := epPMAccount.String()
	fields := map[string]string{
		"actualEquity": in.ActualEquity, "accountEquity": in.AccountEquity, "totalAvailableBalance": in.TotalAvailable,
		"accountInitialMargin": in.AccountInitialMargin, "accountMaintMargin": in.AccountMaintMargin, "uniMMR": in.UniMMR,
	}
	parsed := map[string]*big.Rat{}
	for name, value := range fields {
		rat, err := adapters.Decimal(where, name, value)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		parsed[name] = rat
	}
	report := adapters.AccountReport{
		AccountType:        TypePortfolioMargin,
		Status:             adapters.StatusOK,
		Equity:             adapters.Plain(in.ActualEquity),
		AdjustedEquity:     adapters.Plain(in.AccountEquity),
		AvailableEquity:    adapters.Plain(in.TotalAvailable),
		UnifiedMarginRatio: adapters.Plain(in.UniMMR),
	}
	if adjusted := parsed["accountEquity"]; adjusted.Sign() > 0 {
		report.InitialMarginRatio = adapters.Ratio(parsed["accountInitialMargin"], adjusted)
		report.MaintenanceMarginRatio = adapters.Ratio(parsed["accountMaintMargin"], adjusted)
	}
	return report, nil
}

// mapPMBalances maps /papi/v1/balance rows. Equity is the net coin amount: wallet balance across
// cross margin, UM, and CM, plus unrealized PnL, minus borrowed principal and interest. It may be
// negative. negativeBalance is already inside totalWalletBalance and is not subtracted again.
func mapPMBalances(rows []pmBalance) ([]adapters.Balance, error) {
	where := epPMBalance.String()
	out := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		parsed, nonzero, err := amounts(where, map[string]string{
			"totalWalletBalance": row.TotalWalletBalance, "crossMarginBorrowed": row.CrossMarginBorrowed, "crossMarginInterest": row.CrossMarginInterest,
			"umUnrealizedPNL": row.UMUnrealizedPNL, "cmUnrealizedPNL": row.CMUnrealizedPNL, "crossMarginFree": row.CrossMarginFree,
			"crossMarginLocked": row.CrossMarginLocked, "umWalletBalance": row.UMWalletBalance, "cmWalletBalance": row.CMWalletBalance,
		})
		if err != nil {
			return nil, err
		}
		if !nonzero {
			continue
		}
		symbol, err := coin(where, row.Asset)
		if err != nil {
			return nil, err
		}
		pnl := decimal.Add(parsed["umUnrealizedPNL"], parsed["cmUnrealizedPNL"])
		equity := decimal.Sub(decimal.Sub(decimal.Add(parsed["totalWalletBalance"], pnl), parsed["crossMarginBorrowed"]), parsed["crossMarginInterest"])
		out = append(out, adapters.Balance{
			AssetSymbol:   symbol,
			Equity:        decimal.Format(equity),
			Liability:     decimal.FormatPtr(parsed["crossMarginBorrowed"]),
			Interest:      decimal.FormatPtr(parsed["crossMarginInterest"]),
			UnrealizedPnl: decimal.FormatPtr(pnl),
		})
	}
	return out, nil
}

// positionRisk is one row of a position endpoint: /papi/v1/um/positionRisk and
// /papi/v1/cm/positionRisk in Portfolio Margin, /fapi/v3/positionRisk and /dapi/v1/positionRisk in
// Classic Trading. /fapi/v3/positionRisk has no leverage; withLeverage fills it in.
type positionRisk struct {
	Symbol                string `json:"symbol"`
	PositionAmt           string `json:"positionAmt"`
	PositionSide          string `json:"positionSide"`
	EntryPrice            string `json:"entryPrice"`
	MarkPrice             string `json:"markPrice"`
	LiquidationPrice      string `json:"liquidationPrice"`
	UnRealizedProfit      string `json:"unRealizedProfit"`
	Leverage              string `json:"leverage"`
	MarginType            string `json:"marginType"`
	MarginAsset           string `json:"marginAsset"`
	PositionInitialMargin string `json:"positionInitialMargin"`
	IsolatedWallet        string `json:"isolatedWallet"`
}

// positionSource is the rows of one position endpoint and the market they trade in.
type positionSource struct {
	market market
	ep     endpoint
	rows   []positionRisk
}

// positions maps every open position. A zero position is omitted; an open one whose contract is
// unknown fails the Account instead of being dropped.
func (a *Adapter) positions(ctx context.Context, c *client, sources ...positionSource) ([]adapters.Position, error) {
	out := make([]adapters.Position, 0)
	for _, source := range sources {
		for i := range source.rows {
			row := &source.rows[i]
			amount, err := adapters.Decimal(source.ep.String(), "positionAmt", row.PositionAmt)
			if err != nil {
				return nil, err
			}
			if amount.Sign() == 0 {
				continue
			}
			futuresContract, err := a.futuresContracts.lookup(ctx, c, source.market, row.Symbol)
			if err != nil {
				return nil, err
			}
			position, err := mapPosition(source.market, source.ep, row, amount, &futuresContract)
			if err != nil {
				return nil, err
			}
			out = append(out, position)
		}
	}
	return out, nil
}

// mapPosition maps one open position. USDⓈ-M quantities are in base units, COIN-M quantities in
// contracts.
func mapPosition(m market, ep endpoint, row *positionRisk, amount *big.Rat, futuresContract *adapters.FuturesContract) (adapters.Position, error) {
	unit := adapters.UnitBase
	if m == marketCM {
		unit = adapters.UnitContracts
	}
	if _, _, err := amounts(ep.String(), map[string]string{
		"entryPrice": row.EntryPrice, "markPrice": row.MarkPrice, "liquidationPrice": row.LiquidationPrice,
		"unRealizedProfit": row.UnRealizedProfit, "leverage": row.Leverage,
		"positionInitialMargin": row.PositionInitialMargin, "isolatedWallet": row.IsolatedWallet,
	}); err != nil {
		return adapters.Position{}, err
	}
	side := adapters.SideLong
	switch strings.ToUpper(row.PositionSide) {
	case "SHORT":
		side = adapters.SideShort
	case "LONG":
	case "BOTH":
		if amount.Sign() < 0 {
			side = adapters.SideShort
		}
	default:
		return adapters.Position{}, adapters.Malformed(ep.String(), "positionSide of "+row.Symbol)
	}
	mode := "hedged"
	if strings.EqualFold(row.PositionSide, "BOTH") {
		mode = "one_way"
	}
	out := adapters.Position{
		PlatformPositionRef: fmt.Sprintf("%s:%s:%s", m, row.Symbol, strings.ToUpper(row.PositionSide)),
		FuturesContract:     *futuresContract,
		Side:                side,
		PositionMode:        mode,
		Quantity:            decimal.Format(decimal.Abs(amount)),
		QuantityUnit:        unit,
		EntryPrice:          adapters.Plain(row.EntryPrice),
		MarkPrice:           adapters.Plain(row.MarkPrice),
		LiquidationPrice:    adapters.NonZero(row.LiquidationPrice),
		UnrealizedPnl:       adapters.Plain(row.UnRealizedProfit),
		Leverage:            adapters.Plain(row.Leverage),
	}
	if ep == epUMPositionRisk || ep == epCMPositionRisk {
		out.MarginMode = strings.ToLower(strings.TrimSpace(row.MarginType))
		switch out.MarginMode {
		case "cross":
			out.Margin, out.MarginBasis = adapters.Plain(row.PositionInitialMargin), "initial"
		case "isolated":
			out.Margin, out.MarginBasis = adapters.Plain(row.IsolatedWallet), "allocated"
		default:
			out.MarginMode = ""
		}
		out.MarginAssetSymbol = strings.TrimSpace(row.MarginAsset)
		if m == marketCM && out.MarginAssetSymbol == "" {
			out.MarginAssetSymbol = futuresContract.SettlementAssetSymbol
		}
	} else if ep == epPMUMPositionRisk || ep == epPMCMPositionRisk {
		// Retail Portfolio Margin futures use Cross and retain initial leverage.
		// These risk responses do not provide a position margin amount; the read
		// model may derive its initial requirement without allocating collateral.
		out.MarginMode = "cross"
	}
	return out, nil
}
