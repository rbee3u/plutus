package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/offline"
)

func TestMain(m *testing.M) {
	offline.Install()
	os.Exit(m.Run())
}

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "plutus") {
		t.Fatalf("help output %q does not mention plutus", out)
	}
	if strings.Contains(out, "demo") {
		t.Fatalf("help output still advertises demo: %q", out)
	}
	if !strings.Contains(out, "serve") {
		t.Fatalf("help output does not mention serve: %q", out)
	}
	if !strings.Contains(out, "migrate") {
		t.Fatalf("help output does not mention migrate: %q", out)
	}
	if !strings.Contains(out, "user") {
		t.Fatalf("help output does not mention user: %q", out)
	}
	if !strings.Contains(out, "collect") {
		t.Fatalf("help output does not mention collect: %q", out)
	}
	if !strings.Contains(out, "db") {
		t.Fatalf("help output does not mention db: %q", out)
	}
	if !strings.Contains(out, "config") {
		t.Fatalf("help output does not mention --config: %q", out)
	}
	if !strings.Contains(out, "env-file") {
		t.Fatalf("help output does not mention --env-file: %q", out)
	}
}
