package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProcessLoggingLifecycle(t *testing.T) {
	if err := os.MkdirAll("/tmp/plutus", 0o750); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("/tmp/plutus", "cli-logging-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Chdir(dir)
	if err := os.WriteFile("app.toml", []byte("[logging]\nsql_enabled = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("empty.env", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var console, output bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&output)
	root.SetErr(&console)
	root.SetArgs([]string{"--config", "app.toml", "--env-file", "empty.env", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("logs"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("help initialized logs")
	}
	root = NewRootCommand("test")
	root.SetOut(&output)
	root.SetErr(&console)
	var proc *process
	failure := errors.New("runtime test failure")
	cmd := &cobra.Command{Use: "probe", RunE: withProcess(func(_ *cobra.Command, received *process) error {
		proc = received
		received.logs.App.Info("before failure")
		return failure
	})}
	root.AddCommand(cmd)
	root.SetArgs([]string{"--config", "app.toml", "--env-file", "empty.env", "probe"})
	err = root.Execute()
	if !errors.Is(err, failure) {
		t.Fatalf("command error changed: %v", err)
	}
	var reported *ReportedError
	if !errors.As(err, &reported) {
		t.Fatal("terminal error not marked as reported")
	}
	if proc == nil {
		t.Fatal("handler did not receive the process")
	}
	if strings.Count(console.String(), "runtime test failure") != 1 {
		t.Fatalf("error duplication: %s", console.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "logs", "sql.log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("SQL disabled config lost")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "logs", "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("before failure")) || !bytes.Contains(raw, []byte("runtime test failure")) {
		t.Fatal("command app log missing")
	}
	proc.logs.App.Info("after close")
	after, err := os.ReadFile(filepath.Join(dir, "logs", "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, after) || !strings.Contains(console.String(), "after close") {
		t.Fatal("logger not closed on error")
	}
}
