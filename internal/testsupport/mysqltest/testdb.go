// Package mysqltest owns isolated, per-package MySQL databases for tests.
package mysqltest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// GotestDSNEnv names the server-level MySQL DSN that `go test` uses to create throwaway databases.
// It is read from the process environment or from `config/gotest.env` (see config.Lookup).
const GotestDSNEnv = "PLUTUS_GOTEST_DSN"

// gotestPrefix is the name prefix of every throwaway database the harness creates.
const gotestPrefix = "plutus_gotest_"

// gotestStaleAfter is the age after which a leftover throwaway database is swept.
const gotestStaleAfter = time.Hour

var gotestDatabaseName = regexp.MustCompile(`^plutus_gotest_(\d+)_[0-9a-f]+$`)

// GotestDSN returns the server-level DSN for `go test`, or "" when tests must skip.
// The DSN must not select a database; the harness creates its own.
func GotestDSN(tb testing.TB) string {
	tb.Helper()
	dsn, err := config.Lookup(GotestDSNEnv)
	if err != nil {
		tb.Fatal(err)
	}
	if dsn == "" {
		tb.Skip(GotestDSNEnv + " is not set in the environment or config/gotest.env")
	}
	return dsn
}

// TestDatabase is one throwaway database created for a test package.
type TestDatabase struct {
	// Name is the database name, for example plutus_gotest_1758600000_3fa9c1e2.
	Name string
	// DSN selects the database with the server credentials of PLUTUS_GOTEST_DSN.
	DSN string
	// DB is a migrated pool on the database. The harness closes it in Drop.
	DB *sql.DB

	admin *sql.DB
}

// RunTestMain is the TestMain body for a package whose tests need MySQL.
//
// It creates one migrated throwaway database for the package, points OpenTestDB and TestDSN at
// it, runs m, drops the database, and exits with the test status. Without PLUTUS_GOTEST_DSN the
// tests run and the database-backed ones skip. Packages that need no database do not call it.
func RunTestMain(m *testing.M) {
	dsn, err := config.Lookup(GotestDSNEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dsn == "" {
		os.Exit(m.Run())
	}
	tdb, err := CreateTestDatabase(context.Background(), dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	packageDatabase = tdb
	code := m.Run()
	if err := tdb.Drop(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// packageDatabase is the database RunTestMain created for the current test binary. It is read by
// TestDSN and OpenTestDB while m.Run executes.
var packageDatabase *TestDatabase

// CreateTestDatabase creates and migrates a throwaway database on the server that serverDSN
// selects. It also drops throwaway databases older than one hour left by killed test runs.
func CreateTestDatabase(ctx context.Context, serverDSN string) (*TestDatabase, error) {
	cfg, err := mysqldriver.ParseDSN(serverDSN)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", GotestDSNEnv, err)
	}
	if cfg.DBName != "" {
		return nil, fmt.Errorf("%s must not select a database (got %q); the test harness creates plutus_gotest_* databases itself", GotestDSNEnv, cfg.DBName)
	}
	admin, err := mysql.Open(cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		return nil, fmt.Errorf("%s: %w", GotestDSNEnv, err)
	}
	if err := sweepStaleTestDatabases(ctx, admin, time.Now()); err != nil {
		_ = admin.Close()
		return nil, err
	}
	name := newTestDatabaseName(time.Now())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		return nil, fmt.Errorf("create %s: %w", name, err)
	}
	cfg.DBName = name
	db, err := mysql.Open(cfg.FormatDSN())
	if err != nil {
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS `"+name+"`")
		_ = admin.Close()
		return nil, err
	}
	tdb := &TestDatabase{Name: name, DSN: cfg.FormatDSN(), DB: db, admin: admin}
	if err := mysql.Up(ctx, db); err != nil {
		_ = tdb.Drop(ctx)
		return nil, err
	}
	if err := mysql.CheckReady(ctx, db); err != nil {
		_ = tdb.Drop(ctx)
		return nil, err
	}
	return tdb, nil
}

// Drop closes the pool and drops the database.
func (t *TestDatabase) Drop(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if t.DB != nil {
		_ = t.DB.Close()
		t.DB = nil
	}
	if t.admin == nil {
		return nil
	}
	defer func() { _ = t.admin.Close(); t.admin = nil }()
	if _, err := t.admin.ExecContext(ctx, "DROP DATABASE IF EXISTS `"+t.Name+"`"); err != nil {
		return fmt.Errorf("drop %s: %w", t.Name, err)
	}
	return nil
}

// TestDSN returns the DSN of the package's throwaway database. It skips the test when
// PLUTUS_GOTEST_DSN is unset and fails it when the package forgot to call RunTestMain.
func TestDSN(tb testing.TB) string {
	tb.Helper()
	GotestDSN(tb)
	if packageDatabase == nil {
		tb.Fatal("this package needs `func TestMain(m *testing.M) { mysqltest.RunTestMain(m) }`")
	}
	return packageDatabase.DSN
}

// OpenTestDB returns the package's migrated throwaway database. The pool is shared by every
// test in the package and closed by RunTestMain; callers must not close it.
func OpenTestDB(tb testing.TB) *sql.DB {
	tb.Helper()
	TestDSN(tb)
	return packageDatabase.DB
}

func newTestDatabaseName(now time.Time) string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return gotestPrefix + strconv.FormatInt(now.Unix(), 10) + "_" + hex.EncodeToString(buf[:])
}

// sweepStaleTestDatabases drops plutus_gotest_* databases whose timestamp is older than an hour.
// Parallel test binaries create their own fresh databases, so only abandoned ones are old enough.
func sweepStaleTestDatabases(ctx context.Context, admin *sql.DB, now time.Time) error {
	rows, err := admin.QueryContext(ctx, "SHOW DATABASES LIKE '"+gotestPrefix+"%'")
	if err != nil {
		return fmt.Errorf("list test databases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var stale []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan database name: %w", err)
		}
		if isStaleTestDatabase(name, now) {
			stale = append(stale, name)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list test databases: %w", err)
	}
	for _, name := range stale {
		if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS `"+name+"`"); err != nil {
			return fmt.Errorf("drop stale %s: %w", name, err)
		}
	}
	return nil
}

func isStaleTestDatabase(name string, now time.Time) bool {
	match := gotestDatabaseName.FindStringSubmatch(strings.ToLower(name))
	if match == nil {
		return false
	}
	created, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return false
	}
	return now.Sub(time.Unix(created, 0)) > gotestStaleAfter
}
