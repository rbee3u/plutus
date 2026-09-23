package mysql

import (
	"context"
	"testing"
	"time"
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

	var tables int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME <> 'goose_db_version'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 16 {
		t.Fatalf("base tables %d, want 16", tables)
	}
}
