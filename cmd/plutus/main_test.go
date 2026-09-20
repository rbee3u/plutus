package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
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
}
