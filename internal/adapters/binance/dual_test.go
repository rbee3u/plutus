package binance

import (
	"net/http"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestRecordedDualInvestmentMapsSubscription(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeDualInvestment)
	dual := reportOf(t, batch, TypeDualInvestment)
	if dual.Status != adapters.StatusOK || len(dual.DualPositions) != 1 || len(dual.Balances) != 1 {
		t.Fatalf("recorded dual report: %+v", dual)
	}
	page := body[dualInvestmentPage](t, r, epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1))
	if page.Total != 1 || len(page.List) != 1 || page.List[0].PurchaseStatus != "PURCHASE_SUCCESS" {
		t.Fatal("the recorded fixture must contain one running subscription")
	}
	row, position := page.List[0], dual.DualPositions[0]
	wantExpiry := time.Date(2026, time.October, 2, 8, 0, 0, 0, time.UTC)
	wantSubscribed := time.UnixMilli(1790678854360).UTC()
	if row.OptionType != "CALL" || position.PlatformPositionRef != dualID(row.ID) ||
		position.OrderRef == nil || *position.OrderRef != dualID(row.OrderID) ||
		position.InvestSymbol != row.InvestCoin || rat(t, position.Principal).Cmp(rat(t, row.SubscriptionAmount)) != 0 ||
		position.BaseSymbol == nil || *position.BaseSymbol != row.InvestCoin ||
		position.QuoteSymbol == nil || *position.QuoteSymbol != row.ExercisedCoin ||
		position.Direction == nil || *position.Direction != "sell_high" ||
		position.TargetPrice == nil || rat(t, *position.TargetPrice).Cmp(rat(t, row.StrikePrice)) != 0 ||
		position.AnnualizedRate == nil || rat(t, *position.AnnualizedRate).Cmp(rat(t, row.APR)) != 0 ||
		position.RateKind == nil || *position.RateKind != "apr" ||
		position.ExpiryAt == nil || !position.ExpiryAt.Equal(wantExpiry) ||
		position.SubscribedAt == nil || !position.SubscribedAt.Equal(wantSubscribed) ||
		position.Status != "active" || len(position.MissingFields()) != 0 {
		t.Fatalf("recorded subscription mapped incompletely: %+v", position)
	}
	if dual.Balances[0].Symbol != row.InvestCoin || rat(t, dual.Balances[0].Equity).Cmp(rat(t, row.SubscriptionAmount)) != 0 {
		t.Fatalf("recorded principal balance: %+v", dual.Balances)
	}
	if reportOf(t, batch, TypeEarn).Status != adapters.StatusOK {
		t.Fatal("ordinary Earn must remain successful beside the recorded Dual subscription")
	}
}

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
