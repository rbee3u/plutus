package bybit

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestDualSelectionAndDocumentedTerms(t *testing.T) {
	for _, types := range [][]adapters.AccountType{{TypeFunding}, {TypeEarn}, {TypeDualInvestment}, {TypeEarn, TypeDualInvestment}} {
		r := newReplay(t, "dual_investment", nil)
		batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, types)
		if err != nil {
			t.Fatal(err)
		}
		if len(batch.Reports) != len(types) {
			t.Fatal("auxiliary account escaped")
		}
		expected := 0
		for _, kind := range types {
			if kind == TypeEarn || kind == TypeDualInvestment {
				expected = 1
			}
		}
		if r.count(epDualAssets.path) != expected || r.count(epAssetOverview.path) != expected {
			t.Fatal("shared collection must run once, only when selected")
		}
		for _, r := range batch.Reports {
			if r.Status != adapters.StatusOK {
				t.Fatalf("report: %+v", r)
			}
			if r.AccountType == TypeDualInvestment {
				if len(r.DualPositions) != 1 {
					t.Fatalf("positions: %+v", r)
				}
				p := r.DualPositions[0]
				if p.PlatformPositionRef != "19035" || *p.ProductRef != "36320" || *p.BaseSymbol != "ETH" || *p.Direction != "buy_low" || *p.TargetPrice != "2325" || *p.AnnualizedRate != "9.027" || *p.RateKind != "apy" {
					t.Fatalf("documented terms: %+v", p)
				}
			}
			if r.AccountType == TypeEarn && (len(r.Balances) != 1 || r.Balances[0].Equity != "20") {
				t.Fatal("dual principal in Earn")
			}
		}
	}
}

func TestDualOnlyDependsOnAuxiliarySavings(t *testing.T) {
	r := newReplay(t, "dual_investment", map[string]string{epFlexible.path: `{"retCode":0,"result":{"list":[],"nextPageCursor":"unread"}}`})
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeFunding, TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	if reportOf(t, &batch, TypeDualInvestment).ErrorClass != adapters.ClassNormalization || reportOf(t, &batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatalf("auxiliary failure: %+v", batch)
	}
}
