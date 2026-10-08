package mysql_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/infra/logging"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestLoggedMySQLExecution(t *testing.T) {
	dsn := mysqltest.TestDSN(t)
	if err := os.MkdirAll("/tmp/plutus", 0o750); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("/tmp/plutus", "mysql-logging-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	logs, err := logging.New(logging.Options{Directory: dir, MaxSizeMB: 1, SQLEnabled: true}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logs.Close() }()
	db, err := mysql.Open(dsn, mysql.Loggers{App: logs.App, SQL: logs.SQL})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ctx := logging.WithFields(context.Background(), slog.String("trace_id", "sql-test"), slog.String("job_id", "job-test"))
	var number uint64
	var binary []byte
	var nullable sql.NullString
	var text string
	err = db.QueryRowContext(ctx, "SELECT ?, ?, ?, ?", uint64(math.MaxUint64), []byte{0, 255}, nil, "O'Brien").Scan(&number, &binary, &nullable, &text)
	if err != nil {
		t.Fatal(err)
	}
	if number != math.MaxUint64 || !bytes.Equal(binary, []byte{0, 255}) || nullable.Valid || text != "O'Brien" {
		t.Fatal("parameter semantics changed")
	}
	instant := time.Date(2026, 10, 2, 1, 2, 3, 123456789, time.UTC)
	if err := db.QueryRowContext(ctx, "SELECT ?, ?", json.RawMessage(nil), instant).Scan(&nullable, &text); err != nil {
		t.Fatal(err)
	}
	if nullable.Valid || text != "2026-10-02 01:02:03.123456789" {
		t.Fatalf("driver parameter representation changed: %v %q", nullable, text)
	}
	stmt, err := db.PrepareContext(ctx, "SELECT ?")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stmt.Close() }()
	if err := stmt.QueryRowContext(ctx, uint64(math.MaxUint64)).Scan(&number); err != nil {
		_ = stmt.Close()
		t.Fatal(err)
	}
	_ = stmt.Close()
	if number != math.MaxUint64 {
		t.Fatal("statement conversion changed")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var one int
	if err := tx.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "SELECT missing_logging_column"); err == nil {
		t.Fatal("expected SQL error")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sql.log"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(raw)
	for _, want := range []string{"SET time_zone", "trace_id=sql-test", "job_id=job-test", "BEGIN", "COMMIT", "ROLLBACK", "level=ERROR", "O''Brien", "X'00ff'", "SELECT NULL, '2026-10-02 01:02:03.123456789'"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in SQL log", want)
		}
	}
	if strings.Count(output, "SELECT 18446744073709551615") != 2 {
		t.Fatal("prepared execution logged more than once")
	}
}
