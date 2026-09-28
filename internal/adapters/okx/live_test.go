//go:build live

package okx

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/livetest"
	"github.com/rbee3u/plutus/internal/decimal"
)

// TestLiveOKX calls the real endpoints with the test environment's read-only key set and asserts
// structure and success, never specific numbers. The Margin Mode is read from the account, never
// assumed. It then compares every collected Account with OKX's own valuation of the same bucket, so
// value the adapter does not collect (a trading or funding bucket OKX adds later) fails here
// instead of vanishing under the Complete Set rule; for Earn, Collect runs the same check itself.
func TestLiveOKX(t *testing.T) {
	creds := livetest.Credentials(t, adapters.PlatformOKX, true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	a := New(nil)

	result, err := a.Validate(ctx, creds, adapters.StructureOKXUnified)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.ExternalRef == "" || result.MarginMode == "" || !adapters.ValidMarginMode(adapters.StructureOKXUnified, result.MarginMode) {
		t.Fatalf("validate returned %+v", result)
	}
	mode := result.MarginMode
	t.Logf("account structure %s, margin mode %s", adapters.StructureOKXUnified, mode)
	if _, err := a.Validate(ctx, creds, adapters.StructureBybitUTA2); err == nil {
		t.Fatal("another Platform's Account Structure must be refused")
	}

	batch, err := a.Collect(ctx, creds, adapters.StructureOKXUnified, allTypes)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if batch.MarginMode != mode {
		t.Fatalf("collect read margin mode %s, validate %s", batch.MarginMode, mode)
	}
	if len(batch.Reports) != len(allTypes) {
		t.Fatalf("%d reports, want %d", len(batch.Reports), len(allTypes))
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
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
			if p.PlatformPositionRef == "" || p.Instrument.Kind == "" || p.QuantityUnit != adapters.UnitContracts || !decimal.Valid(p.Quantity) {
				t.Errorf("%s position %+v", report.AccountType, p)
			}
		}
		t.Logf("%s: %d balances, %d positions, equity reported: %v", report.AccountType, len(report.Balances), len(report.Positions), report.Equity != nil)
	}
	trading := reportOf(t, batch, TypeTrading)
	if trading.Equity == nil {
		t.Errorf("trading report has no equity: %+v", trading)
	}
	switch mode {
	case adapters.MarginOKXMCM, adapters.MarginOKXPM:
		if trading.AdjustedEquity == nil || trading.AvailableEquity == nil {
			t.Errorf("%s trading report lacks the adjusted or available equity: %+v", mode, trading)
		}
	case adapters.MarginOKXFutures:
		if trading.AdjustedEquity != nil || trading.UnifiedMarginRatio != nil {
			t.Errorf("futures mode reports no account-level margin figures, got %+v", trading)
		}
	}
	for _, p := range batch.Prices {
		if !decimal.Valid(p.Price) || (p.QuoteSymbol != "USDT" && p.QuoteSymbol != "USDC") {
			t.Errorf("price %+v", p)
		}
	}
	t.Logf("%d prices; the Earn check passed", len(batch.Prices))

	// Collect has just read the dual investment history and the valuation, both limited to one
	// request per second, so each is read again only after a pause.
	c := a.client(creds)
	if err := a.wait(ctx); err != nil {
		t.Fatal(err)
	}
	orders, err := a.dualInvestmentOrders(ctx, c)
	if err != nil {
		t.Fatalf("dual investment history: %v", err)
	}
	invested := 0
	for i := range orders {
		if holdsPrincipal, _ := dcdPrincipalInvested(orders[i].State); holdsPrincipal {
			invested++
		}
	}
	t.Logf("dual investment: %d orders in the history, %d still invested", len(orders), invested)
	dual := reportOf(t, batch, TypeDualInvestment)
	for i := range dual.DualPositions {
		position := &dual.DualPositions[i]
		if _, err := adapters.NormalizeDualPosition("live", position); err != nil {
			t.Error(err)
		}
	}
	if len(dual.DualPositions) == 0 {
		t.Log("Dual Investment: empty live coverage only")
	}

	if err := a.wait(ctx); err != nil {
		t.Fatal(err)
	}
	compareWithOKXValuation(ctx, t, c, &batch)
}

// compareWithOKXValuation checks each collected Account against OKX's valuation of its bucket in
// USDT: the trading account's reported equity, and the funding and earn Balances priced as
// valuation prices them (USD, USDT, and USDC at 1, other coins at their batch price). Every other
// bucket must be empty, because nothing collects it.
func compareWithOKXValuation(ctx context.Context, t *testing.T, c *client, batch *adapters.Batch) {
	t.Helper()
	var rows []struct {
		Details map[string]string `json:"details"`
	}
	if err := c.call(ctx, epValuation, &rows); err != nil {
		t.Fatalf("asset valuation: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("asset valuation rows %d", len(rows))
	}
	buckets := map[string]*big.Rat{}
	for name, value := range rows[0].Details {
		amount, ok := decimal.Parse(value)
		if !ok {
			t.Fatalf("asset valuation bucket %s is not a decimal", name)
		}
		buckets[name] = amount
	}

	prices := map[string]*big.Rat{}
	for _, p := range batch.Prices {
		prices[p.AssetSymbol] = decimal.MustParse(p.Price)
	}
	collected := map[string]*big.Rat{}
	var unpriced []string
	for _, report := range batch.Reports {
		if report.Equity != nil {
			collected[string(report.AccountType)] = decimal.MustParse(*report.Equity)
			continue
		}
		sum := new(big.Rat)
		for _, b := range report.Balances {
			symbol := strings.ToUpper(b.Symbol)
			price, ok := big.NewRat(1, 1), true
			if !adapters.USDEquivalent(symbol) {
				price, ok = prices[symbol]
			}
			if !ok {
				unpriced = append(unpriced, fmt.Sprintf("%s in %s", b.Symbol, report.AccountType))
				continue
			}
			sum.Add(sum, new(big.Rat).Mul(decimal.MustParse(b.Equity), price))
		}
		key := string(report.AccountType)
		if report.AccountType == TypeDualInvestment {
			key = string(TypeEarn)
		}
		if collected[key] == nil {
			collected[key] = new(big.Rat)
		}
		collected[key].Add(collected[key], sum)
	}

	// Compare the independent Dual principal against the aggregate's residual.
	dualUSD := new(big.Rat)
	for _, report := range batch.Reports {
		if report.AccountType != TypeDualInvestment {
			continue
		}
		for _, b := range report.Balances {
			unit := big.NewRat(1, 1)
			if !adapters.USDEquivalent(b.Symbol) {
				unit = prices[b.Symbol]
			}
			if unit != nil {
				dualUSD.Add(dualUSD, decimal.Mul(decimal.MustParse(b.Equity), unit))
			}
		}
	}
	if combined, ok := collected[string(TypeEarn)]; ok && buckets[string(TypeEarn)] != nil {
		ordinary := decimal.Sub(combined, dualUSD)
		residual := decimal.Sub(buckets[string(TypeEarn)], ordinary)
		t.Logf("Dual principal %s USD; OKX Earn minus ordinary Earn %s USD; residual difference %s USD; %d normalized subscriptions", dualUSD.FloatString(2), residual.FloatString(2), decimal.Sub(residual, dualUSD).FloatString(2), len(reportOf(t, *batch, TypeDualInvestment).DualPositions))
		if dualUSD.Sign() > 0 && !withinBucketTolerance(ordinary, buckets[string(TypeEarn)]) {
			t.Log("The active Dual principal is material: ordinary Earn alone does not explain OKX's Earn bucket.")
		}
	}
	names := make([]string, 0, len(buckets))
	for name := range buckets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ours, known := collected[name]
		if !known {
			ours = new(big.Rat)
		}
		theirs := buckets[name]
		if !withinBucketTolerance(ours, theirs) {
			t.Errorf("%s: collected %s USD, OKX values the bucket at %s USD (off by %s); unpriced: %v",
				name, ours.FloatString(2), theirs.FloatString(2), gap(ours, theirs), unpriced)
			continue
		}
		t.Logf("%s: collected value is off by %s", name, gap(ours, theirs))
	}
	for _, accountType := range allTypes {
		if accountType == TypeDualInvestment {
			continue
		}
		if _, ok := buckets[string(accountType)]; !ok {
			t.Errorf("asset valuation has no %s bucket", accountType)
		}
	}
}

// withinBucketTolerance allows 1% of OKX's figure, and at least 1 USD so a near-empty bucket does
// not fail on rounding: prices move between the calls, and OKX values USDC at market instead of
// at 1.
func withinBucketTolerance(ours, theirs *big.Rat) bool {
	limit := new(big.Rat).Mul(new(big.Rat).Abs(theirs), big.NewRat(1, 100))
	if floor := big.NewRat(1, 1); limit.Cmp(floor) < 0 {
		limit = floor
	}
	return new(big.Rat).Abs(new(big.Rat).Sub(ours, theirs)).Cmp(limit) <= 0
}

// gap is the relative difference from OKX's figure, or the absolute one for an empty bucket.
func gap(ours, theirs *big.Rat) string {
	if theirs.Sign() == 0 {
		return ours.FloatString(2) + " USD against an empty bucket"
	}
	ratio, _ := new(big.Rat).Quo(new(big.Rat).Sub(ours, theirs), theirs).Float64()
	return fmt.Sprintf("%+.3f%%", ratio*100)
}
