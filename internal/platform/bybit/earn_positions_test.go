package bybit

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestEarnCatalogueMetadataIsOptionalAndSelected(t *testing.T) {
	for _, scenario := range []string{"matched", "failed", "mismatch", "invalid", "invalid_yield"} {
		t.Run(scenario, func(t *testing.T) {
			answers := map[string]string{}
			if scenario == "failed" {
				answers[epEarnProduct.path] = `{"retCode":10006,"result":{}}`
			}
			if scenario == "mismatch" {
				answers[epEarnProduct.path] = `{"retCode":0,"result":{"list":[]}}`
			}
			if scenario == "invalid" {
				answers[epEarnProduct.path] = `{"retCode":0,"result":{"list":[{"category":"FlexibleSaving","productId":"flex-USDT","coin":"USDT","estimateApr":"bad"}]}}`
			}
			if scenario == "invalid_yield" {
				answers[epFlexible.path] = `{"retCode":0,"result":{"list":[{"category":"FlexibleSaving","productId":"flex-USDT","coin":"USDT","amount":"20","claimableYield":"bad"}]}}`
			}
			r := newReplay(t, "", answers)
			batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeEarn, TypeDualInvestment})
			if err != nil {
				t.Fatal(err)
			}
			earn := reportOf(t, &batch, TypeEarn)
			if earn.Status != adapters.StatusSucceeded || r.count(epEarnProduct.path) != 1 || earn.Balances[0].Equity != "20" || reportOf(t, &batch, TypeDualInvestment).Status != adapters.StatusSucceeded {
				t.Fatalf("optional failure changed funds: %+v", earn)
			}
			p := earn.EarnPositions[0]
			if scenario == "invalid_yield" && (len(p.Rewards) != 0 || len(p.Problems) != 1) {
				t.Fatalf("invalid pending yield must be an auxiliary issue: %+v", p)
			}
			if p.Status != nil {
				t.Fatal("catalogue availability became position status")
			}
			if scenario == "matched" || scenario == "invalid_yield" {
				if len(p.Rates) != 1 || p.Rates[0].Value != "0.0425" || p.Rates[0].Kind != "apr" || p.Rates[0].Qualifier != "estimated" {
					t.Fatalf("rate: %+v", p)
				}
			} else if len(p.Problems) == 0 || len(p.Rates) != 0 {
				t.Fatalf("metadata issue: %+v", p)
			}
		})
	}
	r := newReplay(t, "", nil)
	if _, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeDualInvestment}); err != nil {
		t.Fatal(err)
	}
	if r.count(epEarnProduct.path) != 0 {
		t.Fatal("catalogue read for unselected Earn")
	}
}

func TestEarnFixedPositionsKeepAPYAndEarlyRedemption(t *testing.T) {
	r := newReplay(t, "fixed_saving", nil)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeEarn)
	if earn.Status != adapters.StatusSucceeded || len(earn.EarnPositions) != 3 {
		t.Fatalf("report: %+v", earn)
	}
	p := earn.EarnPositions[2]
	if p.PositionKey != "fixed:4065" || *p.Status != "EarlyRedemptionProcessing" || p.MaturityAt == nil || p.SubscribedAt == nil || p.Rates[0].Kind != "apy" || p.Rewards[0].Kind != "estimated" {
		t.Fatalf("fixed: %+v", p)
	}
}
