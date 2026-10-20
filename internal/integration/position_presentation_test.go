package integration

import (
	"testing"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestPositionCompletenessSeparatesBalanceOmissionsAndMissingSources(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Unpriced balance", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures")
	item = h.storeCredentials(item, "unpriced-position-completeness")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	positions, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || first.Point.Completeness() != valuation.CompletenessIncomplete || positions.Completeness != valuation.CompletenessComplete || len(positions.Issues) != 0 {
		t.Fatalf("balance omission contaminated notional: %+v %v", positions, err)
	}
	missing := h.createIdentity("Missing positions", adapters.PlatformBybit, adapters.StructureBybitUTA2, "unified")
	missing = h.storeCredentials(missing, "position-missing-source")
	enabled, result, err := h.identities.Enable(h.ctx, h.portfolioID, missing.ID, missing.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	broken := h.storeCredentials(enabled, "invalid-position-missing-source")
	if _, err := h.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='enabled' WHERE id=?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	second, _ := h.runCycle()
	accountID := h.accountsByType(second.Point.CycleID, missing.ID)["unified"].AccountID
	positions, err = h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Platform: "binance", Limit: 50})
	if err != nil || positions.Completeness != valuation.CompletenessComplete || positions.TotalCompleteness != valuation.CompletenessComplete || len(positions.TotalIssues) != 0 {
		t.Fatalf("unrelated missing account qualified filtered notional: %+v %v", positions, err)
	}
	positions, err = h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || positions.Completeness != valuation.CompletenessIncomplete || positions.TotalUSD == nil || *positions.TotalUSD == "0" {
		t.Fatalf("missing position source omitted: %+v %v", positions, err)
	}
	found := false
	for _, issue := range positions.Issues {
		if issue.Code == "missing" && issue.AccountID != nil && *issue.AccountID == accountID {
			found = true
		}
	}
	if !found {
		t.Fatal("missing account without position rows was not named")
	}
}
