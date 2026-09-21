package seeds

import (
	"context"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

const walkthroughCheckDB = "plutus_seedcheck"

func TestWalkthroughSQLMatchesSchema(t *testing.T) {
	dsn := mysql.TestDSN(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := mysql.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	if err := mysql.Up(admin); err != nil {
		t.Fatal(err)
	}

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+walkthroughCheckDB+"`"); err != nil {
		t.Skipf("cannot create %s: %v", walkthroughCheckDB, err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS `"+walkthroughCheckDB+"`")
	})

	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = walkthroughCheckDB
	db, err := mysql.Open(cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := mysql.DropAllTables(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := mysql.Up(db); err != nil {
		t.Fatal(err)
	}
	sqlText, err := WalkthroughSQL()
	if err != nil {
		t.Fatal(err)
	}
	if err := mysql.ExecSQLStatements(ctx, db, sqlText); err != nil {
		t.Fatal(err)
	}

	var observations, identities, audits int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations").Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM identities").Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if observations < 1500 {
		t.Fatalf("observations=%d, want at least 1500", observations)
	}
	if identities < 6 {
		t.Fatalf("identities=%d, want at least 5", identities)
	}
	if audits < 8 {
		t.Fatalf("audit_events=%d, want at least 8", audits)
	}
}
