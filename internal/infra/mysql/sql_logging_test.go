package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/infra/logging"
)

func TestRenderSQL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, query, want string
		values            []driver.Value
		fail              bool
	}{
		{"types", "SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?", "SELECT NULL, 1, -1, 18446744073709551615, 1.5, 'O''Brien', X'00ff', '2026-10-02 01:02:03.123456789', '0000-00-00'", []driver.Value{nil, true, int64(-1), uint64(math.MaxUint64), 1.5, "O'Brien", []byte{0, 255}, time.Date(2026, 10, 2, 9, 2, 3, 123456789, time.FixedZone("local", 8*3600)), time.Time{}}, false},
		{"quotes", "SELECT '?', \"?\", `?`, ?, 'a''?b'", "SELECT '?', \"?\", `?`, 42, 'a''?b'", []driver.Value{int64(42)}, false},
		{"comments", "-- ?\nSELECT ? /* ? */ # ?\n", "-- ?\nSELECT 42 /* ? */ # ?\n", []driver.Value{int64(42)}, false},
		{"minus", "SELECT 1--?", "SELECT 1--42", []driver.Value{int64(42)}, false},
		{"mode independent string", "SELECT ?", "SELECT CONVERT(X'615c620a' USING utf8mb4)", []driver.Value{"a\\b\n"}, false},
		{"binary null", "SELECT ?, ?", "SELECT NULL, X''", []driver.Value{[]byte(nil), []byte{}}, false},
		{"json null", "SELECT ?, ?, ?", "SELECT NULL, '', '{\"key\":\"value\"}'", []driver.Value{json.RawMessage(nil), json.RawMessage{}, json.RawMessage(`{"key":"value"}`)}, false},
		{"time trailing zeros", "SELECT ?", "SELECT '2026-10-02 00:00:00.12'", []driver.Value{time.Date(2026, 10, 2, 0, 0, 0, 120000000, time.UTC)}, false},
		{"false", "SELECT ?", "SELECT 0", []driver.Value{false}, false},
		{"literal statement", "INSERT INTO users VALUES ('hash')", "INSERT INTO users VALUES ('hash')", nil, false},
		{"mismatch", "SELECT ?, ?", "", []driver.Value{int64(1)}, true},
		{"unterminated", "SELECT ? /*", "", []driver.Value{int64(1)}, true},
		{"sql mode ambiguity", "SELECT 'a\\'?', ?", "", []driver.Value{int64(1)}, true},
		{"executable comment", "SELECT /*! ? */ ?", "", []driver.Value{int64(1)}, true},
		{"unsupported", "SELECT ?", "", []driver.Value{struct{}{}}, true},
		{"nan", "SELECT ?", "", []driver.Value{math.NaN()}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := renderSQL(tc.query, namedValues(tc.values))
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("rendered %q err=%v; want %q failure=%v", got, err, tc.want, tc.fail)
			}
		})
	}
}

type testConnector struct{ conn driver.Conn }

func (c testConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (testConnector) Driver() driver.Driver                          { return testDriver{} }

type testDriver struct{}

func (testDriver) Open(string) (driver.Conn, error) { return &testConn{}, nil }

type testConn struct {
	prepares               int
	checked, reset, pinged bool
}

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	c.prepares++
	if query == "bad prepare" {
		return nil, errors.New("prepare failed")
	}
	return &testStmt{query: query}, nil
}
func (*testConn) Close() error              { return nil }
func (*testConn) Begin() (driver.Tx, error) { return testTx{}, nil }
func (*testConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) > 0 {
		return nil, driver.ErrSkip
	}
	if query == "fail" {
		return nil, errors.New("exec failed")
	}
	return driver.RowsAffected(1), nil
}

func (*testConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, driver.ErrSkip
}
func (c *testConn) CheckNamedValue(*driver.NamedValue) error { c.checked = true; return nil }
func (c *testConn) ResetSession(context.Context) error       { c.reset = true; return nil }
func (c *testConn) Ping(context.Context) error               { c.pinged = true; return nil }
func (*testConn) IsValid() bool                              { return true }

type testStmt struct{ query string }

func (*testStmt) Close() error                               { return nil }
func (*testStmt) NumInput() int                              { return -1 }
func (*testStmt) CheckNamedValue(*driver.NamedValue) error   { return nil }
func (*testStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(1), nil }
func (*testStmt) Query([]driver.Value) (driver.Rows, error)  { return &testRows{}, nil }

type testRows struct{ read bool }

func (*testRows) Columns() []string                     { return []string{"value"} }
func (*testRows) Close() error                          { return nil }
func (*testRows) ColumnTypeDatabaseTypeName(int) string { return "BIGINT" }
func (r *testRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	values[0] = int64(42)
	return nil
}

type testTx struct{}

func (testTx) Commit() error   { return nil }
func (testTx) Rollback() error { return nil }

// Gin pools contexts; a canceled request's automatic rollback may finish after
// the context has been reassigned to another request.
type reusableContext struct{ context.Context }

func TestTransactionLoggingSnapshotsCorrelation(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"commit", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			if err := os.MkdirAll("/tmp/plutus", 0o750); err != nil {
				t.Fatal(err)
			}
			dir, err := os.MkdirTemp("/tmp/plutus", "transaction-logging-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			logs, err := logging.New(logging.Options{Directory: dir, MaxSizeMB: 1, SQLEnabled: true}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = logs.Close() }()
			ctx := &reusableContext{Context: logging.WithFields(context.Background(), slog.String("trace_id", "original-request"), slog.String("job_id", "original-job"))}
			conn := &loggingConn{Conn: &testConn{}, logger: logs.SQL}
			tx, err := conn.BeginTx(ctx, driver.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			ctx.Context = logging.WithFields(context.Background(), slog.String("trace_id", "next-request"))
			if operation == "commit" {
				err = tx.Commit()
			} else {
				err = tx.Rollback()
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "sql.log"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if strings.Count(text, "trace_id=original-request") != 2 || strings.Count(text, "job_id=original-job") != 2 || strings.Contains(text, "next-request") {
				t.Fatalf("transaction correlation changed after context reuse: %s", text)
			}
		})
	}
}

func TestDriverLoggingPreservesFallbackAndInterfaces(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	conn := &testConn{}
	db := sql.OpenDB(&loggingConnector{Connector: testConnector{conn: conn}, logger: logger})
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, "INSERT INTO example VALUES (?)", uint64(math.MaxUint64))
	if err != nil {
		t.Fatal(err)
	}
	if count, err := result.RowsAffected(); count != 1 || err != nil {
		t.Fatalf("result %d %v", count, err)
	}
	rows, err := db.QueryContext(ctx, "SELECT ?", int64(42))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.ColumnTypes()
	if err != nil {
		_ = rows.Close()
		t.Fatal(err)
	}
	if columns[0].DatabaseTypeName() != "BIGINT" {
		_ = rows.Close()
		t.Fatal("row metadata lost")
	}
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		if value != 42 {
			_ = rows.Close()
			t.Fatal("query result changed")
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		t.Fatal(err)
	}
	_ = rows.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
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
	if _, err := db.ExecContext(ctx, "fail"); err == nil {
		t.Fatal("execution failure changed")
	}
	if unexpected, err := db.PrepareContext(ctx, "bad prepare"); err == nil {
		defer func() { _ = unexpected.Close() }()
		t.Fatal("prepare failure changed")
	}
	if !conn.checked || !conn.reset || !conn.pinged || conn.prepares != 3 {
		t.Fatalf("interfaces not forwarded: %+v", conn)
	}
	text := output.String()
	if strings.Count(text, "sql execution") != 8 || strings.Count(text, "18446744073709551615") != 1 || strings.Contains(text, "skip") {
		t.Fatalf("duplicate/missing events: %s", text)
	}
	if !strings.Contains(text, "operation=prepare") || !strings.Contains(text, "level=ERROR") {
		t.Fatalf("missing failures: %s", text)
	}
	output.Reset()
	recordSQL(ctx, logger, "exec", "SELECT ?", namedValues([]driver.Value{struct{}{}}), time.Now(), nil)
	if !strings.Contains(output.String(), "render_error=") || !strings.Contains(output.String(), "args=") {
		t.Fatalf("missing render fallback: %s", output.String())
	}
}
