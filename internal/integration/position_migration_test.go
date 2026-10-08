package integration

import (
	"context"
	"maps"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestPositionMigrationOnPopulatedHistory(t *testing.T) {
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
	for _, spec := range []struct {
		platform  adapters.Platform
		structure adapters.AccountStructure
		account   adapters.AccountType
	}{
		{adapters.PlatformBinance, adapters.StructureBinancePM, "portfolio_margin"},
		{adapters.PlatformOKX, adapters.StructureOKXUnified, "trading"},
		{adapters.PlatformBybit, adapters.StructureBybitUTA2, "unified"},
		{adapters.PlatformKraken, adapters.StructureKrakenUnified, "main"},
	} {
		item := h.createIdentity(string(spec.platform), spec.platform, spec.structure, spec.account)
		var stored identity.Identity
		if spec.platform == adapters.PlatformKraken {
			stored, err = h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}, adapters.Credentials{APIKey: "migration-positions", APISecret: "secret", Futures: &adapters.KeyPair{APIKey: "migration-futures", APISecret: "secret"}})
			if err != nil {
				t.Fatal(err)
			}
		} else {
			stored = h.storeCredentials(item, "migration-positions")
		}
		if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, stored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
			t.Fatalf("enable %+v %v", result, err)
		}
	}
	h.runCycle()
	if err := goose.DownTo(db.DB, ".", 20260929200000); err != nil {
		t.Fatal(err)
	}
	// Simulate the exact canonical refs older adapters retained. Keep one
	// contradictory hedge ref and one case mismatch, both deliberately unknown.
	if _, err := db.DB.ExecContext(h.ctx, `UPDATE positions p JOIN instruments n ON n.id=p.instrument_id
        SET p.platform_position_ref=CASE
        WHEN n.platform='binance' AND p.side='short' THEN CONCAT('um:',n.symbol,':BOTH')
        WHEN n.platform='binance' THEN CONCAT('um:',n.symbol,':SHORT')
        WHEN n.platform='okx' AND p.side='short' THEN CONCAT(n.symbol,':net')
        WHEN n.platform='okx' THEN CONCAT(n.symbol,':LONG')
        WHEN n.platform='kraken' AND p.side='short' THEN CONCAT(n.symbol,':',p.side)
 WHEN n.platform='kraken' THEN CONCAT(n.symbol,':short')
 WHEN n.platform='bybit' AND p.side='short' THEN CONCAT(n.symbol,':0')
        ELSE CONCAT(n.symbol,':1') END`); err != nil {
		t.Fatal(err)
	}
	type source struct{ ref, quantity, side, margin string }
	before := map[string]source{}
	rows, err := db.DB.QueryContext(h.ctx, "SELECT HEX(report_id),platform_position_ref,quantity,side,COALESCE(CAST(margin AS CHAR),'') FROM positions")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		var row source
		if err := rows.Scan(&id, &row.ref, &row.quantity, &row.side, &row.margin); err != nil {
			t.Fatal(err)
		}
		before[id+":"+row.ref] = row
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 {
		t.Fatal("migration requires populated Position history")
	}
	if err := mysql.Up(db.DB); err != nil {
		t.Fatal(err)
	}
	rows, err = db.DB.QueryContext(h.ctx, `SELECT HEX(p.report_id),p.platform_position_ref,p.quantity,p.side,COALESCE(CAST(p.margin AS CHAR),''),COALESCE(p.position_mode,''),COALESCE(p.margin_basis,''),n.platform FROM positions p JOIN instruments n ON n.id=p.instrument_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	after := map[string]source{}
	for rows.Next() {
		var id, mode, basis, platform string
		var row source
		if err := rows.Scan(&id, &row.ref, &row.quantity, &row.side, &row.margin, &mode, &basis, &platform); err != nil {
			t.Fatal(err)
		}
		after[id+":"+row.ref] = row
		want := "one_way"
		if row.side == "long" {
			want = ""
			if platform == "bybit" {
				want = "hedged"
			}
		}
		if mode != want {
			t.Fatalf("%s mode %q, want %q", row.ref, mode, want)
		}
		if platform == "bybit" && row.margin != "" && basis != "initial" {
			t.Fatal("legacy Bybit margin mislabeled")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(before, after) {
		t.Fatalf("migration changed Position history:\nbefore: %+v\nafter: %+v", before, after)
	}
	var modes int
	if err := db.DB.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM account_reports WHERE margin_mode IS NOT NULL").Scan(&modes); err != nil || modes != 0 {
		t.Fatalf("historical account mode guessed: %d %v", modes, err)
	}
}
