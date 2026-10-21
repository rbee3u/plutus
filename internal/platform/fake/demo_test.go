package fake

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestDemoNormalMajorityAndConcentration(t *testing.T) {
	anchor := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	normal, exceptional, archived, empty, earn, dual := 0, 0, 0, 0, 0, 0
	normalPerpetuals, normalDeliveries := 0, 0
	deliveryKinds := map[string]bool{}
	shapes := map[string]bool{}
	total := "0"
	for _, d := range DemoIdentities() {
		if d.Profile == "archived" {
			archived++
			if len(DemoAccounts(d, anchor, anchor)) != 0 {
				t.Fatal("archived login remains in the current boundary")
			}
			continue
		}
		if d.Portfolio == "edge-cases" {
			exceptional++
			continue
		}
		normal++
		batch, err := DemoBatch(d, anchor, anchor, DemoAccounts(d, anchor, anchor))
		if err != nil {
			t.Fatal(err)
		}
		shape := ""
		for _, r := range batch.Reports {
			for _, b := range r.Balances {
				shape += fmt.Sprintf("%s/%s=%s;", r.AccountType, b.AssetSymbol, b.Equity)
			}
			if r.Status != adapter.StatusSucceeded {
				t.Fatalf("normal login %s has a current failure", d.ID)
			}
			if len(r.Balances) == 0 {
				empty++
			}
			earn += len(r.EarnPositions)
			dual += len(r.DualPositions)
			if r.Equity != nil {
				total = demoAdd(total, *r.Equity)
			} else {
				for _, b := range r.Balances {
					price := DemoPrice(d.Platform, b.AssetSymbol, anchor)
					if price == "" {
						t.Fatalf("normal asset %s has no price", b.AssetSymbol)
					}
					total = demoAdd(total, demoMul(b.Equity, price))
				}
			}
			for _, p := range r.Positions {
				if p.Side != adapter.SideShort || p.MarginSetting != "cross" {
					t.Fatal("normal hedges lost the production-shaped short cross mix")
				}
				switch p.FuturesContract.Kind {
				case adapter.KindLinearPerpetual, adapter.KindInversePerpetual:
					normalPerpetuals++
					if p.FuturesContract.ExpiryAt != nil {
						t.Fatal("normal perpetual gained a delivery expiry")
					}
				case adapter.KindLinearDelivery, adapter.KindInverseDelivery:
					normalDeliveries++
					deliveryKinds[p.FuturesContract.Kind] = true
					if p.FuturesContract.ExpiryAt == nil || !p.FuturesContract.ExpiryAt.After(anchor) {
						t.Fatal("normal delivery leg lost its future expiry")
					}
				default:
					t.Fatalf("normal hedge has an unsupported kind %s", p.FuturesContract.Kind)
				}
			}
		}
		shapes[shape] = true
	}

	if normal != 12 || exceptional != 3 || archived != 1 || len(shapes) != normal {
		t.Fatalf("Identity mix normal=%d exceptional=%d archived=%d distinct=%d", normal, exceptional, archived, len(shapes))
	}
	if empty < 20 || earn < 15 || earn > 30 || dual != 2 {
		t.Fatalf("wallet/product concentration: empty=%d earn=%d dual=%d", empty, earn, dual)
	}
	if normalPerpetuals < 20 || normalDeliveries < 2 || normalDeliveries > 8 ||
		!deliveryKinds[adapter.KindLinearDelivery] || !deliveryKinds[adapter.KindInverseDelivery] {
		t.Fatalf("normal futures mix lost its perpetual majority or delivery coverage: perpetual=%d delivery=%d kinds=%v", normalPerpetuals, normalDeliveries, deliveryKinds)
	}
	if demoNumber(total).Cmp(demoNumber("200000")) < 0 || demoNumber(total).Cmp(demoNumber("400000")) > 0 {
		t.Fatalf("normal value left the intended synthetic scale: %s", total)
	}
}

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
	covered := map[adapter.AccountStructure]map[adapter.MarginMode]bool{}
	seen := map[string]bool{}
	definitions := DemoIdentities()
	// Scope coverage belongs to explicit fixtures, independently of the seed mix.
	for _, p := range adapter.NewRegistry().Platforms() {
		for _, s := range p.Structures {
			modes := []adapter.MarginMode{""}
			if len(s.MarginModes) > 0 {
				modes = nil
				for _, m := range s.MarginModes {
					modes = append(modes, m.ID)
				}
			}
			for _, mode := range modes {
				definitions = append(definitions, DemoIdentity{ID: "test-" + string(s.ID) + "-" + string(mode), Portfolio: "default", Name: "Capability fixture", Platform: p.ID, Structure: s.ID, Mode: mode, Profile: "metadata", Connection: "enabled"})
			}
		}
	}
	for _, d := range definitions {
		if err := ValidateDemoIdentity(d); err != nil {
			t.Fatal(err)
		}
		if seen[d.ID] {
			t.Fatal("duplicate identity")
		}
		seen[d.ID] = true
		if covered[d.Structure] == nil {
			covered[d.Structure] = map[adapter.MarginMode]bool{}
		}
		covered[d.Structure][d.Mode] = true
		for _, at := range []time.Time{anchor.AddDate(0, 0, -400), anchor.AddDate(0, 0, -7), anchor, anchor.AddDate(0, 0, 65)} {
			batch, err := DemoBatch(d, anchor, at, adapter.AccountTypeIDs(d.Structure))
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range batch.Reports {
				if r.Status != adapter.StatusSucceeded {
					continue
				}
				coins := map[string]bool{}
				for _, b := range r.Balances {
					if coins[b.AssetSymbol] || !decimal.Valid(b.Equity) {
						t.Fatalf("%s duplicate/invalid balance %+v", d.ID, b)
					}
					coins[b.AssetSymbol] = true
				}
				if r.AccountType == "earn" {
					if err := adapter.CheckEarnPositions(r.EarnPositions, r.Balances); err != nil {
						t.Fatalf("%s: %v", d.ID, err)
					}
				}
				for _, p := range r.Positions {
					if !decimal.Valid(p.SourceQuantity) || decimal.ParseOrZero(p.SourceQuantity).Sign() <= 0 {
						t.Fatal("invalid position quantity")
					}
					if p.FuturesContract.Kind == adapter.KindLinearDelivery || p.FuturesContract.Kind == adapter.KindInverseDelivery {
						if p.FuturesContract.ExpiryAt == nil {
							t.Fatal("delivery without expiry")
						}
					}
				}
			}
		}
	}
	for _, p := range adapter.NewRegistry().Platforms() {
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
	creds := adapter.Credentials{APIKey: key, APISecret: "offline-secret", Futures: &adapter.KeyPair{APIKey: key, APISecret: "offline-futures"}}
	if err := adapter.CheckCredentials(d.Platform, d.Structure, adapter.AccountTypeIDs(d.Structure), creds); err != nil {
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
	runtime, err := a.Collect(context.Background(), creds, d.Structure, adapter.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := DemoBatch(d, anchor, anchor.Add(time.Minute), adapter.AccountTypeIDs(d.Structure))
	if err != nil || !reflect.DeepEqual(runtime, expected) {
		t.Fatalf("runtime diverged from seed generator: %v", err)
	}
	creds.APIKey = demoPrefix + "unknown:123"
	if _, err := a.Validate(context.Background(), creds, d.Structure); err == nil {
		t.Fatal("unknown reserved key accepted")
	}
	creds.APIKey = DemoKey("daily-bybit", anchor)
	if _, err := a.Validate(context.Background(), creds, d.Structure); err == nil {
		t.Fatal("scenario used on wrong platform")
	}
}

func TestDemoEventsAreCoherent(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	d := demoByID(t, "daily-classic")
	amount := func(kind adapter.AccountType, symbol string, at time.Time) string {
		r := demoReport(d, anchor, at, kind)
		for _, b := range r.Balances {
			if b.AssetSymbol == symbol {
				return b.Equity
			}
		}
		return "0"
	}
	for _, event := range []struct {
		days        int
		kind        adapter.AccountType
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
		for _, kind := range []adapter.AccountType{"funding", "earn", adapter.TypeDualInvestment} {
			before = demoAdd(before, amount(kind, "USDT", at.Add(-time.Second)))
			after = demoAdd(after, amount(kind, "USDT", at))
		}
		if before != after {
			t.Fatal("subscription lost or double-counted principal")
		}
	}
	at := DemoEventTime(anchor, -3)
	before, after := "0", "0"
	for _, kind := range []adapter.AccountType{"spot", "funding", "earn", adapter.TypeDualInvestment} {
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
	if failed.Status != adapter.StatusFailed || len(failed.Positions) != 0 {
		t.Fatal("failure invented a complete set")
	}
}

func TestDemoTimeAndMarketDeterminism(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	d := largeDemoIdentity()
	first, err := DemoBatch(d, anchor, anchor, adapter.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	again, err := DemoBatch(d, anchor, anchor, adapter.AccountTypeIDs(d.Structure))
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("same observation differs")
	}
	later, err := DemoBatch(d, anchor, anchor.Add(time.Minute), adapter.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range first.Reports {
		if r.AccountType == "earn" {
			if !r.EarnPositions[1].SubscribedAt.Equal(*later.Reports[i].EarnPositions[1].SubscribedAt) {
				t.Fatal("subscription date moved on observation")
			}
		}
		if r.AccountType == adapter.TypeDualInvestment {
			if !r.DualPositions[0].ExpiryAt.Equal(*later.Reports[i].DualPositions[0].ExpiryAt) {
				t.Fatal("expiry moved on observation")
			}
		}
	}
	future, err := DemoBatch(d, anchor, anchor.AddDate(0, 0, 65), adapter.AccountTypeIDs(d.Structure))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range future.Reports {
		//nolint:exhaustive // manual is application-owned and outside the private adapter report fixture.
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
		case adapter.TypeDualInvestment:
			if len(r.DualPositions) < 60 {
				t.Fatal("Dual pagination disappeared")
			}
		}
	}
	if DemoPrice(adapter.PlatformBinance, "DEMO-NOPRICE", anchor) != "" {
		t.Fatal("unpriced asset gained price")
	}
	bad := d
	bad.Mode = adapter.MarginOKXPM
	if ValidateDemoIdentity(bad) == nil {
		t.Fatal("unsupported mode accepted")
	}
}

func TestDemoDualStagesAndRollover(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	d := largeDemoIdentity()
	initial := demoReport(d, anchor, anchor, adapter.TypeDualInvestment)
	later := demoReport(d, anchor, anchor.AddDate(0, 0, 2), adapter.TypeDualInvestment)
	rolled := demoReport(d, anchor, anchor.AddDate(0, 0, 4), adapter.TypeDualInvestment)
	if initial.DualPositions[0].Status != "active" || later.DualPositions[0].Status != "redeeming" {
		t.Fatal("product phase does not follow actual time")
	}
	if !initial.DualPositions[0].ExpiryAt.Equal(*later.DualPositions[0].ExpiryAt) || initial.DualPositions[0].PositionKey != later.DualPositions[0].PositionKey {
		t.Fatal("existing term changed identity/date")
	}
	if initial.DualPositions[0].PositionKey == rolled.DualPositions[0].PositionKey || rolled.DualPositions[0].Status != "pending" {
		t.Fatal("settled subscription was not replaced by new subscription")
	}
	if !reflect.DeepEqual(initial.Balances, rolled.Balances) {
		t.Fatal("settlement and reinvestment changed principal")
	}
}

func largeDemoIdentity() DemoIdentity {
	return DemoIdentity{ID: "pagination-fixture", Portfolio: "default", Name: "Pagination fixture", Platform: adapter.PlatformBinance, Structure: adapter.StructureBinanceClassic, Profile: "large", Connection: "enabled"}
}
