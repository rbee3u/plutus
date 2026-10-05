package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRetiredSeedProfilesAreRejectedBeforeRuntime(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"seed", "reset"} {
		t.Run(command, func(t *testing.T) {
			cmd := NewRootCommand("test")
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			// A nonexistent config would fail runtime loading if flag parsing let it run.
			cmd.SetArgs([]string{"--config", "/tmp/plutus/nonexistent-seed-config.toml", "db", command, "--profile", "walkthrough"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag: --profile") {
				t.Fatalf("retired flag must fail before database access, got %v", err)
			}
		})
	}
}
