package logging

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testDirectory(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/plutus", 0o750); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("/tmp/plutus", "logging-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "logs")
}

func readLog(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name+".log"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRoutingLevelContextAndAppend(t *testing.T) {
	t.Parallel()
	dir := testDirectory(t)
	var console bytes.Buffer
	opts := Options{Directory: dir, Level: slog.LevelWarn, MaxSizeMB: 1, MaxBackups: 2, SQLEnabled: true}
	logs, err := New(opts, &console)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithFields(context.Background(), slog.String("trace_id", "old"), slog.String("cycle_id", "cycle"))
	ctx = WithFields(ctx, slog.String("trace_id", "trace"))
	logs.App.Info("filtered")
	logs.App.With("source", "test").WarnContext(ctx, "application", slog.String("trace_id", "explicit"))
	logs.Access.InfoContext(ctx, "request")
	logs.SQL.InfoContext(ctx, "query")
	_, _ = Writer(logs.App, slog.LevelError).Write([]byte("framework\nmultiline\n"))
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
	app, access, sql := readLog(t, dir, "app"), readLog(t, dir, "access"), readLog(t, dir, "sql")
	if strings.Contains(app, "filtered") || strings.Contains(app, "request") || strings.Contains(app, "query") {
		t.Fatalf("wrong app routing: %s", app)
	}
	if !strings.Contains(app, "trace_id=explicit") || strings.Count(app, "trace_id=") != 1 || !strings.Contains(app, "cycle_id=cycle") {
		t.Fatalf("app fields: %s", app)
	}
	if strings.Count(app, "\n") != 2 || !strings.Contains(app, `framework\nmultiline`) {
		t.Fatalf("multiline was not quoted: %q", app)
	}
	if !strings.Contains(access, "trace_id=trace") || !strings.Contains(sql, "trace_id=trace") || console.String() != app {
		t.Fatalf("routing/context: %q %q %q", access, sql, console.String())
	}
	logs, err = New(opts, &console)
	if err != nil {
		t.Fatal(err)
	}
	logs.App.Error("appended")
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readLog(t, dir, "app"), "application") || !strings.Contains(readLog(t, dir, "app"), "appended") {
		t.Fatal("existing file was not appended")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("directory permissions %o", info.Mode().Perm())
	}
	info, err = os.Stat(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions %o", info.Mode().Perm())
	}
}

func TestSQLDisabledAndStartupFailure(t *testing.T) {
	t.Parallel()
	dir := testDirectory(t)
	logs, err := New(Options{Directory: dir, MaxSizeMB: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if logs.SQL != nil {
		t.Fatal("disabled SQL logger exists")
	}
	if _, err := os.Stat(filepath.Join(dir, "sql.log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sql file exists: %v", err)
	}
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sql.log"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{Directory: dir, MaxSizeMB: 1, SQLEnabled: true}, io.Discard); err == nil {
		t.Fatal("accepted unusable SQL file")
	}
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	dir := testDirectory(t)
	logs, err := New(Options{Directory: dir, MaxSizeMB: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() { logs.Access.Info("request", slog.Int("index", i)) })
	}
	wg.Wait()
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(readLog(t, dir, "access"), "\n"); lines != 100 {
		t.Fatalf("got %d records", lines)
	}
}

func TestRotationCompressionAndRetention(t *testing.T) {
	t.Parallel()
	dir := testDirectory(t)
	logs, err := New(Options{Directory: dir, MaxSizeMB: 1, MaxBackups: 2, Compress: true}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logs.Close() }()
	for i := range 5 {
		logs.Access.Info("request", slog.Int("index", i), slog.String("padding", strings.Repeat("x", 600000)))
		time.Sleep(5 * time.Millisecond) // Keep backup names distinct at millisecond precision.
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, err := filepath.Glob(filepath.Join(dir, "access-*.log.gz"))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := filepath.Glob(filepath.Join(dir, "access-*.log"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 2 && len(plain) == 0 {
			for _, file := range files {
				f, err := os.Open(file)
				if err != nil {
					t.Fatal(err)
				}
				gz, err := gzip.NewReader(f)
				if err != nil {
					_ = f.Close()
					t.Fatal(err)
				}
				contents, err := io.ReadAll(gz)
				_ = gz.Close()
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(contents, []byte("index=2")) && !bytes.Contains(contents, []byte("index=3")) {
					t.Fatal("wrong backup retained")
				}
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("backup maintenance incomplete: gzip=%d plain=%d", len(files), len(plain))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if info, err := os.Stat(filepath.Join(dir, "access.log")); err != nil || info.Size() > 1024*1024 {
		t.Fatalf("active file size: %v %v", info, err)
	}
}

func TestBackupAgeMaintenance(t *testing.T) {
	t.Parallel()
	dir := testDirectory(t)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "access-"+time.Now().UTC().Add(-48*time.Hour).Format("2006-01-02T15-04-05.000")+".log")
	fresh := filepath.Join(dir, "access-"+time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15-04-05.000")+".log")
	for _, path := range []string{old, fresh} {
		if err := os.WriteFile(path, []byte("backup\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	logs, err := New(Options{Directory: dir, MaxSizeMB: 1, MaxAgeDays: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logs.Close() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(old); errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired backup not removed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh backup removed: %v", err)
	}
}

type failingFile struct {
	fail bool
	bytes.Buffer
}

func (f *failingFile) Write(p []byte) (int, error) {
	if f.fail {
		return 0, errors.New("disk failed")
	}
	return f.Buffer.Write(p)
}
func (*failingFile) Close() error { return nil }

func TestWriteFailureFallbackAndRecovery(t *testing.T) {
	t.Parallel()
	for _, mirror := range []bool{false, true} {
		var console bytes.Buffer
		file := &failingFile{fail: true}
		output := &sink{file: file, console: &console, mirror: mirror, name: "test"}
		if n, err := output.Write([]byte("record\n")); err != nil || n != 7 {
			t.Fatalf("fallback: %d %v", n, err)
		}
		if strings.Count(console.String(), "record\n") != 1 || !strings.Contains(console.String(), "disk failed") {
			t.Fatalf("fallback console %q", console.String())
		}
		file.fail = false
		if _, err := output.Write([]byte("recovered\n")); err != nil {
			t.Fatal(err)
		}
		if file.String() != "recovered\n" {
			t.Fatal("file did not recover")
		}
	}
	dir := testDirectory(t)
	var console bytes.Buffer
	logs, err := New(Options{Directory: dir, MaxSizeMB: 1}, &console)
	if err != nil {
		t.Fatal(err)
	}
	logs.Access.Info(strings.Repeat("x", 1024*1024+1))
	if !strings.Contains(console.String(), "exceeds maximum file size") {
		t.Fatal("oversize record did not fall back")
	}
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
}
