//go:build live

package bybit

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/livetest"
	"github.com/rbee3u/plutus/internal/decimal"
)

// overviewBuckets maps each Account Type to its asset-overview account type.
var overviewBuckets = map[adapters.AccountType]string{
	TypeUnified: "UnifiedTradingAccount",
	TypeFunding: "FundingAccount",
	TypeEarn:    earnBucket,
}

func TestLiveBybit(t *testing.T) {
	creds := livetest.Credentials(t, adapters.PlatformBybit, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	a := New(nil)
	structure := adapters.StructureBybitUTA2
	result, err := a.Validate(ctx, creds, structure)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.ExternalRef == "" || result.MarginMode == "" || !adapters.ValidMarginMode(structure, result.MarginMode) {
		t.Fatalf("validation returned reference %t and Margin Mode %q", result.ExternalRef != "", result.MarginMode)
	}
	t.Logf("Margin Mode: %s", result.MarginMode)
	accounts := adapters.AccountTypeIDs(structure)
	batch, err := a.Collect(ctx, creds, structure, accounts)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if batch.MarginMode != result.MarginMode || len(batch.Reports) != len(accounts) {
		t.Fatalf("batch Margin Mode %q with %d reports", batch.MarginMode, len(batch.Reports))
	}
	for i := range batch.Reports {
		report := &batch.Reports[i]
		if report.Status != adapters.StatusOK {
			t.Errorf("%s: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
			continue
		}
		for _, balance := range report.Balances {
			if balance.Symbol == "" || !decimal.Valid(balance.Equity) || balance.Name != "" {
				t.Errorf("%s: invalid balance", report.AccountType)
			}
		}
		if report.AccountType == TypeUnified {
			checkUnified(t, report, batch.MarginMode)
		}
		t.Logf("%s: %d balances, %d positions", report.AccountType, len(report.Balances), len(report.Positions))
	}
	c := a.client(creds)
	positions, err := pages[dualAssetPosition](ctx, c, epDualAssets, nil)
	if err != nil {
		t.Errorf("dual assets: %v", err)
	}
	t.Logf("Dual Asset positions: %d", len(positions))
	compareOverview(ctx, t, c, &batch)
}

// checkUnified checks what each Margin Mode documents: no account-wide figures in Isolated
// Margin, no leverage, margin, or liquidation price on Portfolio Margin positions.
func checkUnified(t *testing.T, report *adapters.AccountReport, mode adapters.MarginMode) {
	t.Helper()
	switch mode {
	case adapters.MarginBybitIsolated:
		if report.Equity != nil || report.AvailableEquity != nil || report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil {
			t.Error("Isolated Margin reports account-wide figures")
		}
	case adapters.MarginBybitCross, adapters.MarginBybitPM:
		if report.Equity == nil {
			t.Error("Unified equity is missing")
		}
	}
	for i := range report.Positions {
		position := &report.Positions[i]
		if position.Quantity == "" || position.Instrument.Kind == "" || position.MarkPrice == nil {
			t.Errorf("%s: incomplete position", position.PlatformPositionRef)
		}
		if mode == adapters.MarginBybitPM && (position.Leverage != nil || position.Margin != nil || position.LiquidationPrice != nil) {
			t.Errorf("%s: a Portfolio Margin position carries its own margin figures", position.PlatformPositionRef)
		}
	}
}

// compareOverview compares every collected Account with Bybit's own USD valuation of it in
// asset-overview, within 1% and at least 1 USD, and fails on a populated bucket Plutus does not
// collect. The runtime Earn check covers Earn only; this also covers Unified and Funding.
func compareOverview(ctx context.Context, t *testing.T, c *client, batch *adapters.Batch) {
	t.Helper()
	var overview struct {
		List []struct {
			AccountType       string `json:"accountType"`
			TotalEquity       string `json:"totalEquity"`
			ValuationCurrency string `json:"valuationCurrency"`
		} `json:"list"`
	}
	if err := c.call(ctx, epAssetOverview, nil, &overview); err != nil {
		t.Fatalf("asset overview: %v", err)
	}
	prices := spotPrices{}
	for _, price := range batch.Prices {
		prices[price.AssetSymbol+price.QuoteSymbol] = price.Price
	}
	collected := map[string]*adapters.AccountReport{}
	for i := range batch.Reports {
		collected[overviewBuckets[batch.Reports[i].AccountType]] = &batch.Reports[i]
	}
	for _, bucket := range overview.List {
		valued, ok := decimal.Parse(bucket.TotalEquity)
		if !ok || bucket.ValuationCurrency != "USD" {
			t.Fatalf("%s: invalid asset-overview valuation", bucket.AccountType)
		}
		report, ok := collected[bucket.AccountType]
		if !ok {
			if valued.Sign() != 0 {
				t.Errorf("Bybit values %s, which Plutus does not collect, at more than zero", bucket.AccountType)
			}
			continue
		}
		delete(collected, bucket.AccountType)
		if report.Status != adapters.StatusOK {
			continue
		}
		itemized := new(big.Rat)
		if report.Equity != nil {
			itemized = decimal.MustParse(*report.Equity)
		} else {
			for _, balance := range report.Balances {
				unit, ok := prices.usd(balance.Symbol)
				if !ok {
					t.Errorf("%s: %s has no price", report.AccountType, balance.Symbol)
					continue
				}
				itemized.Add(itemized, decimal.Mul(decimal.MustParse(balance.Equity), unit))
			}
		}
		tolerance := new(big.Rat).Quo(decimal.Abs(valued), big.NewRat(100, 1))
		if tolerance.Cmp(big.NewRat(1, 1)) < 0 {
			tolerance = big.NewRat(1, 1)
		}
		t.Logf("%s: Bybit %s USD, Plutus %s USD", report.AccountType, valued.FloatString(2), itemized.FloatString(2))
		if decimal.Abs(decimal.Sub(itemized, valued)).Cmp(tolerance) > 0 {
			t.Errorf("%s differs from Bybit's valuation by more than 1%% (at least 1 USD); uncollected value may exist", report.AccountType)
		}
	}
	for name, report := range collected {
		if len(report.Balances) > 0 {
			t.Errorf("asset-overview omits the populated %s account", name)
		}
	}
}
