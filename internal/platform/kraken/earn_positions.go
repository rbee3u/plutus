package kraken

import (
	"math/big"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type earnStrategy struct {
	ID  string `json:"id"`
	APR struct {
		Low  string `json:"low"`
		High string `json:"high"`
	} `json:"apr_estimate"`
	YieldSource struct {
		Type string `json:"type"`
	} `json:"yield_source"`
	LockType struct {
		Type      string `json:"type"`
		Bonding   *int64 `json:"bonding_period"`
		Unbonding *int64 `json:"unbonding_period"`
		Payout    *int64 `json:"payout_frequency"`
		Months    *int64 `json:"duration_months"`
	} `json:"lock_type"`
}

type earnNativeAmount struct {
	Native number `json:"native"`
}

type earnBucket struct {
	Native      number `json:"native"`
	Allocations []struct {
		Native  number `json:"native"`
		Created string `json:"created_at"`
		Expires string `json:"expires"`
	} `json:"allocations"`
}

type earnAllocation struct {
	StrategyID  string `json:"strategy_id"`
	NativeAsset string `json:"native_asset"`
	Allocated   struct {
		Total     earnNativeAmount `json:"total"`
		Bonding   *earnBucket      `json:"bonding"`
		Unbonding *earnBucket      `json:"unbonding"`
		ExitQueue *earnBucket      `json:"exit_queue"`
		Pending   *earnBucket      `json:"pending"`
		Timed     *earnBucket      `json:"timed"`
	} `json:"amount_allocated"`
	Rewarded earnNativeAmount `json:"total_rewarded"`
	Payout   struct {
		Accumulated earnNativeAmount `json:"accumulated_reward"`
		Estimated   earnNativeAmount `json:"estimated_reward"`
		Start       string           `json:"period_start"`
		End         string           `json:"period_end"`
	} `json:"payout"`
}

func (r *earnAllocation) position(symbol, amount string, s earnStrategy) adapters.EarnPosition {
	kind := s.LockType.Type
	if kind == "instant" {
		kind = "flexible"
	}
	p := adapters.NewEarnPosition("strategy:"+r.StrategyID, kind, "strategy", "allocated")
	p.ProductRef = adapters.OptionalText(r.StrategyID)
	p.ProductName = adapters.OptionalText("Kraken Earn")
	p.Assets = append(p.Assets, adapters.EarnAmount{Symbol: symbol, Amount: amount})
	p.AddDetail("Yield source", s.YieldSource.Type)
	low := p.OptionalDecimal("Estimated APR lower bound", s.APR.Low)
	high := p.OptionalDecimal("Estimated APR upper bound", s.APR.High)
	if low != nil && high != nil {
		if decimal.ParseOrZero(*high).Cmp(decimal.ParseOrZero(*low)) < 0 {
			p.Problems = append(p.Problems, "Estimated APR bounds are contradictory.")
		} else {
			upper := decimal.Format(new(big.Rat).Quo(decimal.ParseOrZero(*high), decimal.ParseOrZero("100")))
			p.Rates = append(p.Rates, adapters.EarnRate{Value: decimal.Format(new(big.Rat).Quo(decimal.ParseOrZero(*low), decimal.ParseOrZero("100"))), Upper: &upper, Kind: "apr", Qualifier: "estimated", Label: "Estimated APR"})
		}
	}
	p.AddReward(symbol, string(r.Rewarded.Native), "cumulative")
	p.AddDetail("Cumulative reward scope", "Account lifetime earnings for this strategy, including earlier allocations.")
	p.AddReward(symbol, string(r.Payout.Accumulated.Native), "pending")
	p.AddReward(symbol, string(r.Payout.Estimated.Native), "estimated")
	addEarnTime(&p, "Payout period start", r.Payout.Start)
	addEarnTime(&p, "Payout period end", r.Payout.End)
	for _, d := range []struct {
		label string
		value *int64
	}{{"Bonding period (seconds)", s.LockType.Bonding}, {"Unbonding period (seconds)", s.LockType.Unbonding}, {"Payout frequency (seconds)", s.LockType.Payout}, {"Term (months)", s.LockType.Months}} {
		if d.value != nil {
			if *d.value < 0 {
				p.Problems = append(p.Problems, d.label+" is invalid.")
			} else {
				p.AddDetail(d.label, new(big.Int).SetInt64(*d.value).String())
			}
		}
	}
	var statuses []string
	for _, entry := range []struct {
		label  string
		bucket *earnBucket
	}{{"Bonding", r.Allocated.Bonding}, {"Exit queue", r.Allocated.ExitQueue}, {"Unbonding", r.Allocated.Unbonding}, {"Pending", r.Allocated.Pending}, {"Timed allocation", r.Allocated.Timed}} {
		if entry.bucket == nil {
			continue
		}
		n := p.OptionalDecimal(entry.label+" amount", string(entry.bucket.Native))
		if n == nil || decimal.ParseOrZero(*n).Sign() == 0 {
			continue
		}
		statuses = append(statuses, entry.label)
		p.AddDetail(entry.label+" amount", *n+" "+symbol)
		for _, allocation := range entry.bucket.Allocations {
			addEarnTime(&p, entry.label+" request started", allocation.Created)
			addEarnTime(&p, entry.label+" stage ends", allocation.Expires)
			if n := p.OptionalDecimal(entry.label+" allocation", string(allocation.Native)); n != nil {
				p.AddDetail(entry.label+" allocation", *n+" "+symbol)
			}
		}
	}
	// Only report stages Kraken supplied; total minus stage amounts is not a
	// documented independent lifecycle bucket and cannot establish Active.
	if len(statuses) > 0 {
		p.Status = adapters.OptionalText(strings.Join(statuses, " / "))
	}
	return p
}

func addEarnTime(p *adapters.EarnPosition, label, value string) {
	if value == "" {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		p.Problems = append(p.Problems, label+" is invalid.")
		return
	}
	p.Details = append(p.Details, adapters.EarnDetail{Label: label, At: &at})
}
