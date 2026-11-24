package financial

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestExposureDatabaseAggregatesAcrossAccountsAndKeepsDualPrincipal(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	at := time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)

	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "exposure-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("portfolio: %+v %v", owned, err)
	}
	p := PortfolioContext{ID: owned[0].ID, Timezone: "UTC"}

	identities, err := identity.NewService(db, identity.Options{})
	if err != nil {
		t.Fatal(err)
	}
	item, err := identities.Create(ctx, p.ID, &identity.CreateInput{
		ActorUserID:          user.ID,
		Platform:             adapter.PlatformBinance,
		DisplayName:          "Main",
		AccountStructure:     adapter.StructureBinanceClassic,
		SelectedAccountTypes: []adapter.AccountType{"spot", "um_futures", adapter.TypeDualInvestment},
	})
	if err != nil {
		t.Fatal(err)
	}
	accounts := map[adapter.AccountType]uuid.UUID{}
	for _, account := range item.Accounts {
		accounts[account.AccountType] = account.ID
	}

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	cycle := uuid.New()
	exec("INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES (?,?,'succeeded','schedule',?,?,?,?)", p.ID[:], cycle[:], at, at, at, at)

	btcAsset, usdtAsset := uuid.New(), uuid.New()
	exec("INSERT INTO assets (id,portfolio_id,platform,symbol,canonical_asset_symbol,first_observed_at,last_observed_at,mapping_version) VALUES (?,?,'binance','BTC','BTC',?,?,1),(?,?,'binance','USDT','USDT',?,?,1)", btcAsset[:], p.ID[:], at, at, usdtAsset[:], p.ID[:], at, at)
	price := uuid.New()
	exec("INSERT INTO price_observations (id,portfolio_id,source_market_ref,platform,requested_canonical_asset_symbol,retrieved_at,price,quote_canonical_asset_symbol) VALUES (?,?,'fixture','binance','BTC',?,60000,'USDT')", price[:], p.ID[:], at)

	report := func(kind adapter.AccountType, equity any) uuid.UUID {
		id := uuid.New()
		accountID := accounts[kind]
		exec("INSERT INTO account_reports (id,portfolio_id,cycle_id,account_id,status,collected_at,recorded_at,reported_equity_usd) VALUES (?,?,?,?,'succeeded',?,?,?)", id[:], p.ID[:], cycle[:], accountID[:], at, at, equity)
		return id
	}
	balance := func(reportID, assetID uuid.UUID, equity string) {
		exec("INSERT INTO balances (portfolio_id,report_id,asset_id,equity) VALUES (?,?,?,?)", p.ID[:], reportID[:], assetID[:], equity)
	}

	spotReport := report("spot", nil)
	balance(spotReport, btcAsset, "0.3")
	futuresReport := report("um_futures", nil)
	balance(futuresReport, usdtAsset, "1000")
	dualReport := report(adapter.TypeDualInvestment, nil)
	balance(dualReport, usdtAsset, "60")
	manualReport := report(adapter.TypeManual, nil)
	balance(manualReport, btcAsset, "0")

	contract := uuid.New()
	exec("INSERT INTO futures_contracts (portfolio_id,id,platform,symbol,terms_hash,kind,base_asset_symbol,quote_asset_symbol,settlement_asset_symbol,contract_size,first_observed_at,last_observed_at) VALUES (?,?,'binance','BTCUSDT',UNHEX(REPEAT('ab',32)),'linear_perpetual','BTC','USDT','USDT',1,?,?)", p.ID[:], contract[:], at, at)
	exec("INSERT INTO futures_positions (portfolio_id,report_id,position_key,futures_contract_id,side,position_mode,source_quantity,source_quantity_unit,entry_price,mark_price,unrealized_pnl,margin_setting,margin_asset_symbol,margin_basis) VALUES (?,?,'perp-1',?,'long','one_way',1,'base',59000,60000,0,'cross','USDT','initial')", p.ID[:], futuresReport[:], contract[:])

	values := valuation.NewService(db)
	if _, err := values.WritePoint(ctx, p.ID, cycle, at); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, values)
	list, err := service.Exposure(ctx, p, &ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Summary.AssetCount != 2 || list.Summary.LongCount != 2 || list.Coverage.MissingAccountCount != 0 || !list.Coverage.Complete {
		t.Fatalf("summary/coverage = %+v %+v", list.Summary, list.Coverage)
	}
	if len(list.Items) != 2 {
		t.Fatalf("items = %+v", list.Items)
	}
	rows := map[string]ExposureRow{}
	for _, row := range list.Items {
		rows[row.CanonicalAssetSymbol] = row
	}
	btc := rows["BTC"]
	if btc.NetQuantity == nil || *btc.NetQuantity != "1.3" || btc.AccountCount != 3 {
		t.Fatalf("BTC row = %+v", btc)
	}
	if btc.PriceUSD == nil || *btc.PriceUSD != "60000" || btc.ValueUSD == nil || *btc.ValueUSD != "78000" {
		t.Fatalf("BTC value = %+v", btc)
	}
	usdt := rows["USDT"]
	if usdt.NetQuantity == nil || *usdt.NetQuantity != "1060" || usdt.AccountCount != 2 {
		t.Fatalf("USDT row = %+v", usdt)
	}
	if usdt.ValueUSD == nil || *usdt.ValueUSD != "1060" {
		t.Fatalf("USDT value = %+v", usdt)
	}
	if list.Summary.StablecoinValueUSD == nil || *list.Summary.StablecoinValueUSD != "1060" {
		t.Fatalf("stablecoin value = %+v", list.Summary.StablecoinValueUSD)
	}
	if list.Summary.NonStablecoinValueUSD == nil || *list.Summary.NonStablecoinValueUSD != "78000" {
		t.Fatalf("non-stablecoin value = %+v", list.Summary.NonStablecoinValueUSD)
	}
	if list.Summary.TotalValueUSD == nil || *list.Summary.TotalValueUSD != "79060" {
		t.Fatalf("total value = %+v", list.Summary.TotalValueUSD)
	}
	if list.Summary.UnpricedAssetCount != 0 || len(list.Summary.UnpricedAssetSymbols) != 0 {
		t.Fatalf("unpriced = %+v", list.Summary)
	}
	if len(list.Summary.StablecoinSymbols) != 1 || list.Summary.StablecoinSymbols[0] != "USDT" {
		t.Fatalf("stablecoin symbols = %+v", list.Summary.StablecoinSymbols)
	}
	if list.Summary.Completeness != valuation.CompletenessComplete {
		t.Fatalf("completeness = %+v", list.Summary)
	}
	if list.Items[0].CanonicalAssetSymbol != "BTC" {
		t.Fatalf("default value order = %+v", list.Items)
	}

	filtered, err := service.Exposure(ctx, p, &ListFilter{Symbol: "btc"})
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].CanonicalAssetSymbol != "BTC" || filtered.Summary.AssetCount != 1 {
		t.Fatalf("symbol filter = %+v %v", filtered, err)
	}
	if filtered.Summary.StablecoinValueUSD == nil || *filtered.Summary.StablecoinValueUSD != "0" {
		t.Fatalf("filtered stablecoin value = %+v", filtered.Summary.StablecoinValueUSD)
	}
	if filtered.Summary.NonStablecoinValueUSD == nil || *filtered.Summary.NonStablecoinValueUSD != "78000" {
		t.Fatalf("filtered non-stablecoin value = %+v", filtered.Summary.NonStablecoinValueUSD)
	}
	if filtered.Summary.TotalValueUSD == nil || *filtered.Summary.TotalValueUSD != "78000" {
		t.Fatalf("filtered total value = %+v", filtered.Summary.TotalValueUSD)
	}
	if len(filtered.Summary.StablecoinSymbols) != 0 {
		t.Fatalf("filtered stablecoin symbols = %+v", filtered.Summary.StablecoinSymbols)
	}
	page, err := service.Exposure(ctx, p, &ListFilter{Limit: 1, Offset: 1})
	if err != nil || len(page.Items) != 1 || page.Total != 2 || page.Offset != 1 || page.Limit != 1 {
		t.Fatalf("page = %+v %v", page, err)
	}
	if page.Summary.TotalValueUSD == nil || *page.Summary.TotalValueUSD != "79060" {
		t.Fatalf("page summary value = %+v", page.Summary.TotalValueUSD)
	}
	symbolOrder, err := service.Exposure(ctx, p, &ListFilter{Sort: ExposureSortSymbol})
	if err != nil || len(symbolOrder.Items) != 2 || symbolOrder.Items[0].CanonicalAssetSymbol != "BTC" || symbolOrder.Items[1].CanonicalAssetSymbol != "USDT" {
		t.Fatalf("symbol order = %+v %v", symbolOrder, err)
	}
	if _, err := service.Exposure(ctx, p, &ListFilter{Sort: "bad"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid sort error = %v", err)
	}

	detail, err := service.ExposureDetail(ctx, p, &ListFilter{}, "btc")
	if err != nil {
		t.Fatal(err)
	}
	if detail.AccountCount != 3 || len(detail.Contributions) != 3 || detail.NetQuantity == nil || *detail.NetQuantity != "1.3" {
		t.Fatalf("detail = %+v", detail)
	}
	var spot, futures, manual *ExposureContribution
	for i := range detail.Contributions {
		c := &detail.Contributions[i]
		switch c.AccountType {
		case "spot":
			spot = c
		case "um_futures":
			futures = c
		case "manual":
			manual = c
		}
	}
	if spot == nil || spot.Quantity == nil || *spot.Quantity != "0.3" || spot.ContractOnly {
		t.Fatalf("spot contribution = %+v", spot)
	}
	if futures == nil || futures.Quantity == nil || *futures.Quantity != "1" || !futures.ContractOnly {
		t.Fatalf("futures contribution = %+v", futures)
	}
	if manual == nil || manual.Quantity == nil || *manual.Quantity != "0" || manual.ContractOnly {
		t.Fatalf("manual zero contribution = %+v", manual)
	}
	if _, err := service.ExposureDetail(ctx, p, &ListFilter{}, "missing"); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("missing detail error = %v", err)
	}
}
