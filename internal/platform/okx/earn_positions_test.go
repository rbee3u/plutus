package okx

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestEarnMultiAssetOrderAndSavingsPreference(t *testing.T) {
	r, a := newReplay(t)
	r.always(epStaking, ok(`[{"ordId":"multi","productId":"p","state":"2","term":"7","apy":"0.07","purchasedTime":"1776068177000","estSettlementTime":"1776385800000","investData":[{"ccy":"ETH","amt":"0.1"},{"ccy":"USDT","amt":"2"}],"earningData":[{"ccy":"USDT","earnings":"1","earningType":"0"},{"ccy":"COMP","earnings":"0.1","earningType":"1"}]}]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	report := reportOf(t, batch, TypeEarn)
	if report.Status != adapters.StatusOK || !report.EarnDetailsCollected || len(report.EarnPositions) != 2 {
		t.Fatalf("report: %+v", report)
	}
	p := report.EarnPositions[1]
	if len(p.Assets) != 2 || p.MaturityAt != nil || p.SubscribedAt == nil || *p.Status != "Redeeming" || p.Rewards[0].Kind != "estimated" || p.Rewards[1].AssetSymbol != "COMP" || p.Rewards[1].Kind != "cumulative" {
		t.Fatalf("order: %+v", p)
	}
	s := report.EarnPositions[0]
	if s.Rates[0].Qualifier != "minimum" || s.Rewards[0].Kind != "reported" || s.Status != nil || s.SubscribedAt != nil {
		t.Fatalf("savings: %+v", s)
	}
}

func TestEarnUnknownRewardScopeRetainsReportedAmount(t *testing.T) {
	var order stakingOrder
	order.EarningData = append(order.EarningData, struct {
		Ccy      string `json:"ccy"`
		Earnings string `json:"earnings"`
		Type     string `json:"earningType"`
	}{Ccy: "USDT", Earnings: "0.5", Type: "99"})
	p := order.position()
	if len(p.Rewards) != 1 || p.Rewards[0].Kind != "reported" || p.Rewards[0].Amount != "0.5" || len(p.Problems) != 1 {
		t.Fatalf("unknown-scope reward was lost: %+v", p)
	}
}
