package fake

import (
	"fmt"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func demoEarn(r *adapters.AccountReport, d DemoIdentity, anchor, at time.Time) {
	r.EarnDetailsCollected = true
	count := 4
	if d.Profile == "large" {
		count = 64
	}
	for i := 0; i < count; i++ {
		kind, grain, basis := "flexible", "product", "total"
		symbol, amount := "USDT", "1000"
		switch d.Platform {
		case adapters.PlatformBinance:
			if i%2 == 1 {
				kind, grain, basis, symbol, amount = "fixed", "position", "principal", "ETH", "0.15"
			}
		case adapters.PlatformOKX, adapters.PlatformBybit:
			if i%2 == 1 {
				kind, grain, basis, symbol, amount = "on_chain", "position", "principal", "ETH", "0.15"
			}
		case adapters.PlatformKraken:
			kind, grain, basis, symbol, amount = []string{"flexible", "bonded", "timed", "bonded"}[i%4], "strategy", "allocated", "DOT", "20"
		}
		if d.Profile == "large" {
			amount = "25"
			if symbol != "USDT" {
				amount = "0.01"
			}
		}
		cohort := demoCohort(anchor, at, i%4)
		ref := fmt.Sprintf("earn-%02d-%d", i, cohort.Unix())
		p := adapters.NewEarnPosition(ref, kind, grain, basis)
		p.ProductRef, p.ProductName = adapters.DualText(fmt.Sprintf("%s-%s-%02d", symbol, kind, i)), adapters.DualText("Offline "+symbol+" "+kind)
		p.Assets = []adapters.EarnAmount{{Symbol: symbol, Amount: amount}}
		p.AddRate("0.035", "apr", "estimated", "Estimated APR", nil)
		p.AddReward(symbol, demoMul(amount, "0.002"), "cumulative")
		p.Status = adapters.DualText("Active")
		if grain == "position" {
			p.SubscribedAt = &cohort
		} else {
			p.PlatformPositionRef = fmt.Sprintf("earn-%s-%02d", grain, i)
		}
		if kind == "fixed" || kind == "timed" && d.Platform != adapters.PlatformKraken {
			maturity := cohort.AddDate(0, 0, 28)
			p.MaturityAt = &maturity
			delivery := maturity.AddDate(0, 0, 1)
			p.Details = append(p.Details, adapters.EarnDetail{Label: "Principal delivery", At: &delivery})
		}
		if kind == "on_chain" || kind == "bonded" {
			p.AddDetail("Redemption rule", "Reported seven-day unbonding; maturity is unavailable.")
			if i%4 == 3 && !at.Before(cohort.AddDate(0, 0, 21)) {
				p.Status = adapters.DualText("Redeeming")
				settlement := cohort.AddDate(0, 0, 28)
				p.Details = append(p.Details, adapters.EarnDetail{Label: "Redemption settlement", At: &settlement})
			}
		}
		if i%4 == 0 {
			p.Rates = []adapters.EarnRate{{Value: "0.015", Upper: adapters.DualText("0.045"), Kind: "apy", Qualifier: "estimated", Label: "Estimated APY range"}}
			p.AddReward(symbol, demoMul(amount, "0.0001"), "pending")
		} else if i%4 == 2 {
			p.Rates = nil
			p.AddRate("0.025", "apr", "reported", "Base APR", nil)
			p.AddRate("0.05", "apr", "tier", "First tier APR", nil)
			p.AddRate("0.01", "apr", "additional", "Additional reward APR", adapters.DualText("USDT"))
			p.AddReward("USDT", "0.12", "estimated")
			p.AddDetail("Configured minimum lending rate", "0.02; owner preference, not a reported return")
		}
		if d.Platform == adapters.PlatformKraken && i%4 == 3 {
			p.Assets = append(p.Assets, adapters.EarnAmount{Symbol: "ETH", Amount: "0.05"})
			p.AddReward("ETH", "0.0002", "pending")
		}
		if d.Profile == "metadata" && i == 0 {
			p.Rates, p.SubscribedAt = nil, nil
			p.Problems = append(p.Problems, "Optional public product rate is unavailable.")
		}
		r.EarnPositions = append(r.EarnPositions, p)
	}
	// A named historical investment is removed only by a successful report.
	if d.Portfolio != "edge-cases" && !at.Before(DemoEventTime(anchor, -33)) && at.Before(DemoEventTime(anchor, -3)) {
		p := adapters.NewEarnPosition("historical-earn-redemption", "fixed", "position", "principal")
		p.ProductName = adapters.DualText("Offline completed term")
		p.Assets = []adapters.EarnAmount{{Symbol: "USDT", Amount: "400"}}
		subscribed, maturity := DemoEventTime(anchor, -33), DemoEventTime(anchor, -3)
		p.SubscribedAt, p.MaturityAt, p.Status = &subscribed, &maturity, adapters.DualText("Active")
		if d.Platform != adapters.PlatformBinance {
			p.Type, p.MaturityAt = "on_chain", nil
			if d.Platform == adapters.PlatformKraken {
				p.Type, p.Granularity, p.AmountBasis, p.SubscribedAt = "timed", "strategy", "allocated", nil
			}
			p.Details = append(p.Details, adapters.EarnDetail{Label: "Redemption settlement", At: &maturity})
			if !at.Before(DemoEventTime(anchor, -5)) {
				p.Status = adapters.DualText("Redeeming")
			}
		}
		r.EarnPositions = append(r.EarnPositions, p)
	}
	demoAmounts(r)
	if d.Profile == "scope" && DemoScope(d, anchor, at, "earn") == 1 {
		dual := adapters.AccountReport{}
		demoDual(&dual, d, anchor, at)
		// The old Account really held both groups. Preserve that source quantity.
		for _, b := range dual.Balances {
			found := false
			for i := range r.Balances {
				if r.Balances[i].Symbol == b.Symbol {
					r.Balances[i].Equity = demoAdd(r.Balances[i].Equity, b.Equity)
					found = true
					break
				}
			}
			if !found {
				r.Balances = append(r.Balances, b)
			}
		}
		r.EarnDetailsCollected, r.EarnPositions = false, nil
	}
	if d.Profile == "legacy" {
		r.EarnDetailsCollected, r.EarnPositions = false, nil
	}
}

func demoDual(r *adapters.AccountReport, d DemoIdentity, anchor, at time.Time) {
	count := 5
	if d.Profile == "large" {
		count = 64
	}
	for i := 0; i < count; i++ {
		cohort := demoCohort(anchor, at, []int{17, 21, 18, 19, 20}[i%5])
		expiry := cohort.AddDate(0, 0, 25)
		invest, amount, direction, target := "USDT", "200", "buy_low", "60000"
		if i%2 == 1 {
			invest, amount, direction, target = "BTC", "0.003", "sell_high", "72000"
		}
		// Principal remains held during the reported payout stages, then a
		// successful 28-day rollover settles and reinvests it. These strikes
		// lie outside the bounded offline price wave, so currency is preserved.
		var status string
		switch {
		case at.Before(cohort.AddDate(0, 0, 1)):
			status = "pending"
		case at.Before(expiry):
			status = "active"
		case at.Before(expiry.AddDate(0, 0, 1)):
			status = "settling"
		case at.Before(expiry.AddDate(0, 0, 2)):
			status = "redeeming"
		default:
			status = "refunding"
		}
		ref := fmt.Sprintf("dual-%02d-%d", i, cohort.Unix())
		p := adapters.DualPosition{PlatformPositionRef: ref, ProductRef: adapters.DualText("BTC-USDT-" + target), OrderRef: adapters.DualText("offline-" + ref), BaseSymbol: adapters.DualText("BTC"), QuoteSymbol: adapters.DualText("USDT"), InvestSymbol: invest, Principal: amount, Direction: adapters.DualText(direction), TargetPrice: adapters.DualText(target), SubscribedAt: &cohort, ExpiryAt: &expiry, AnnualizedRate: adapters.DualText("0.18"), RateKind: adapters.DualText("apr"), Status: status}
		if i%3 == 0 {
			p.RateKind, p.AnnualizedRate = adapters.DualText("apy"), adapters.DualText("0.12")
		}
		if d.Profile == "metadata" && i == 0 {
			p.TargetPrice, p.ExpiryAt, p.AnnualizedRate, p.RateKind = nil, nil, nil, nil
		}
		r.DualPositions = append(r.DualPositions, p)
	}
	if d.Portfolio != "edge-cases" && !at.Before(DemoEventTime(anchor, -10)) && at.Before(DemoEventTime(anchor, -3)) {
		subscribed, expiry := DemoEventTime(anchor, -10), DemoEventTime(anchor, -3)
		r.DualPositions = append(r.DualPositions, adapters.DualPosition{PlatformPositionRef: "historical-dual-settlement", BaseSymbol: adapters.DualText("BTC"), QuoteSymbol: adapters.DualText("USDT"), InvestSymbol: "USDT", Principal: "200", Direction: adapters.DualText("buy_low"), TargetPrice: adapters.DualText("10000"), SubscribedAt: &subscribed, ExpiryAt: &expiry, AnnualizedRate: adapters.DualText("0.02"), RateKind: adapters.DualText("apr"), Status: "active"})
	}
	demoAmounts(r)
}

// DemoValidate checks the simulated credential outcome independently of later
// Account-level collection failures.
func demoValidate(d DemoIdentity, anchor, at time.Time) error {
	if d.Profile == "invalid" && !at.Before(DemoEventTime(anchor, -2)) {
		return adapters.NewError(adapters.ClassAuthentication, "The Platform rejected the revoked read-only key.")
	}
	return nil
}
