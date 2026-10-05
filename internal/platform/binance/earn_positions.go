package binance

import (
	"encoding/json"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

type earnHolding struct {
	Asset              string            `json:"asset"`
	TotalAmount        string            `json:"totalAmount"`
	Amount             string            `json:"amount"`
	ProductID          string            `json:"productId"`
	ProjectID          string            `json:"projectId"`
	PositionID         json.RawMessage   `json:"positionId"`
	APR                string            `json:"latestAnnualPercentageRate"`
	APY                string            `json:"apy"`
	Tiers              map[string]string `json:"tierAnnualPercentageRate"`
	CumulativeTotal    string            `json:"cumulativeTotalRewards"`
	CumulativeRealTime string            `json:"cumulativeRealTimeRewards"`
	CumulativeBonus    string            `json:"cumulativeBonusRewards"`
	Yesterday          string            `json:"yesterdayRealTimeRewards"`
	RewardAsset        string            `json:"rewardAsset"`
	RewardAmount       string            `json:"rewardAmt"`
	ExtraAsset         string            `json:"extraRewardAsset"`
	ExtraAPR           string            `json:"extraRewardAPR"`
	EstimatedExtra     string            `json:"estExtraRewardAmt"`
	BoostAsset         string            `json:"boostRewardAsset"`
	BoostAPR           string            `json:"boostApr"`
	TotalBoost         string            `json:"totalBoostRewardAmt"`
	PurchaseTime       json.RawMessage   `json:"purchaseTime"`
	RewardsEnd         json.RawMessage   `json:"rewardsEndDate"`
	Delivery           json.RawMessage   `json:"deliverDate"`
	NextPay            string            `json:"nextPay"`
	NextPayDate        json.RawMessage   `json:"nextPayDate"`
	Duration           json.RawMessage   `json:"duration"`
	Status             string            `json:"status"`
	CanRedeem          *bool             `json:"canRedeem"`
	AutoSubscribe      *bool             `json:"autoSubscribe"`
}

func (r *earnHolding) position(field, amount string) (adapters.EarnPosition, error) {
	ref := r.ProductID
	if ref == "" {
		ref = strings.ToUpper(strings.TrimSpace(r.Asset))
	}
	p := adapters.NewEarnPosition("flexible:"+ref, "flexible", "product", "total")
	p.ProductRef = adapters.DualText(r.ProductID)
	p.ProductName = adapters.DualText("Simple Earn Flexible")
	if field == "amount" {
		ref = dualID(r.PositionID)
		if ref == "" {
			return p, adapters.Malformed("Simple Earn Locked", "position identity")
		}
		p = adapters.NewEarnPosition("locked:"+ref, "fixed", "position", "principal")
		p.ProductRef = adapters.DualText(r.ProjectID)
		p.ProductName = adapters.DualText("Simple Earn Locked")
		p.Status = adapters.DualText(r.Status)
		p.SubscribedAt = p.RequiredMillis("Subscription time", dualID(r.PurchaseTime))
		p.MaturityAt = p.RequiredMillis("Term end", dualID(r.RewardsEnd))
		p.AddMillisDetail("Principal delivery", dualID(r.Delivery))
		p.AddMillisDetail("Next reward payment", dualID(r.NextPayDate))
		p.AddDetail("Duration (days)", dualID(r.Duration))
		p.AddRate(r.APY, "apy", "reported", "Platform-reported APY", nil)
		p.AddDetail("Rate convention", "The API labels this APY; Binance's product help describes APR. No compounding conversion is applied.")
		p.AddReward(r.RewardAsset, r.RewardAmount, "reported")
		p.AddReward(r.RewardAsset, r.NextPay, "next_payment")
		p.AddRate(r.ExtraAPR, "apr", "additional", "Extra reward APR", adapters.DualText(r.ExtraAsset))
		p.AddReward(r.ExtraAsset, r.EstimatedExtra, "estimated")
		p.AddRate(r.BoostAPR, "apr", "additional", "Boost APR", adapters.DualText(r.BoostAsset))
		p.AddReward(r.BoostAsset, r.TotalBoost, "cumulative_boost")
	} else {
		p.AddRate(r.APR, "apr", "reported", "Current APR", nil)
		for _, tier := range adapters.SortedKeys(r.Tiers) {
			p.AddRate(r.Tiers[tier], "apr", "tier", "Bonus tier "+tier, nil)
		}
		p.AddReward(r.Asset, r.CumulativeTotal, "cumulative")
		p.AddReward(r.Asset, r.CumulativeRealTime, "cumulative_realtime")
		p.AddReward(r.Asset, r.CumulativeBonus, "cumulative_bonus")
		p.AddReward(r.Asset, r.Yesterday, "yesterday")
	}
	if r.CanRedeem != nil {
		if *r.CanRedeem {
			p.AddDetail("Redeemable", "Yes")
		} else {
			p.AddDetail("Redeemable", "No")
		}
	}
	if r.AutoSubscribe != nil {
		if *r.AutoSubscribe {
			p.AddDetail("Auto subscribe", "Enabled")
		} else {
			p.AddDetail("Auto subscribe", "Disabled")
		}
	}
	p.Assets = append(p.Assets, adapters.EarnAmount{Symbol: r.Asset, Amount: amount})
	err := adapters.NormalizeEarnPosition(&p)
	return p, err
}
