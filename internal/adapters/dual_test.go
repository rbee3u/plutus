package adapters

import (
	"testing"
	"time"
)

func TestDualTermsPreservePrincipalAndRejectContradictions(t *testing.T) {
	t.Parallel()
	minimal := DualPosition{PlatformPositionRef: "subscription-1", InvestSymbol: "USDT", Principal: "500", Status: "active"}
	p, err := NormalizeDualPosition("test", &minimal)
	if err != nil || p.Principal != "500" || len(p.MissingFields()) != 6 {
		t.Fatalf("missing terms must retain principal: %+v %v", p, err)
	}
	for name, mutate := range map[string]func(*DualPosition){
		"no id":              func(p *DualPosition) { p.PlatformPositionRef = "" },
		"no currency":        func(p *DualPosition) { p.InvestSymbol = "" },
		"no principal":       func(p *DualPosition) { p.Principal = "" },
		"negative principal": func(p *DualPosition) { p.Principal = "-1" },
		"zero principal":     func(p *DualPosition) { p.Principal = "0" },
		"contradictory direction": func(p *DualPosition) {
			p.BaseSymbol = DualText("BTC")
			p.QuoteSymbol = DualText("USDT")
			p.Direction = DualText("sell_high")
		},
		"unrelated currency":   func(p *DualPosition) { p.BaseSymbol = DualText("ETH"); p.QuoteSymbol = DualText("BTC") },
		"identical currencies": func(p *DualPosition) { p.BaseSymbol = DualText("USDT"); p.QuoteSymbol = DualText("USDT") },
		"invalid target":       func(p *DualPosition) { p.TargetPrice = DualText("n/a") },
		"negative rate":        func(p *DualPosition) { p.AnnualizedRate = DualText("-0.1"); p.RateKind = DualText("apr") },
		"unknown state":        func(p *DualPosition) { p.Status = "unknown" },
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
	p.BaseSymbol = DualText("btc")
	p.QuoteSymbol = DualText("usdt")
	p.TargetPrice = DualText("62000")
	p.ExpiryAt = &expiry
	p.AnnualizedRate = DualText("0.18")
	p.RateKind = DualText("apr")
	p, err = NormalizeDualPosition("test", &p)
	if err != nil || *p.Direction != "buy_low" || len(p.MissingFields()) != 0 {
		t.Fatalf("complete terms: %+v %v", p, err)
	}
}

func TestDualScopeIsLimitedToTheThreePlatforms(t *testing.T) {
	t.Parallel()
	for _, platform := range []Platform{PlatformBinance, PlatformOKX, PlatformBybit, PlatformKraken} {
		for _, kind := range []AccountType{"earn", TypeDualInvestment, "funding", "trading", "main"} {
			want := uint16(1)
			if platform != PlatformKraken && (kind == "earn" || kind == TypeDualInvestment) {
				want = 2
			}
			if got := ReportScope(platform, kind); got != want {
				t.Fatalf("%s/%s scope %d, want %d", platform, kind, got, want)
			}
		}
	}
}
