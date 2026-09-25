//go:build live

package kraken

import (
	"context"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/livetest"
	"github.com/rbee3u/plutus/internal/decimal"
)

// TestLiveKraken calls the real endpoints with the test environment's read-only key sets and
// asserts structure and success, never specific numbers. The Account Structure is read from the
// account's balances feed, never assumed; the other structure has documented coverage only
// (ADR 0027), and here it must be refused before any Account is read.
func TestLiveKraken(t *testing.T) {
	creds := livetest.KrakenCredentials(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	a := New(nil)

	s, err := a.client(creds).balancesSnapshot(ctx)
	if err != nil {
		t.Fatalf("balances snapshot: %v", err)
	}
	structure, err := structureOf(&s)
	if err != nil {
		t.Fatalf("account structure: %v", err)
	}
	other := adapters.StructureKrakenSeparate
	if structure == other {
		other = adapters.StructureKrakenUnified
	}
	t.Logf("account structure %s; %s has documented coverage only", structure, other)

	result, err := a.Validate(ctx, creds, structure)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.ExternalRef == "" || result.MarginMode != "" {
		t.Fatal("validate must return the customer reference and no Margin Mode")
	}
	// Kraken documents the key check's iiban as nullable; without it a Futures-only Identity is
	// bound to the accountUid, and adding the Spot keys later changes its reference.
	alone, err := a.Validate(ctx, adapters.Credentials{Futures: creds.Futures}, structure)
	if err != nil || alone.ExternalRef == "" {
		t.Fatalf("validate with the Futures keys alone: %v", err)
	}
	t.Logf("the Futures key check names the customer reference: %v", alone.ExternalRef == result.ExternalRef)
	if _, err := a.Validate(ctx, creds, other); classOf(err) != adapters.ClassUnsupported {
		t.Fatalf("validate as %s: %v", other, err)
	}
	if _, err := a.Collect(ctx, creds, other, adapters.AccountTypeIDs(other)); classOf(err) != adapters.ClassUnsupported {
		t.Fatalf("collect as %s: %v", other, err)
	}
	if _, err := a.Validate(ctx, creds, adapters.StructureOKXUnified); classOf(err) != adapters.ClassUnsupported {
		t.Fatalf("another Platform's Account Structure must be refused: %v", err)
	}

	accounts := adapters.AccountTypeIDs(structure)
	batch, err := a.Collect(ctx, creds, structure, accounts)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if batch.MarginMode != "" || len(batch.Reports) != len(accounts) {
		t.Fatalf("%d reports for %d Accounts, margin mode %q", len(batch.Reports), len(accounts), batch.MarginMode)
	}
	for i := range batch.Reports {
		report := &batch.Reports[i]
		if report.AccountType != accounts[i] || report.Status != adapters.StatusOK {
			t.Fatalf("%s failed: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
		}
		if report.ObservedAt.IsZero() {
			t.Errorf("%s has no observation time", report.AccountType)
		}
		for _, b := range report.Balances {
			if b.Symbol == "" || b.Name != "" || !decimal.Valid(b.Equity) {
				t.Errorf("%s has an invalid balance", report.AccountType)
			}
		}
		for j := range report.Positions {
			p := &report.Positions[j]
			if p.PlatformPositionRef == "" || p.Instrument.Kind == "" || p.Instrument.ContractSize == nil ||
				p.QuantityUnit != adapters.UnitContracts || !decimal.Valid(p.Quantity) {
				t.Errorf("%s has an invalid position", report.AccountType)
			}
		}
		multiCollateral := report.AccountType == TypeMultiM || (report.AccountType == TypeMain && structure == adapters.StructureKrakenUnified)
		if multiCollateral && (report.Equity == nil || report.AdjustedEquity == nil) {
			t.Errorf("%s lacks the multi-collateral wallet's equity figures", report.AccountType)
		}
		if report.AccountType == TypeCoinM && report.Equity == nil {
			t.Error("coin_m has no equity")
		}
		t.Logf("%s: %d balances, %d positions, equity reported: %v", report.AccountType, len(report.Balances), len(report.Positions), report.Equity != nil)
	}
	for _, p := range batch.Prices {
		if !decimal.Valid(p.Price) || !isPriceQuote(p.QuoteSymbol) || adapters.USDEquivalent(p.AssetSymbol) {
			t.Errorf("invalid price row for %s", p.AssetSymbol)
		}
	}
	t.Logf("%d prices", len(batch.Prices))
}
