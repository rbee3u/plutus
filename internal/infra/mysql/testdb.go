package mysql

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

// OpenTestDB opens a migrated MySQL database for integration tests.
func OpenTestDB(tb testing.TB) *sql.DB {
	tb.Helper()
	dsn := os.Getenv("PLUTUS_TEST_DATABASE_DSN")
	if dsn == "" {
		tb.Skip("PLUTUS_TEST_DATABASE_DSN is not set")
	}
	db, err := Open(dsn)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := Up(db); err != nil {
		tb.Fatalf("migrate: %v", err)
	}
	if err := CheckReady(ctx, db); err != nil {
		tb.Fatalf("ready: %v", err)
	}
	return db
}
