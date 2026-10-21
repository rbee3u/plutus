package adapter

import (
	"testing"
	"time"
)

func TestDualTermsPreservePrincipalAndRejectContradictions(t *testing.T) {
	t.Parallel()
	minimal := DualPosition{PositionKey: "subscription-1", PrincipalAssetSymbol: "USDT", Principal: "500", Status: "active"}
	p, err := NormalizeDualPosition("test", &minimal)
	if err != nil || p.Principal != "500" || len(p.MissingFields()) != 6 {
		t.Fatalf("missing terms must retain principal: %+v %v", p, err)
	}
	for name, mutate := range map[string]func(*DualPosition){
		"no id":              func(p *DualPosition) { p.PositionKey = "" },
		"no currency":        func(p *DualPosition) { p.PrincipalAssetSymbol = "" },
		"no principal":       func(p *DualPosition) { p.Principal = "" },
		"negative principal": func(p *DualPosition) { p.Principal = "-1" },
		"zero principal":     func(p *DualPosition) { p.Principal = "0" },
		"contradictory direction": func(p *DualPosition) {
			p.BaseAssetSymbol = OptionalText("BTC")
			p.QuoteAssetSymbol = OptionalText("USDT")
			p.Direction = OptionalText("sell_high")
		},
		"unrelated currency": func(p *DualPosition) {
			p.BaseAssetSymbol = OptionalText("ETH")
			p.QuoteAssetSymbol = OptionalText("BTC")
		},
		"identical currencies": func(p *DualPosition) {
			p.BaseAssetSymbol = OptionalText("USDT")
			p.QuoteAssetSymbol = OptionalText("USDT")
		},
		"invalid target": func(p *DualPosition) { p.TargetPrice = OptionalText("n/a") },
		"negative rate":  func(p *DualPosition) { p.AnnualizedRate = OptionalText("-0.1"); p.RateKind = OptionalText("apr") },
		"unknown state":  func(p *DualPosition) { p.Status = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			p := minimal
			mutate(&p)
			if _, err := NormalizeDualPosition("test", &p); err == nil {
				t.Fatal("contradictory or missing principal information was accepted")
			}
		})
	}
	expiry := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	p = minimal
	p.BaseAssetSymbol = OptionalText("btc")
	p.QuoteAssetSymbol = OptionalText("usdt")
	p.TargetPrice = OptionalText("62000")
	p.ExpiryAt = &expiry
	p.AnnualizedRate = OptionalText("0.18")
	p.RateKind = OptionalText("apr")
	p, err = NormalizeDualPosition("test", &p)
	if err != nil || *p.Direction != "buy_low" || len(p.MissingFields()) != 0 {
		t.Fatalf("complete terms: %+v %v", p, err)
	}
}
