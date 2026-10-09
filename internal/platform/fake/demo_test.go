package fake

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func demoByID(t *testing.T, id string) DemoIdentity {
	t.Helper()
	for _, d := range DemoIdentities() {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("unknown scenario %s", id)
	return DemoIdentity{}
}

func TestDemoCapabilityMatrix(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	covered := map[adapters.AccountStructure]map[adapters.MarginMode]bool{}
	seen := map[string]bool{}
	for _, d := range DemoIdentities() {
		if err := ValidateDemoIdentity(d); err != nil {
			t.Fatal(err)
		}
		if seen[d.ID] {
			t.Fatal("duplicate Identity")
		}
		seen[d.ID] = true
		if covered[d.Structure] == nil {
			covered[d.Structure] = map[adapters.MarginMode]bool{}
		}
		covered[d.Structure][d.Mode] = true
		for _, at := range []time.Time{anchor.AddDate(0, 0, -400), anchor.AddDate(0, 0, -7), anchor, anchor.AddDate(0, 0, 65)} {
			batch, err := DemoBatch(d, anchor, at, adapters.AccountTypeIDs(d.Structure))
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range batch.Reports {
				if r.Status != adapters.StatusOK {
					continue
				}
				coins := map[string]bool{}
				for _, b := range r.Balances {
					if coins[b.PlatformAssetSymbol] || !decimal.Valid(b.Equity) {
						t.Fatalf("%s duplicate/invalid Balance %+v", d.ID, b)
					}
					coins[b.PlatformAssetSymbol] = true
				}
				if r.EarnDetailsCollected {
					if err := adapters.CheckEarnPositions(r.EarnPositions, r.Balances); err != nil {
						t.Fatalf("%s: %v", d.ID, err)
					}
				}
				for _, p := range r.Positions {
					if !decimal.Valid(p.Quantity) || decimal.ParseOrZero(p.Quantity).Sign() <= 0 {
						t.Fatal("invalid position quantity")
					}
					if p.FuturesContract.Kind == adapters.KindLinearDelivery || p.FuturesContract.Kind == adapters.KindInverseDelivery {
						if p.FuturesContract.ExpiryAt == nil {
							t.Fatal("delivery without expiry")
						}
					}
				}
			}
		}
	}
	for _, p := range adapters.NewRegistry().Platforms() {
		for _, s := range p.Structures {
			modes := s.MarginModes
			if len(modes) == 0 {
				if !covered[s.ID][""] {
					t.Fatalf("missing %s", s.ID)
				}
			}
			for _, m := range modes {
				if !covered[s.ID][m.ID] {
					t.Fatalf("missing %s/%s", s.ID, m.ID)
				}
			}
		}
	}
}

func TestDemoCredentialContinuity(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	d := demoByID(t, "daily-kraken")
	key := DemoKey(d.ID, anchor)
	creds := adapters.Credentials{APIKey: key, APISecret: "offline-secret", Futures: &adapters.KeyPair{APIKey: key, APISecret: "offline-futures"}}
	if err := adapters.CheckCredentials(d.Platform, d.Structure, adapters.AccountTypeIDs(d.Structure), creds); err != nil {
		t.Fatal(err)
	}
	a := New(d.Platform).WithClock(func() time.Time { return anchor.Add(time.Minute) })
	validated, err := a.Validate(context.Background(), creds, d.Structure)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ExternalRef != ExternalRef(DemoKey(d.ID, anchor.AddDate(0, 0, 1))) {
		t.Fatal("external reference depends on anchor")
	}
	runtime, err := a.Collect(context.Background(), creds, d.Structure, adapters.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := DemoBatch(d, anchor, anchor.Add(time.Minute), adapters.AccountTypeIDs(d.Structure))
	if err != nil || !reflect.DeepEqual(runtime, expected) {
		t.Fatalf("runtime diverged from seed generator: %v", err)
	}
	creds.APIKey = demoPrefix + "unknown:123"
	if _, err := a.Validate(context.Background(), creds, d.Structure); err == nil {
		t.Fatal("unknown reserved key accepted")
	}
	creds.APIKey = DemoKey("daily-bybit", anchor)
	if _, err := a.Validate(context.Background(), creds, d.Structure); err == nil {
		t.Fatal("scenario used on wrong Platform")
	}
}

func TestDemoEventsAreCoherent(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	d := demoByID(t, "daily-classic")
	amount := func(kind adapters.AccountType, symbol string, at time.Time) string {
		r := demoReport(d, anchor, at, kind)
		for _, b := range r.Balances {
			if b.PlatformAssetSymbol == symbol {
				return b.Equity
			}
		}
		return "0"
	}
	for _, event := range []struct {
		days        int
		kind        adapters.AccountType
		coin, delta string
	}{
		{-60, "spot", "BTC", "0.1"}, {-30, "funding", "USDT", "5000"}, {-5, "funding", "USDC", "-1000"},
	} {
		at := DemoEventTime(anchor, event.days)
		if change := demoSub(amount(event.kind, event.coin, at), amount(event.kind, event.coin, at.Add(-time.Second))); change != event.delta {
			t.Fatalf("external flow delta %s, want %s", change, event.delta)
		}
	}
	for _, days := range []int{-33, -10} {
		at := DemoEventTime(anchor, days)
		before, after := "0", "0"
		for _, kind := range []adapters.AccountType{"funding", "earn", adapters.TypeDualInvestment} {
			before = demoAdd(before, amount(kind, "USDT", at.Add(-time.Second)))
			after = demoAdd(after, amount(kind, "USDT", at))
		}
		if before != after {
			t.Fatal("subscription lost or double-counted principal")
		}
	}
	at := DemoEventTime(anchor, -3)
	before, after := "0", "0"
	for _, kind := range []adapters.AccountType{"spot", "funding", "earn", adapters.TypeDualInvestment} {
		before = demoAdd(before, amount(kind, "USDT", at.Add(-time.Second)))
		after = demoAdd(after, amount(kind, "USDT", at))
	}
	if before != after {
		t.Fatalf("internal transfer/redemption changed principal: %s -> %s", before, after)
	}
	historic := demoReport(d, anchor, anchor.AddDate(0, 0, -5), "um_futures")
	current := demoReport(d, anchor, anchor, "um_futures")
	if len(historic.Positions) != len(current.Positions)+1 {
		t.Fatal("successful close did not remove historical hedge")
	}
	failed := demoReport(demoByID(t, "edge-stale"), anchor, anchor, "cm_futures")
	if failed.Status != adapters.StatusFailed || len(failed.Positions) != 0 {
		t.Fatal("failure invented a Complete Set")
	}
	legacy := demoReport(demoByID(t, "edge-scope"), anchor, anchor.AddDate(0, 0, -5), "earn")
	if legacy.EarnDetailsCollected || len(legacy.EarnPositions) != 0 {
		t.Fatal("old combined scope gained invented details")
	}
}

func TestDemoTimeAndMarketDeterminism(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	d := demoByID(t, "large-classic")
	first, err := DemoBatch(d, anchor, anchor, adapters.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	again, err := DemoBatch(d, anchor, anchor, adapters.AccountTypeIDs(d.Structure))
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("same observation differs")
	}
	later, err := DemoBatch(d, anchor, anchor.Add(time.Minute), adapters.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range first.Reports {
		if r.AccountType == "earn" {
			if !r.EarnPositions[1].SubscribedAt.Equal(*later.Reports[i].EarnPositions[1].SubscribedAt) {
				t.Fatal("subscription date moved on observation")
			}
		}
		if r.AccountType == adapters.TypeDualInvestment {
			if !r.DualPositions[0].ExpiryAt.Equal(*later.Reports[i].DualPositions[0].ExpiryAt) {
				t.Fatal("expiry moved on observation")
			}
		}
	}
	future, err := DemoBatch(d, anchor, anchor.AddDate(0, 0, 65), adapters.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range future.Reports {
		//nolint:exhaustive // Manual is application-owned and outside the private Adapter report fixture.
		switch r.AccountType {
		case "spot":
			if len(r.Balances) < 60 {
				t.Fatal("Balance pagination disappeared")
			}
		case "um_futures":
			if len(r.Positions) < 60 {
				t.Fatal("Futures pagination disappeared")
			}
		case "earn":
			if len(r.EarnPositions) < 60 {
				t.Fatal("Earn pagination disappeared")
			}
		case adapters.TypeDualInvestment:
			if len(r.DualPositions) < 60 {
				t.Fatal("Dual pagination disappeared")
			}
		}
	}
	if DemoPrice(adapters.PlatformBinance, "DEMO-NOPRICE", anchor) != "" {
		t.Fatal("unpriced asset gained price")
	}
	bad := d
	bad.Mode = adapters.MarginOKXPM
	if ValidateDemoIdentity(bad) == nil {
		t.Fatal("unsupported mode accepted")
	}
}

func TestDemoDualStagesAndRollover(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	d := demoByID(t, "large-classic")
	initial := demoReport(d, anchor, anchor, adapters.TypeDualInvestment)
	later := demoReport(d, anchor, anchor.AddDate(0, 0, 2), adapters.TypeDualInvestment)
	rolled := demoReport(d, anchor, anchor.AddDate(0, 0, 4), adapters.TypeDualInvestment)
	if initial.DualPositions[0].Status != "active" || later.DualPositions[0].Status != "redeeming" {
		t.Fatal("product phase does not follow actual time")
	}
	if !initial.DualPositions[0].ExpiryAt.Equal(*later.DualPositions[0].ExpiryAt) || initial.DualPositions[0].PlatformPositionRef != later.DualPositions[0].PlatformPositionRef {
		t.Fatal("existing term changed identity/date")
	}
	if initial.DualPositions[0].PlatformPositionRef == rolled.DualPositions[0].PlatformPositionRef || rolled.DualPositions[0].Status != "pending" {
		t.Fatal("settled subscription was not replaced by new subscription")
	}
	if !reflect.DeepEqual(initial.Balances, rolled.Balances) {
		t.Fatal("settlement and reinvestment changed principal")
	}
}
