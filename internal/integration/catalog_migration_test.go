package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestCatalogMigrationPreservesHistory(t *testing.T) {
	for _, version := range []int64{20261005020000, 20261009020000} {
		t.Run(strconv.FormatInt(version, 10), func(t *testing.T) { testCatalogMigrationHistory(t, version) })
	}
}

func testCatalogMigrationHistory(t *testing.T, version int64) {
	db, err := mysqltest.CreateTestDatabase(context.Background(), mysqltest.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	h := newHarness(t, db.DB)
	identity := manualEnabled(t, h, &manualTestAdapter{usd: "30", positions: true})
	state := manualSave(t, h, identity, "DEMO-USD", "5")
	asset, err := h.catalog.GetAsset(h.ctx, state.Items[0].AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutMapping(h.ctx, asset.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: h.portfolioID, Version: asset.Version, CanonicalSymbol: "USD"}); err != nil {
		t.Fatal(err)
	}
	assetID := state.Items[0].AssetID
	excluded, err := h.catalog.PutExclusion(h.ctx, assetID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: true, Reason: "Reviewed test asset"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutExclusion(h.ctx, assetID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Version: excluded.ExclusionVersion}); err != nil {
		t.Fatal(err)
	}
	initial, job := h.runCycle()
	if _, _, err := h.jobs.EnqueueRecalc(h.ctx, h.portfolioID, jobs.RecalcPayload{Reason: "mapping", AssetID: assetID.String()}, &h.userID); err != nil {
		t.Fatal(err)
	}
	queued, _, err := h.jobs.EnqueueCycle(h.ctx, h.portfolioID, jobs.TriggerManual, nil)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := h.manual.Freeze(h.ctx, h.portfolioID, queued.ID)
	if err != nil || len(frozen) == 0 {
		t.Fatalf("freeze %+v %v", frozen, err)
	}
	// Populate every renamed reference with non-canonical codes. A schema rename
	// must preserve these observations rather than applying current mappings.
	for _, query := range []string{
		`INSERT INTO dual_positions (report_id,portfolio_id,platform_position_ref,base_asset_symbol,quote_asset_symbol,invest_asset_symbol,principal,status)
		 SELECT id,portfolio_id,'migration-dual','XBT','ZUSD','ZUSD',12.5,'active' FROM account_reports LIMIT 1`,
		`INSERT INTO prices (platform,canonical_asset_symbol,observed_at,price,quote_canonical_asset_symbol) VALUES ('kraken','BTC','2026-01-01 00:00:00.123456',123.5,'USD')`,
		`UPDATE futures_contracts SET base_asset_symbol='XBT',quote_asset_symbol='ZUSD',settlement_asset_symbol='XBT'`,
		`UPDATE positions SET margin_asset_symbol='XBT'`,
	} {
		if _, err := db.DB.ExecContext(h.ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	wantSchema := catalogSchema(t, db.DB)
	assertAssetReferenceColumns(t, db.DB)
	if err := goose.DownTo(db.DB, ".", version); err != nil {
		t.Fatal(err)
	}
	if version == 20261005020000 {
		// Reconstruct the actual pre-upgrade JSON protocol without changing values.
		for _, table := range []string{"manual_revisions", "manual_cycle_accounts"} {
			// #nosec G202 -- table is selected from this fixed fixture list.
			if _, err := db.DB.ExecContext(h.ctx, "UPDATE "+table+" SET balances_json=REPLACE(REPLACE(CAST(balances_json AS CHAR), '\"platformAssetId\"', '\"listingId\"'), '\"platformAssetSymbol\"', '\"symbol\"')"); err != nil {
				t.Fatal(err)
			}
		}
	}
	legacyAction, legacyResource := "catalog.listing.", "asset_listing"
	if version == 20261009020000 {
		legacyAction, legacyResource = "catalog.platform_asset.", "platform_asset"
	}
	if _, err := db.DB.ExecContext(h.ctx, "UPDATE audit_events SET action=REPLACE(action,'catalog.asset.',?),resource_type=? WHERE resource_type='asset'", legacyAction, legacyResource); err != nil {
		t.Fatal(err)
	}

	tables := [][2]string{{"asset_listings", "assets"}, {"instruments", "futures_contracts"}, {"balances", "balances"}, {"positions", "positions"}, {"manual_balances", "manual_balances"}, {"dual_positions", "dual_positions"}, {"prices", "prices"}, {"portfolio_listing_exclusions", "portfolio_asset_exclusions"}, {"portfolio_listing_exclusion_events", "portfolio_asset_exclusion_events"}, {"manual_revisions", "manual_revisions"}, {"manual_cycle_accounts", "manual_cycle_accounts"}, {"account_reports", "account_reports"}, {"valuation_points", "valuation_points"}, {"audit_events", "audit_events"}, {"jobs", "jobs"}}
	if version == 20261009020000 {
		names := map[string]string{"asset_listings": "platform_assets", "instruments": "futures_contracts", "portfolio_listing_exclusions": "portfolio_platform_asset_exclusions", "portfolio_listing_exclusion_events": "portfolio_platform_asset_exclusion_events"}
		for i := range tables {
			if name, ok := names[tables[i][0]]; ok {
				tables[i][0] = name
			}
		}
	}

	before := make([][]string, len(tables))
	for i, pair := range tables {
		before[i] = catalogTableValues(t, db.DB, pair[0])
	}
	if err := mysql.Up(db.DB); err != nil {
		t.Fatal(err)
	}
	assertAssetReferenceColumns(t, db.DB)
	if got := catalogSchema(t, db.DB); !reflect.DeepEqual(wantSchema, got) {
		t.Fatalf("schema round trip changed columns, keys, or checks:\nwant %v\ngot %v", wantSchema, got)
	}
	var legacyNames string
	if err := db.DB.QueryRowContext(t.Context(), `SELECT COALESCE(GROUP_CONCAT(DISTINCT CONCAT(table_name, '.', name)), '')
FROM (
    SELECT table_name, index_name AS name FROM information_schema.statistics WHERE table_schema=DATABASE()
    UNION ALL
    SELECT table_name, constraint_name AS name FROM information_schema.table_constraints WHERE table_schema=DATABASE()
) names
WHERE name LIKE '%listing%' OR name LIKE '%instrument%' OR name LIKE '%platform_asset%'`).Scan(&legacyNames); err != nil {
		t.Fatal(err)
	}
	if legacyNames != "" {
		t.Fatalf("legacy index or constraint names: %s", legacyNames)
	}
	for i, pair := range tables {
		if after := catalogTableValues(t, db.DB, pair[1]); !reflect.DeepEqual(before[i], after) {
			t.Fatalf("migration changed %s values", pair[0])
		}
	}
	current, err := h.manual.Get(h.ctx, h.portfolioID, identity.ID)
	if err != nil || len(current.AppliedItems) != 1 || current.AppliedItems[0].AssetID != assetID || current.AppliedItems[0].AssetSymbol != "DEMO-USD" || current.AppliedItems[0].CanonicalAssetSymbol != "USD" {
		t.Fatalf("historical manual read %+v %v", current, err)
	}
	recovered, err := h.manual.Freeze(h.ctx, h.portfolioID, queued.ID)
	if err != nil || !reflect.DeepEqual(frozen, recovered) {
		t.Fatalf("frozen history changed: %v", err)
	}
	events, err := h.services.Audit.List(h.ctx, h.portfolioID, &audit.ListFilter{Action: "catalog.asset.exclude", ResourceType: "asset"})
	if err != nil || len(events.Items) != 1 || events.Items[0].Action != legacyAction+"exclude" {
		t.Fatalf("legacy audit filtering %+v %v", events, err)
	}
	for _, generation := range []struct{ action, resource string }{{"catalog.asset.exclude", "asset"}, {"catalog.platform_asset.exclude", "platform_asset"}, {"catalog.listing.exclude", "asset_listing"}} {
		if _, err := db.DB.ExecContext(h.ctx, "INSERT INTO audit_events (id,portfolio_id,actor_user_id,action,resource_type,resource_id,request_id,occurred_at) VALUES (UUID_TO_BIN(UUID()),?,?,?,?,?,'',UTC_TIMESTAMP(6))", h.portfolioID[:], h.userID[:], generation.action, generation.resource, assetID[:]); err != nil {
			t.Fatal(err)
		}
	}
	all, err := h.services.Audit.List(h.ctx, h.portfolioID, &audit.ListFilter{Action: "catalog.asset.exclude", ResourceType: "asset"})
	if err != nil || len(all.Items) != 4 {
		t.Fatalf("three audit generations: %+v %v", all, err)
	}

	if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, queued.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	point, err := h.valuation.PointByJob(h.ctx, h.portfolioID, job)
	if err != nil || point.TotalUSD != initial.Point.TotalUSD || !point.EvaluatedAt.Equal(initial.Point.EvaluatedAt) {
		t.Fatalf("history changed %+v %v", point, err)
	}
}

func TestCatalogMigrationRejectsSpotBeforeDDL(t *testing.T) {
	db, err := mysqltest.CreateTestDatabase(context.Background(), mysqltest.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := goose.DownTo(db.DB, ".", 20261005020000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(t.Context(), "INSERT INTO instruments (id,platform,symbol,kind,first_seen_at,last_seen_at,created_at,updated_at) VALUES (UUID_TO_BIN(UUID()),'binance','BTCUSDT','spot',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))"); err != nil {
		t.Fatal(err)
	}
	if err := mysql.Up(db.DB); err == nil || !strings.Contains(err.Error(), "spot") {
		t.Fatalf("expected spot preflight: %v", err)
	}
	version, err := mysql.CurrentVersion(db.DB)
	if err != nil || version != 20261005020000 {
		t.Fatalf("version changed %d %v", version, err)
	}
	if len(catalogTableValues(t, db.DB, "instruments")) != 1 {
		t.Fatal("preflight changed data")
	}
}

// Compare physical values independently of renamed columns and index scan order.
func catalogTableValues(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	// #nosec G202 -- callers supply only the fixed migration fixture table names.
	rows, err := db.QueryContext(t.Context(), "SELECT * FROM "+table)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result []string
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, string(encoded))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

// Capture physical schema metadata, including implicit FK indexes, column types,
// key ordering, delete rules, and check expressions across every application table.
func catalogSchema(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var result []string
	for _, query := range []string{
		`SELECT CAST(JSON_ARRAY(table_name,column_name,ordinal_position,column_type,is_nullable,column_default,collation_name,extra) AS CHAR) FROM information_schema.columns WHERE table_schema=DATABASE()`,
		`SELECT CAST(JSON_ARRAY(table_name,index_name,non_unique,seq_in_index,column_name,index_type) AS CHAR) FROM information_schema.statistics WHERE table_schema=DATABASE()`,
		`SELECT CAST(JSON_ARRAY(table_name,constraint_name,column_name,ordinal_position,referenced_table_name,referenced_column_name) AS CHAR) FROM information_schema.key_column_usage WHERE table_schema=DATABASE()`,
		`SELECT CAST(JSON_ARRAY(table_name,constraint_name,update_rule,delete_rule) AS CHAR) FROM information_schema.referential_constraints WHERE constraint_schema=DATABASE()`,
		`SELECT CAST(JSON_ARRAY(constraint_name,check_clause) AS CHAR) FROM information_schema.check_constraints WHERE constraint_schema=DATABASE()`,
	} {
		rows, err := db.QueryContext(t.Context(), query)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if err := rows.Close(); err != nil {
					t.Error(err)
				}
			}()
			for rows.Next() {
				var value string
				if err := rows.Scan(&value); err != nil {
					t.Fatal(err)
				}
				result = append(result, value)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		}()
	}
	sort.Strings(result)
	return result
}

func assertAssetReferenceColumns(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT CONCAT(table_name,'.',column_name)
		FROM information_schema.columns WHERE table_schema=DATABASE()
		AND (column_name LIKE '%symbol%' OR column_name LIKE '%currency%')
		ORDER BY table_name,column_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		got = append(got, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"capital_flows.canonical_asset_symbol",
		"dual_positions.base_asset_symbol",
		"dual_positions.invest_asset_symbol",
		"dual_positions.quote_asset_symbol",
		"futures_contracts.base_asset_symbol",
		"futures_contracts.quote_asset_symbol",
		"futures_contracts.settlement_asset_symbol",
		"futures_contracts.symbol",
		"manual_balances.asset_symbol",
		"assets.canonical_symbol",
		"assets.symbol",
		"positions.margin_asset_symbol",
		"prices.canonical_asset_symbol",
		"prices.quote_canonical_asset_symbol",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("asset references: got %v, want %v", got, want)
	}
}
