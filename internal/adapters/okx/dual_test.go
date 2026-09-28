package okx

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestDualSelectionAndDocumentedTerms(t *testing.T) {
	t.Parallel()
	for _, types := range [][]adapters.AccountType{{TypeFunding}, {TypeEarn}, {TypeDualInvestment}, {TypeEarn, TypeDualInvestment}} {
		r, a := newReplay(t, "dual_investment")
		batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, types)
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
		if r.count(epDCD) != expected || r.count(epValuation) != expected {
			t.Fatal("shared collection must run once, only when selected")
		}
		for _, r := range batch.Reports {
			if r.Status != adapters.StatusOK {
				t.Fatalf("report: %+v", r)
			}
			if r.AccountType == TypeDualInvestment {
				if len(r.DualPositions) != 3 {
					t.Fatalf("positions: %+v", r)
				}
				for _, p := range r.DualPositions {
					if len(p.MissingFields()) != 0 || *p.RateKind != "annualized_yield" {
						t.Fatalf("missing documented terms: %+v", p)
					}
					if p.InvestSymbol == "ETH" && (*p.BaseSymbol != "ETH" || *p.TargetPrice != "5000") {
						t.Fatal("synthetic ETH terms contradict the product")
					}
				}
			}
			if r.AccountType == TypeEarn {
				for _, b := range r.Balances {
					if b.Symbol == "BTC" || b.Symbol == "ETH" {
						t.Fatal("dual principal in Earn")
					}
				}
			}
		}
	}
}

func TestDualOnlyFailsWhenAuxiliarySavingsFails(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t, "dual_investment")
	r.always(epSavings, ok(`[{"ccy":"USDT","amt":""}]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	if reportOf(t, batch, TypeDualInvestment).ErrorClass != adapters.ClassNormalization || reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatalf("auxiliary failure: %+v", batch)
	}
}
