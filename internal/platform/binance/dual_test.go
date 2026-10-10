package binance

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestSyntheticDualInvestmentMapsBothDirections(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeDualInvestment)
	dual := reportOf(t, batch, TypeDualInvestment)
	if dual.Status != adapters.StatusOK || len(dual.DualPositions) != 2 || len(dual.Balances) != 2 {
		t.Fatalf("synthetic dual report: %+v", dual)
	}
	page := body[dualInvestmentPage](t, r, epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1))
	if page.Total != 2 || len(page.List) != 2 || r.count(epDualInvest) != 4 {
		t.Fatal("synthetic page must contain both subscriptions and require no additional page")
	}
	want := map[string]struct {
		order, invest, principal, direction, target, apr string
		subscribed                                       int64
	}{
		"1001": {"2001", "USDT", "10", "buy_low", "15", "0.08", 1700000000000},
		"1002": {"2002", "SOL", "0.1", "sell_high", "25", "0.12", 1700000001000},
	}
	seen := map[string]bool{}
	for _, position := range dual.DualPositions {
		expected, ok := want[position.PlatformPositionRef]
		if !ok || seen[position.PlatformPositionRef] {
			t.Fatalf("unexpected or duplicate synthetic position %+v", position)
		}
		seen[position.PlatformPositionRef] = true
		if deref(position.OrderRef) != expected.order || position.InvestAssetSymbol != expected.invest ||
			position.Principal != expected.principal || deref(position.BaseAssetSymbol) != "SOL" || deref(position.QuoteAssetSymbol) != "USDT" ||
			deref(position.Direction) != expected.direction || deref(position.TargetPrice) != expected.target ||
			deref(position.AnnualizedRate) != expected.apr || deref(position.RateKind) != "apr" ||
			position.ExpiryAt == nil || !position.ExpiryAt.Equal(time.UnixMilli(1700600000000).UTC()) ||
			position.SubscribedAt == nil || !position.SubscribedAt.Equal(time.UnixMilli(expected.subscribed).UTC()) ||
			position.Status != "active" || len(position.MissingFields()) != 0 {
			t.Fatalf("synthetic product terms mapped incompletely: %+v", position)
		}
	}
	if balanceOf(t, dual, "USDT").Equity != "10" || balanceOf(t, dual, "SOL").Equity != "0.1" {
		t.Fatalf("synthetic principal must remain in each invested currency: %+v", dual.Balances)
	}
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 2 ||
		balanceOf(t, earn, "USDT").Equity != "2" || balanceOf(t, earn, "ETH").Equity != "0.01" {
		t.Fatalf("Dual principal must not enter ordinary Earn: %+v", earn)
	}
}

func TestDualAggregateMismatchFailsDependentReports(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t, "")
	r.always(epWalletValue, status(http.StatusOK, `[{"walletName":"Earn","balance":"40"}]`))
	batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeDualInvestment)
	for _, kind := range []adapters.AccountType{TypeEarn, TypeDualInvestment} {
		report := reportOf(t, batch, kind)
		if report.Status != adapters.StatusFailed || report.ErrorClass != adapters.ClassUnsupported ||
			!strings.Contains(report.ErrorDetail, "40.00") || !strings.Contains(report.ErrorDetail, "34.00") {
			t.Fatalf("inconsistent sample combination must fail completeness: %+v", report)
		}
	}
}

func TestDualAccountSelectionAndTerms(t *testing.T) {
	t.Parallel()
	for _, types := range [][]adapters.AccountType{{TypeSpot}, {TypeEarn}, {TypeDualInvestment}, {TypeEarn, TypeDualInvestment}} {
		t.Run(selectionName(types), func(t *testing.T) {
			r, a := newReplay(t, dualOnlySamples)
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
					if len(report.DualPositions) != 2 {
						t.Fatalf("positions: %+v", report)
					}
					if len(report.Balances) != 2 || balanceOf(t, report, "USDT").Equity != "10" || balanceOf(t, report, "SOL").Equity != "0.1" {
						t.Fatalf("synthetic principal mapping: %+v", report)
					}
					for _, p := range report.DualPositions {
						if len(p.MissingFields()) != 0 || deref(p.BaseAssetSymbol) != "SOL" || deref(p.QuoteAssetSymbol) != "USDT" {
							t.Fatalf("synthetic product terms: %+v", p)
						}
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
	r, a := newReplay(t, dualOnlySamples)
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
			r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"22.5","walletName":"Earn"}]`))
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
