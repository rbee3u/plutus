package mysql

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/db/migrations"
)

func TestMigrateUpAndSeeds(t *testing.T) {
	db := OpenTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Up is idempotent on an already migrated database.
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
	if tables != 19 {
		t.Fatalf("base tables %d, want 19", tables)
	}
}

func TestMigrateStatusOutput(t *testing.T) {
	db := OpenTestDB(t)
	var output bytes.Buffer
	t.Cleanup(func() { SetMigrationLogger(nil) })
	if err := Status(db, &output); err != nil {
		t.Fatalf("status: %v", err)
	}

	result := output.String()
	if !strings.HasSuffix(result, "\n") {
		t.Fatalf("status must end with a newline: %q", result)
	}
	lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != len(entries)+2 {
		t.Fatalf("got %d lines, want two header lines and %d migration rows: %q", len(lines), len(entries), result)
	}
	if !strings.Contains(lines[0], "Applied At") || !strings.Contains(lines[0], "Migration") || !strings.HasPrefix(strings.TrimSpace(lines[1]), "===") {
		t.Fatalf("unexpected status header: %q", lines[:2])
	}
	for i, entry := range entries {
		if !strings.HasSuffix(lines[i+2], " -- "+entry.Name()) {
			t.Errorf("migration row %d: got %q, want %s", i, lines[i+2], entry.Name())
		}
	}
}
