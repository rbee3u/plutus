package binance

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestEarnDetailsPreserveDatesRateKindsAndRewardScopes(t *testing.T) {
	r, a := newReplay(t, "")
	r.always(epFlexibleEarn, status(http.StatusOK, `{"total":1,"rows":[{"productId":"USDT001","asset":"USDT","totalAmount":"2","latestAnnualPercentageRate":"0.03","tierAnnualPercentageRate":{"0-100":"0.02"},"cumulativeTotalRewards":"0.4","cumulativeRealTimeRewards":"0.3","cumulativeBonusRewards":"0.1"}]}`))
	r.always(epLockedEarn, status(http.StatusOK, `{"total":1,"rows":[{"positionId":1,"projectId":"ETH001","asset":"ETH","amount":"0.01","APY":"0.05","purchaseTime":1776068177000,"rewardsEndDate":1776385800000,"deliverDate":1776472200000,"rewardAsset":"BNB","rewardAmt":"0.1","status":"HOLDING"}]}`))
	report := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeEarn), TypeEarn)
	if report.Status != adapters.StatusSucceeded || len(report.EarnPositions) != 2 {
		t.Fatalf("report: %+v", report)
	}
	fixed := report.EarnPositions[1]
	if fixed.Granularity != "position" || fixed.Rates[0].Kind != "apy" || fixed.MaturityAt.UnixMilli() != 1776385800000 || fixed.SubscribedAt == nil || fixed.Rewards[0].Kind != "reported" || fixed.Rewards[0].AssetSymbol != "BNB" {
		t.Fatalf("fixed: %+v", fixed)
	}
	flexible := report.EarnPositions[0]
	if flexible.MaturityAt != nil || flexible.SubscribedAt != nil || flexible.Status != nil || flexible.Rates[0].Kind != "apr" || flexible.Rates[1].Qualifier != "tier" || flexible.Rewards[0].Kind != "cumulative" {
		t.Fatalf("flexible: %+v", flexible)
	}
	var raw earnHolding
	if err := json.Unmarshal([]byte(`{"positionId":2,"asset":"ETH","apy":"invalid","purchaseTime":"bad"}`), &raw); err != nil {
		t.Fatal(err)
	}
	p, err := raw.position("amount", "1")
	if err != nil || len(p.Rates) != 0 || len(p.Problems) < 2 || p.Assets[0].Amount != "1" {
		t.Fatalf("optional metadata: %+v %v", p, err)
	}
}
