package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

func TestCollectIsDeterministic(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	a := New(adapter.PlatformBinance).WithClock(func() time.Time { return at })
	creds := adapter.Credentials{APIKey: "key-alice", APISecret: "secret"}
	first, err := a.Collect(context.Background(), creds, adapter.StructureBinancePM, []adapter.AccountType{"portfolio_margin", "spot"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Collect(context.Background(), creds, adapter.StructureBinancePM, []adapter.AccountType{"portfolio_margin", "spot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Reports) != 2 || first.Reports[0].Equity == nil || *first.Reports[0].Equity != *second.Reports[0].Equity {
		t.Fatalf("reports differ: %+v vs %+v", first.Reports, second.Reports)
	}
	if len(first.Prices) != 4 {
		t.Fatalf("prices=%d", len(first.Prices))
	}
	if first.MarginMode != "" {
		t.Fatalf("Binance has no margin mode, got %q", first.MarginMode)
	}
}

func TestEveryAccountTypeOfEveryStructureReports(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	creds := adapter.Credentials{APIKey: "key-alice", APISecret: "secret", Futures: &adapter.KeyPair{APIKey: "fk", APISecret: "fs"}}
	for _, info := range adapter.NewRegistry().Platforms() {
		a := New(info.ID).WithClock(func() time.Time { return at })
		for _, structure := range info.Structures {
			batch, err := a.Collect(context.Background(), creds, structure.ID, adapter.AccountTypeIDs(structure.ID))
			if err != nil {
				t.Fatalf("%s: %v", structure.ID, err)
			}
			if !adapter.ValidMarginMode(structure.ID, batch.MarginMode) {
				t.Fatalf("%s: margin mode %q does not belong to the structure", structure.ID, batch.MarginMode)
			}
			for _, report := range batch.Reports {
				if report.Status != adapter.StatusSucceeded || len(report.Balances) == 0 {
					t.Fatalf("%s/%s: %+v", structure.ID, report.AccountType, report)
				}
			}
		}
	}
}

func TestMarginModeFollowsTheKey(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	if MarginMode(adapter.StructureOKXUnified, "k", at) != MarginMode(adapter.StructureOKXUnified, "k", at.Add(time.Hour)) {
		t.Fatal("an ordinary key keeps its margin mode")
	}
	if MarginMode(adapter.StructureBybitUTA2, "switch-k", at) == MarginMode(adapter.StructureBybitUTA2, "switch-k", at.Add(time.Minute)) {
		t.Fatal("a switch- key changes its margin mode every minute")
	}
	isolated := Scenario("k", at).Report(adapter.MarginBybitIsolated, "unified", at, false)
	if isolated.Equity != nil || isolated.ReportedMarginRatio != nil || len(isolated.Balances) == 0 {
		t.Fatal("Isolated margin reports no account-wide figures")
	}
	pm := Scenario("k", at).Report(adapter.MarginBybitPM, "unified", at, false)
	if pm.Positions[0].Leverage != nil || pm.Positions[0].LiquidationPrice != nil || pm.Equity == nil {
		t.Fatal("Portfolio margin positions carry no leverage or liquidation price")
	}
}

func TestBehaviorPrefixesKeepTheAccount(t *testing.T) {
	t.Parallel()
	if ExternalRef("flaky-alice") != ExternalRef("alice") || ExternalRef("switch-alice") != ExternalRef("alice") {
		t.Fatal("a behavior prefix reaches the same account")
	}
	if ExternalRef("bob") == ExternalRef("alice") {
		t.Fatal("another key reaches another account")
	}
}

func TestKeyPrefixesReproduceFailureClasses(t *testing.T) {
	t.Parallel()
	a := New(adapter.PlatformOKX)
	for key, class := range map[string]string{
		"invalid-x": adapter.ClassAuthentication, "forbidden-x": adapter.ClassAuthorization,
		"ratelimit-x": adapter.ClassRateLimited, "restructured-x": adapter.ClassUnsupported,
	} {
		_, err := a.Validate(context.Background(), adapter.Credentials{APIKey: key, APISecret: "s"}, adapter.StructureOKXUnified)
		var adapterErr *adapter.Error
		if !errors.As(err, &adapterErr) || adapterErr.Class != class {
			t.Fatalf("%s: got %v", key, err)
		}
	}
	if _, err := a.Validate(context.Background(), adapter.Credentials{APIKey: "k", APISecret: "s"}, adapter.StructureBinancePM); err == nil {
		t.Fatal("a structure of another platform must be unsupported")
	}
	batch, err := a.Collect(context.Background(), adapter.Credentials{APIKey: "k", APISecret: "s"}, adapter.StructureOKXUnified, []adapter.AccountType{"trading", "spot"})
	if err != nil || batch.Reports[1].Status != adapter.StatusFailed || batch.Reports[1].ErrorClass != adapter.ClassUnsupported {
		t.Fatalf("an account type outside the structure fails only its report: %+v %v", batch.Reports, err)
	}
}
