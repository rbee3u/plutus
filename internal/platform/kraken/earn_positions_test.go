package kraken

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestEarnStrategyRatesStagesAndRewards(t *testing.T) {
	r := newReplay(t, "", map[string]string{
		epEarnStrategies.String():  `{"error":[],"result":{"items":[{"id":"ES1","lock_type":{"type":"bonded","unbonding_period":604800},"apr_estimate":{"low":"3.5","high":"5.5"}}],"next_cursor":null}}`,
		epEarnAllocations.String(): `{"error":[],"result":{"items":[{"strategy_id":"ES1","native_asset":"USDT","amount_allocated":{"total":{"native":"20"},"bonding":{"native":"5","allocations":[{"native":"5","created_at":"2026-10-01T00:00:00Z","expires":"2026-10-03T00:00:00Z"}]},"unbonding":{"native":"3","allocations":[]}},"total_rewarded":{"native":"1"},"payout":{"accumulated_reward":{"native":"0.2"},"estimated_reward":{"native":"0.1"},"period_end":"2026-10-08T00:00:00Z"}}],"next_cursor":null}}`,
	})
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureKrakenUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	report := reportsByType(t, &batch)[TypeEarn]
	if report.Status != adapters.StatusOK || !report.EarnDetailsCollected || len(report.EarnPositions) != 1 {
		t.Fatalf("report: %+v", report)
	}
	p := report.EarnPositions[0]
	if p.Granularity != "strategy" || p.Type != "bonded" || p.MaturityAt != nil || p.SubscribedAt != nil || *p.Status != "Bonding / Unbonding" || len(p.Rewards) != 3 || p.Rates[0].Kind != "apr" || p.Rates[0].Upper == nil {
		t.Fatalf("strategy: %+v", p)
	}
}
