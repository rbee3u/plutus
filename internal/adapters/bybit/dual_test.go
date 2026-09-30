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

func TestDocumentedDualPosition(t *testing.T) {
	var documented struct {
		List []dualAssetPosition `json:"list"`
	}
	sampleBody(t, "v5_earn_advance_position.sell_high", &documented)
	if len(documented.List) != 1 || documented.List[0].Status != "Active" || documented.List[0].Direction != "SellHigh" {
		t.Fatal("the documented Dual Asset answer must contain one active SellHigh subscription")
	}

	r := newReplay(t, "", nil)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeEarn, TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeEarn)
	dual := reportOf(t, &batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK || dual.Status != adapters.StatusOK || len(dual.Balances) != 1 || len(dual.DualPositions) != 1 {
		t.Fatalf("documented Earn and Dual reports: %+v", batch.Reports)
	}
	row := documented.List[0]
	p := dual.DualPositions[0]
	if p.PlatformPositionRef != row.PositionID || p.ProductRef == nil || *p.ProductRef != row.ProductID || p.OrderRef == nil || *p.OrderRef != row.OrderID ||
		p.BaseSymbol == nil || *p.BaseSymbol != row.BaseCoin || p.QuoteSymbol == nil || *p.QuoteSymbol != row.QuoteCoin ||
		p.InvestSymbol != row.InvestCoin || p.Principal != row.Amount || p.Direction == nil || *p.Direction != "sell_high" ||
		p.TargetPrice == nil || *p.TargetPrice != row.TargetPrice || p.ExpiryAt == nil || p.ExpiryAt.UnixMilli() != 1700600000000 ||
		p.SubscribedAt == nil || p.SubscribedAt.UnixMilli() != 1700000000000 ||
		p.AnnualizedRate == nil || *p.AnnualizedRate != "0.1" || p.RateKind == nil || *p.RateKind != "apy" || p.Status != "active" {
		t.Fatalf("documented Dual Asset terms: %+v", p)
	}
	if dual.Balances[0].Symbol != row.InvestCoin || dual.Balances[0].Equity != row.Amount {
		t.Fatalf("documented Dual Asset principal: %+v", dual.Balances)
	}
	if len(earn.Balances) != 1 || earn.Balances[0].Symbol != "USDT" || earn.Balances[0].Equity != "20" {
		t.Fatalf("ordinary Earn must exclude Dual Asset principal: %+v", earn.Balances)
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
