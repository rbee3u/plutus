//go:build live

package binance

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/livetest"
	"github.com/rbee3u/plutus/internal/decimal"
)

// TestLiveBinance calls the real endpoints with the test environment's read-only key set. It reads
// the key's Account Structure first, so it runs against a Classic Trading or a Portfolio Margin
// account alike, and asserts structure and success only, never specific numbers.
func TestLiveBinance(t *testing.T) {
	creds := livetest.Credentials(t, adapters.PlatformBinance, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	a := New(nil)
	c := a.client(creds)

	var info accountInfo
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		t.Fatalf("account info: %v", err)
	}
	structure, err := info.structure()
	if err != nil {
		t.Fatalf("account info: %v", err)
	}
	t.Logf("account structure: %s (margin opened %v, futures opened %v)", adapters.StructureName(structure), !info.marginClosed(), !info.futuresClosed())

	result, err := a.Validate(ctx, creds, structure)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.ExternalRef == "" || result.MarginMode != "" {
		t.Fatalf("validation returned UID %v and margin mode %q", result.ExternalRef != "", result.MarginMode)
	}
	other := adapters.StructureBinancePM
	if structure == adapters.StructureBinancePM {
		other = adapters.StructureBinanceClassic
	}
	if _, err := a.Validate(ctx, creds, other); err == nil || classOfErr(t, err) != adapters.ClassUnsupported {
		t.Errorf("validate as %s: %v, want a structure mismatch", other, err)
	}

	accounts := adapters.AccountTypeIDs(structure)
	batch, err := a.Collect(ctx, creds, structure, accounts)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(batch.Reports) != len(accounts) || batch.MarginMode != "" {
		t.Fatalf("%d reports for %d Account Types, margin mode %q", len(batch.Reports), len(accounts), batch.MarginMode)
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Errorf("%s failed: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
			continue
		}
		if report.ObservedAt.IsZero() {
			t.Errorf("%s has no observation time", report.AccountType)
		}
		for _, b := range report.Balances {
			if b.Symbol == "" || !decimal.Valid(b.Equity) {
				t.Errorf("%s balance %+v", report.AccountType, b)
			}
		}
		for _, p := range report.Positions {
			if p.PlatformPositionRef == "" || p.Instrument.Kind == "" || !decimal.Valid(p.Quantity) {
				t.Errorf("%s position %+v", report.AccountType, p)
			}
		}
		t.Logf("%s: %d balances, %d positions, equity reported: %v", report.AccountType, len(report.Balances), len(report.Positions), report.Equity != nil)
	}
	if structure == adapters.StructureBinancePM {
		if pm := reportOf(t, batch, TypePortfolioMargin); pm.Status == adapters.StatusOK && (pm.Equity == nil || pm.AdjustedEquity == nil || pm.UnifiedMarginRatio == nil) {
			t.Errorf("portfolio margin report lacks account figures: %+v", pm)
		}
	}
	if len(batch.Prices) == 0 {
		t.Error("no prices returned")
	}
	for _, p := range batch.Prices {
		if !decimal.Valid(p.Price) || (p.QuoteSymbol != "USDT" && p.QuoteSymbol != "USDC") {
			t.Errorf("price %+v", p)
		}
	}
	t.Logf("%d prices", len(batch.Prices))

	prices, _ := a.lastPrices(ctx, c)
	logEarnSources(ctx, t, c, reportOf(t, batch, TypeEarn), prices)
}

// logEarnSources reports whether the dual investment and wallet valuation endpoints answer the
// key, and both sides of the Earn completeness check (ADR 0029).
func logEarnSources(ctx context.Context, t *testing.T, c *client, earn adapters.AccountReport, prices lastPrices) {
	t.Helper()
	running, answered := 0, 0
	for _, state := range runningDualInvestmentStates() {
		var page dualInvestmentPage
		if err := c.call(ctx, epDualInvest, dualInvestmentQuery(state, 1), &page); err != nil {
			t.Errorf("dual investment %s: %v", state, err)
			continue
		}
		answered++
		running += page.Total
	}
	t.Logf("dual investment answered %d of %d running states: %d running positions", answered, len(runningDualInvestmentStates()), running)

	var wallets []struct {
		Balance    string `json:"balance"`
		WalletName string `json:"walletName"`
	}
	if err := c.call(ctx, epWalletValue, walletValueQuery(), &wallets); err != nil {
		t.Errorf("wallet valuation: %v", err)
		return
	}
	names := make([]string, 0, len(wallets))
	var valuation *big.Rat
	for _, wallet := range wallets {
		names = append(names, wallet.WalletName)
		if wallet.WalletName == earnWallet {
			valuation, _ = decimal.Parse(wallet.Balance)
		}
	}
	t.Logf("wallet valuation answered with wallets %s", strings.Join(names, ", "))
	if valuation == nil {
		t.Errorf("wallet valuation has no decimal %s balance", earnWallet)
		return
	}
	if earn.Status != adapters.StatusOK {
		t.Logf("Earn check: Binance values Earn at %s USD; the Earn Account failed: %s", valuation.FloatString(2), earn.ErrorDetail)
		return
	}
	itemized := new(big.Rat)
	for _, b := range earn.Balances {
		unit, ok := prices.usd(b.Symbol)
		if !ok {
			t.Logf("Earn check skipped: no price for %s", b.Symbol)
			return
		}
		itemized.Add(itemized, new(big.Rat).Mul(decimal.MustParse(b.Equity), unit))
	}
	t.Logf("Earn check passed: Binance values Earn at %s USD, Plutus itemizes %s USD", valuation.FloatString(2), itemized.FloatString(2))
}
