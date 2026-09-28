package binance

import (
	"net/http"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestDualAccountSelectionAndTerms(t *testing.T) {
	t.Parallel()
	for _, types := range [][]adapters.AccountType{{TypeSpot}, {TypeEarn}, {TypeDualInvestment}, {TypeEarn, TypeDualInvestment}} {
		t.Run(selectionName(types), func(t *testing.T) {
			r, a := newReplay(t, docDualInvestment)
			batch := collect(t, a, adapters.StructureBinancePM, types...)
			if len(batch.Reports) != len(types) {
				t.Fatal("auxiliary report escaped the adapter")
			}
			selected := false
			for _, kind := range types {
				selected = selected || kind == TypeEarn || kind == TypeDualInvestment
			}
			calls := 0
			if selected {
				calls = 4
			}
			if r.count(epDualInvest) != calls {
				t.Fatalf("dual endpoint calls %d, want %d", r.count(epDualInvest), calls)
			}
			for _, report := range batch.Reports {
				if report.Status != adapters.StatusOK {
					t.Fatalf("report: %+v", report)
				}
				if report.AccountType == TypeEarn && len(report.Balances) != 0 {
					t.Fatal("dual principal leaked into Earn")
				}
				if report.AccountType == TypeDualInvestment {
					if len(report.DualPositions) != 1 {
						t.Fatalf("positions: %+v", report)
					}
					p := report.DualPositions[0]
					if p.PlatformPositionRef != "10160533" || *p.OrderRef != "7973677530" || *p.BaseSymbol != "BNB" || *p.Direction != "buy_low" || *p.TargetPrice != "330" || *p.AnnualizedRate != "0.0365" || *p.RateKind != "apr" || p.ExpiryAt.UnixMilli() != 1708416000000 {
						t.Fatalf("documented product mapping: %+v", p)
					}
				}
			}
		})
	}
}

func selectionName(types []adapters.AccountType) string {
	out := ""
	for _, v := range types {
		out += string(v) + "-"
	}
	return out
}

func TestDualOnlyDependsOnAuxiliarySavings(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t, docDualInvestment)
	r.always(epFlexibleEarn, status(http.StatusOK, `{"total":1,"rows":[{"asset":"USDT","totalAmount":""}]}`))
	batch := collect(t, a, adapters.StructureBinancePM, TypeDualInvestment, TypeSpot)
	if reportOf(t, batch, TypeDualInvestment).ErrorClass != adapters.ClassNormalization || reportOf(t, batch, TypeSpot).Status != adapters.StatusOK {
		t.Fatalf("auxiliary failure: %+v", batch)
	}
}

func TestRepeatedDualSubscriptionCannotChangePrincipal(t *testing.T) {
	t.Parallel()
	for _, amount := range []string{"0.5", "1"} {
		t.Run(amount, func(t *testing.T) {
			r, a := newReplay(t, "")
			r.alwaysAt(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1), status(http.StatusOK, `{"total":1,"list":[{"id":"1","investCoin":"USDT","subscriptionAmount":"0.5","purchaseStatus":"PURCHASE_SUCCESS"}]}`))
			r.alwaysAt(epDualInvest, dualInvestmentQuery("SETTLING", 1), status(http.StatusOK, `{"total":1,"list":[{"id":"1","investCoin":"USDT","subscriptionAmount":"`+amount+`","purchaseStatus":"SETTLING"}]}`))
			report := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeDualInvestment), TypeDualInvestment)
			if amount == "0.5" {
				if report.Status != adapters.StatusOK || len(report.DualPositions) != 1 || report.Balances[0].Equity != "0.5" {
					t.Fatalf("moving subscription counted twice: %+v", report)
				}
			} else if report.ErrorClass != adapters.ClassNormalization {
				t.Fatalf("conflicting principal accepted: %+v", report)
			}
		})
	}
}
