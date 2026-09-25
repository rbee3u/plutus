package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/binance"
	"github.com/rbee3u/plutus/internal/adapters/bybit"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/kraken"
	"github.com/rbee3u/plutus/internal/adapters/okx"
	"github.com/rbee3u/plutus/internal/config"
)

// recorder is implemented by the live adapters that can record test fixtures (ADR 0023). Only
// this command uses it; serve, the worker, and collect never see a raw Platform body.
type recorder interface {
	Record(ctx context.Context, creds adapters.Credentials) ([]fixture.Recording, error)
}

// recorders lists the Platforms whose adapter can record, and whether their keys carry a passphrase.
var recorders = map[adapters.Platform]struct {
	adapter        func() recorder
	needPassphrase bool
}{
	adapters.PlatformKraken:  {adapter: func() recorder { return kraken.New(nil) }},
	adapters.PlatformBybit:   {adapter: func() recorder { return bybit.New(nil) }},
	adapters.PlatformBinance: {adapter: func() recorder { return binance.New(nil) }},
	adapters.PlatformOKX:     {adapter: func() recorder { return okx.New(nil) }, needPassphrase: true},
}

func recordablePlatforms() string {
	names := make([]string, 0, len(recorders))
	for _, platform := range adapters.OfferedPlatforms() {
		if _, ok := recorders[platform]; ok {
			names = append(names, string(platform))
		}
	}
	return strings.Join(names, ", ")
}

func newAdapterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapter",
		Short: "Developer tools for Platform adapters",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newAdapterRecordCommand())
	return cmd
}

func newAdapterRecordCommand() *cobra.Command {
	var platformFlag, outFlag string
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record trimmed Platform responses as test fixtures into tmp/ (read-only keys from config/gotest.env)",
		Long: "record calls every read-only endpoint the Platform adapter uses once with the test environment's\n" +
			"read-only key set (PLUTUS_GOTEST_<PLATFORM>_APIKEY / _SECRET, and _PASSPHRASE for OKX, from the\n" +
			"process environment or config/gotest.env), trims large market-data lists, replaces the account UID,\n" +
			"and writes one JSON fixture per endpoint into --out/<platform>/. Review every file before moving it\n" +
			"into internal/adapters/<platform>/testdata/. It needs no --config, database, or master key.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := checkRecordingDirectory(outFlag); err != nil {
				return err
			}
			platform := adapters.Platform(strings.ToLower(platformFlag))
			spec, ok := recorders[platform]
			if !ok {
				return fmt.Errorf("--platform %q has no live adapter to record; supported: %s", platformFlag, recordablePlatforms())
			}
			creds, err := recorderCredentials(platform, spec.needPassphrase)
			if err != nil {
				return err
			}
			recordings, err := spec.adapter().Record(cmd.Context(), creds)
			if err != nil {
				return fmt.Errorf("record %s: %w", platform, err)
			}
			dir := filepath.Join(outFlag, string(platform))
			recordedAt := time.Now().UTC().Truncate(time.Second)
			for i := range recordings {
				r := &recordings[i]
				path, err := fixture.Write(dir, r.Name, &fixture.File{
					Fixture: fixture.Header{
						Platform: string(platform), Endpoint: r.Endpoint, AccountStructure: r.AccountStructure, MarginMode: r.MarginMode,
						Scenario: r.Scenario, RecordedAt: &recordedAt, Trimmed: r.Trimmed, Redacted: r.Redacted,
					},
					Status: r.Status,
					Body:   r.Body,
				})
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (HTTP %d)\n", path, r.Status)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Review every file for IDs and key echoes before moving it into internal/adapters/%s/testdata/.\n", platform)
			return nil
		},
	}
	cmd.Flags().StringVar(&platformFlag, "platform", "", "Platform to record ("+recordablePlatforms()+")")
	cmd.Flags().StringVar(&outFlag, "out", filepath.Join("tmp", "fixtures"), "output directory; keep it under gitignored tmp/")
	_ = cmd.MarkFlagRequired("platform")
	return cmd
}

// checkRecordingDirectory confines all raw fixture writes to the repository's real tmp tree.
func checkRecordingDirectory(path string) error {
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	out, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	tmp := filepath.Join(root, "tmp")
	inside := func(base, target string) bool {
		rel, err := filepath.Rel(base, target)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if !inside(tmp, out) {
		return errors.New("fixture output must be inside the repository's gitignored tmp directory")
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	realOut, err := filepath.EvalSymlinks(out)
	if err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if !inside(filepath.Join(realRoot, "tmp"), realOut) {
		return errors.New("fixture output must not escape tmp through a symlink")
	}
	return nil
}

// recorderCredentials reads the test environment's read-only key set. The recorder is the only
// part of the binary that reads config/gotest.env; serve, the worker, and collect never do.
func recorderCredentials(platform adapters.Platform, needPassphrase bool) (adapters.Credentials, error) {
	prefix := "PLUTUS_GOTEST_" + strings.ToUpper(string(platform)) + "_"
	var creds adapters.Credentials
	var missing []string
	type variable struct {
		suffix string
		dst    *string
	}
	fields := []variable{{"APIKEY", &creds.APIKey}, {"SECRET", &creds.APISecret}}
	if platform == adapters.PlatformKraken {
		creds.Futures = &adapters.KeyPair{}
		fields = []variable{{"SPOT_APIKEY", &creds.APIKey}, {"SPOT_SECRET", &creds.APISecret}, {"FUTURES_APIKEY", &creds.Futures.APIKey}, {"FUTURES_SECRET", &creds.Futures.APISecret}}
	}
	if needPassphrase {
		fields = append(fields, variable{"PASSPHRASE", &creds.Passphrase})
	}
	for _, field := range fields {
		value, err := config.Lookup(prefix + field.suffix)
		if err != nil {
			return adapters.Credentials{}, err
		}
		*field.dst = strings.TrimSpace(value)
		if *field.dst == "" {
			missing = append(missing, prefix+field.suffix)
		}
	}
	if len(missing) > 0 {
		return adapters.Credentials{}, fmt.Errorf("%s not set in the environment or config/gotest.env (copy config/gotest-example.env)", strings.Join(missing, ", "))
	}
	return creds, nil
}
