package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/catalog"
)

func TestMigrateUpAndSeeds(t *testing.T) {
	dsn := TestDSN(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := Up(db); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := CheckEnvironment(ctx, db); err != nil {
		t.Fatalf("environment: %v", err)
	}
	version, err := CurrentVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	if version != ExpectedSchemaVersion {
		t.Fatalf("version %d, want %d", version, ExpectedSchemaVersion)
	}

	var usd []byte
	if err := db.QueryRowContext(ctx, "SELECT id FROM assets WHERE code = ?", "USD").Scan(&usd); err != nil {
		t.Fatal(err)
	}
	if catalog.AssetUSD.String() == "" {
		t.Fatal("usd constant")
	}
	got, err := catalog.AssetUSD.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if string(usd) != string(got) {
		t.Fatalf("usd id mismatch")
	}
}
