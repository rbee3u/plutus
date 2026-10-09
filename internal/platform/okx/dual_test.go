package okx

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestDocumentedDualPositions(t *testing.T) {
	t.Parallel()
	r, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn, TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	dual := reportOf(t, batch, TypeDualInvestment)
	if dual.Status != adapters.StatusOK || len(dual.DualPositions) != 2 || len(dual.Balances) != 2 {
		t.Fatalf("synthetic dual report: %+v", dual)
	}
	if reportOf(t, batch, TypeEarn).Status != adapters.StatusOK || r.count(epDCD) != 1 {
		t.Fatal("the synthetic Earn and Dual Accounts must share a successful order history read")
	}
	orders := rowsIn[dcdOrder](t, r.file(epDCD, ""))
	if len(orders) != 3 || orders[0].State != "LIVE" || orders[1].State != "LIVE" || orders[2].State != "SETTLED" {
		t.Fatal("the synthetic fixture must retain two live orders and one settled order")
	}
	wantExpiry := time.UnixMilli(1700600000000).UTC()
	for i, p := range dual.DualPositions {
		order := orders[i]
		if order.ExpTime != "" || order.SettleTime == "" || p.PlatformPositionRef != order.OrdID ||
			p.InvestSymbol != order.NotionalCcy || rat(t, p.Principal).Cmp(rat(t, order.NotionalSz)) != 0 ||
			p.ExpiryAt == nil || !p.ExpiryAt.Equal(wantExpiry) ||
			p.TargetPrice == nil || rat(t, *p.TargetPrice).Cmp(rat(t, order.Strike)) != 0 ||
			p.AnnualizedRate == nil || rat(t, *p.AnnualizedRate).Cmp(rat(t, order.AnnualizedYield)) != 0 ||
			p.Status != "active" || len(p.MissingFields()) != 0 {
			t.Fatalf("synthetic order %d mapped incompletely: %+v", i, p)
		}
		wantDirection := "sell_high"
		if order.NotionalCcy == order.QuoteCcy {
			wantDirection = "buy_low"
		}
		if p.Direction == nil || *p.Direction != wantDirection {
			t.Fatalf("synthetic order %d direction: %+v", i, p)
		}
		ms, _ := strconv.ParseInt(order.CTime, 10, 64)
		wantSubscribed := time.UnixMilli(ms).UTC()
		if p.SubscribedAt == nil || !p.SubscribedAt.Equal(wantSubscribed) {
			t.Fatalf("synthetic order %d subscribedAt: %+v", i, p)
		}
	}
}

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
						t.Fatalf("missing synthetic terms: %+v", p)
					}
					if p.InvestSymbol == "ETH" && (*p.BaseSymbol != "ETH" || *p.TargetPrice != "5000") {
						t.Fatal("synthetic ETH terms contradict the product")
					}
				}
			}
			if r.AccountType == TypeEarn {
				for _, b := range r.Balances {
					if b.PlatformAssetSymbol == "BTC" || (b.PlatformAssetSymbol == "ETH" && b.Equity != "0.1") {
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
