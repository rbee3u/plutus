package integration

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestSharedHierarchyAcrossManagementAndHoldingsReads(t *testing.T) {
	h := newHarness(t)
	// Creation order deliberately disagrees with platform and identity display order.
	created := []identity.Identity{
		h.createIdentity("Alpha", adapters.PlatformKraken, adapters.StructureKrakenSeparate, adapters.AccountTypeIDs(adapters.StructureKrakenSeparate)...),
		h.createIdentity("Alpha", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.AccountTypeIDs(adapters.StructureBybitUTA2)...),
		h.createIdentity("Alpha", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.AccountTypeIDs(adapters.StructureOKXUnified)...),
		h.createIdentity("Zulu", adapters.PlatformBinance, adapters.StructureBinancePM, adapters.AccountTypeIDs(adapters.StructureBinancePM)...),
		h.createIdentity("alpha", adapters.PlatformBinance, adapters.StructureBinanceClassic, adapters.AccountTypeIDs(adapters.StructureBinanceClassic)...),
	}
	for _, item := range created {
		creds := adapters.Credentials{APIKey: "order-" + item.ID.String(), APISecret: "secret"}
		if item.Platform == adapters.PlatformKraken {
			creds.Futures = &adapters.KeyPair{APIKey: "futures-order", APISecret: "secret"}
		}
		stored, err := h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}, creds)
		if err != nil {
			t.Fatal(err)
		}
		if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, stored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
			t.Fatalf("enable %s: %+v %v", item.Platform, result, err)
		}
	}
	wantIdentities := []uuid.UUID{created[4].ID, created[3].ID, created[2].ID, created[1].ID, created[0].ID}
	items, err := h.identities.List(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	var wantAccounts []uuid.UUID
	for i, item := range items {
		if item.ID != wantIdentities[i] {
			t.Fatalf("Identity %d = %s, want %s", i, item.ID, wantIdentities[i])
		}
		for _, account := range item.Accounts {
			wantAccounts = append(wantAccounts, account.ID)
		}
	}
	collectable, err := h.identities.ListCollectable(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range collectable {
		if collectable[i].Identity.ID != wantIdentities[i] {
			t.Fatal("collection order differs from management order")
		}
	}
	boundary, err := h.identities.ListBoundary(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range boundary {
		if boundary[i].ID != wantAccounts[i] {
			t.Fatal("Boundary order differs from management order")
		}
	}
	_, cycleID := h.runCycle()
	accounts, err := h.valuation.Accounts(h.ctx, h.portfolioID, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	ranks := map[uuid.UUID]int{}
	for i := range accounts {
		if accounts[i].AccountID != wantAccounts[i] {
			t.Fatalf("report account %d differs from management", i)
		}
		ranks[accounts[i].AccountID] = i
	}
	checkHierarchy := func(name string, ids []uuid.UUID) {
		t.Helper()
		if !slices.IsSortedFunc(ids, func(a, b uuid.UUID) int { return ranks[a] - ranks[b] }) {
			t.Fatalf("%s did not preserve platform / identity / account: %v", name, ids)
		}
	}
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	tree, err := h.financial.Overview(h.ctx, pctx)
	if err != nil {
		t.Fatal(err)
	}
	var treeAccounts []uuid.UUID
	for _, platform := range tree.Platforms {
		for _, item := range platform.Identities {
			for _, account := range item.Accounts {
				treeAccounts = append(treeAccounts, account.AccountID)
			}
		}
	}
	var visibleAccounts []uuid.UUID
	for _, account := range accounts {
		if account.AccountType != string(adapters.TypeManual) {
			visibleAccounts = append(visibleAccounts, account.AccountID)
		}
	}
	if !slices.Equal(treeAccounts, visibleAccounts) {
		t.Fatal("Tree order differs from management order")
	}
	overview, err := h.financial.Overview(h.ctx, pctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, platform := range []string{"binance", "okx", "bybit", "kraken"} {
		if overview.Platforms[i].Platform != platform {
			t.Fatal("Overview changed the platform order")
		}
	}
	balances, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	var balanceAccounts []uuid.UUID
	for _, row := range balances.Items {
		balanceAccounts = append(balanceAccounts, row.AccountID)
	}
	checkHierarchy("Balances", balanceAccounts)
	positions, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 500})
	if err != nil || positions.Total == 0 {
		t.Fatalf("Positions: %+v %v", positions, err)
	}
	var positionAccounts []uuid.UUID
	for offset, row := range positions.Items {
		positionAccounts = append(positionAccounts, row.AccountID)
		page, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Offset: offset, Limit: 1})
		if err != nil || len(page.Items) != 1 || page.Items[0].AccountID != row.AccountID || page.Items[0].PositionKey != row.PositionKey || page.TotalUSD == nil || *page.TotalUSD != *positions.TotalUSD {
			t.Fatalf("Position pagination changed order or total at %d: %+v %v", offset, page, err)
		}
	}
	checkHierarchy("Positions", positionAccounts)
	for _, account := range accounts {
		view, err := h.financial.AccountPositions(h.ctx, pctx, account.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		var flatRefs, detailRefs []string
		for _, row := range positions.Items {
			if row.AccountID == account.AccountID {
				flatRefs = append(flatRefs, row.PositionKey)
			}
		}
		for _, row := range view.Positions {
			detailRefs = append(detailRefs, row.PositionKey)
		}
		if !slices.Equal(flatRefs, detailRefs) {
			t.Fatalf("Account-detail and flat position order disagree: %v / %v", detailRefs, flatRefs)
		}
	}
	duals, err := h.financial.FlatDualPositions(h.ctx, pctx, &financial.ListFilter{Limit: 500})
	if err != nil || duals.Total == 0 {
		t.Fatalf("Dual positions: %+v %v", duals, err)
	}
	var dualAccounts []uuid.UUID
	for _, row := range duals.Items {
		dualAccounts = append(dualAccounts, row.AccountID)
	}
	checkHierarchy("Dual Positions", dualAccounts)

	beforeExit, err := h.valuation.Get(h.ctx, h.portfolioID, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	items, err = h.identities.List(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		item, err = h.identities.Disable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.identities.Archive(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
			t.Fatal(err)
		}
	}
	_, exitCycle := h.runCycle()
	exitPoint, err := h.valuation.Get(h.ctx, h.portfolioID, exitCycle)
	if err != nil {
		t.Fatal(err)
	}
	checkEvents := func(name string, events []valuation.BoundaryEvent) {
		t.Helper()
		ids := make([]uuid.UUID, len(events))
		for i, event := range events {
			ids[i] = event.AccountID
		}
		if !slices.Equal(ids, wantAccounts) {
			t.Fatalf("%s boundary account order = %v, want %v", name, ids, wantAccounts)
		}
	}
	checkEvents("point", exitPoint.BoundaryEvents)
	detail, err := h.financial.Detail(h.ctx, pctx, exitCycle)
	if err != nil {
		t.Fatal(err)
	}
	checkEvents("detail", detail.Point.BoundaryEvents)
	change, err := h.financial.CustomValueChange(h.ctx, pctx, beforeExit.EvaluatedAt, exitPoint.EvaluatedAt)
	if err != nil {
		t.Fatal(err)
	}
	checkEvents("range", change.BoundaryEvents)
	if change.AmountUSD == nil || *change.AmountUSD != "0" {
		t.Fatalf("boundary exits changed the adjusted value change: %+v", change)
	}
	events, err := h.financial.BoundaryEvents(h.ctx, pctx, beforeExit.EvaluatedAt, exitPoint.EvaluatedAt)
	if err != nil {
		t.Fatal(err)
	}
	checkEvents("timeline", events)
}

func TestCatalogBusinessOrderBeforePagination(t *testing.T) {
	h := newHarness(t)
	q := sqlcgen.New(h.db)
	symbol := "ORDER" + uuid.NewString()[:8]
	for _, platform := range []adapters.Platform{adapters.PlatformKraken, adapters.PlatformBybit, adapters.PlatformOKX, adapters.PlatformBinance} {
		if _, _, err := h.catalog.EnsureAsset(h.ctx, q, h.portfolioID, platform, symbol, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := h.catalog.EnsureFuturesContract(h.ctx, q, h.portfolioID, platform, &adapters.FuturesContract{Symbol: symbol, Kind: adapters.KindLinearPerpetual, BaseAssetSymbol: "BTC"}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	filter := catalog.ListFilter{PortfolioID: h.portfolioID, Query: symbol, Offset: 1, Limit: 2}
	assets, err := h.catalog.ListAssets(h.ctx, filter)
	if err != nil || len(assets.Items) != 2 || assets.Items[0].Platform != "okx" || assets.Items[1].Platform != "bybit" || assets.Total != 4 {
		t.Fatalf("Asset page = %+v, %v", assets, err)
	}
	futuresContracts, err := h.catalog.ListFuturesContracts(h.ctx, filter)
	if err != nil || len(futuresContracts.Items) != 2 || futuresContracts.Items[0].Platform != "okx" || futuresContracts.Items[1].Platform != "bybit" || futuresContracts.Total != 4 {
		t.Fatalf("FuturesContract page = %+v, %v", futuresContracts, err)
	}
}
