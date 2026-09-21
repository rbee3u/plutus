package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/config"
)

const testDatabaseDSNEnv = "PLUTUS_TEST_DATABASE_DSN"

// TestDSN returns the MySQL DSN for integration tests.
// It reads PLUTUS_TEST_DATABASE_DSN from the process environment, then `config/test.env` in the working directory or Go module root.
func TestDSN(tb testing.TB) string {
	tb.Helper()
	dsn, err := config.Lookup(testDatabaseDSNEnv)
	if err != nil {
		tb.Fatal(err)
	}
	if dsn == "" {
		tb.Skip("PLUTUS_TEST_DATABASE_DSN is not set in the environment, working-directory config/test.env, or module-root config/test.env")
	}
	return dsn
}

// OpenTestDB opens a migrated MySQL database for integration tests.
func OpenTestDB(tb testing.TB) *sql.DB {
	tb.Helper()
	dsn := TestDSN(tb)
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
