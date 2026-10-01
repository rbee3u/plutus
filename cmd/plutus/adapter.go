package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/binance"
	"github.com/rbee3u/plutus/internal/adapters/bybit"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/kraken"
	"github.com/rbee3u/plutus/internal/adapters/okx"
	"github.com/rbee3u/plutus/internal/adapters/recording"
	"github.com/rbee3u/plutus/internal/config"
)

// Raw responses are available only to this explicit developer command.
type recorder interface {
	RecordRequest(context.Context, adapters.Credentials, fixture.Request, ...recording.Options) (*fixture.File, error)
}

var recorders = map[adapters.Platform]struct {
	adapter        func() recorder
	needPassphrase bool
	requests       func() recording.Catalogue
}{
	adapters.PlatformBinance: {adapter: func() recorder { return binance.New(nil) }, requests: binance.RecordingRequests},
	adapters.PlatformOKX:     {adapter: func() recorder { return okx.New(nil) }, needPassphrase: true, requests: okx.RecordingRequests},
	adapters.PlatformBybit:   {adapter: func() recorder { return bybit.New(nil) }, requests: bybit.RecordingRequests},
	adapters.PlatformKraken:  {adapter: func() recorder { return kraken.New(nil) }, requests: kraken.RecordingRequests},
}

func recordablePlatforms() string {
	var names []string
	for _, platform := range adapters.OfferedPlatforms() {
		if _, ok := recorders[platform]; ok {
			names = append(names, string(platform))
		}
	}
	return strings.Join(names, ", ")
}

func newAdapterCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "adapter", Short: "Developer tools for Platform adapters", Args: cobra.NoArgs}
	cmd.AddCommand(newAdapterRecordCommand())
	return cmd
}

func newAdapterRecordCommand() *cobra.Command {
	var platformFlag, outFlag, variant string
	var only, query, form, keepSymbols []string
	var list, dryRun bool
	cmd := &cobra.Command{Use: "record", Short: "Record one explicit read-only interface response into tmp", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		platform := adapters.Platform(strings.ToLower(platformFlag))
		spec, ok := recorders[platform]
		if !ok {
			return fmt.Errorf("unsupported recording Platform %q; choose %s", platformFlag, recordablePlatforms())
		}
		catalogue := spec.requests()
		if list {
			var names []string
			for name := range catalogue {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				ep := catalogue[name]
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s; required parameters: %s\n", name, ep.Request.Key(), strings.Join(ep.Required, ", "))
			}
			return nil
		}
		if len(only) != 1 {
			return errors.New("--only must select exactly one interface; use --list-requests")
		}
		if !fixture.ValidVariant(variant) {
			return errors.New("--variant must contain only lowercase letters, digits and underscores")
		}
		name := strings.TrimSuffix(only[0], ".json")
		request, err := recording.SelectRequest(catalogue, name, query, form)
		if err != nil {
			return err
		}
		filename := fixture.Name(request.Path, variant) + ".json"
		if dryRun {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\nOutput: %s\nOne explicit request; no keys read, preflight, retry, redirect or additional page.\n", request.Key(), filename)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Local market selection: keep symbols %v; representative product types.\n", keepSymbols)
			return nil
		}
		if err := checkRecordingDirectory(outFlag); err != nil {
			return err
		}
		creds, err := recorderCredentials(platform, spec.needPassphrase, catalogue, request)
		if err != nil {
			return err
		}
		file, captureErr := spec.adapter().RecordRequest(cmd.Context(), creds, request, recording.Options{KeepSymbols: keepSymbols})
		if file != nil {
			parent := filepath.Join(outFlag, string(platform))
			if err := os.MkdirAll(parent, 0o750); err != nil {
				return err
			}
			dir, err := os.MkdirTemp(parent, time.Now().UTC().Format("20060102T150405Z")+"-")
			if err != nil {
				return err
			}
			path, err := fixture.Write(dir, strings.TrimSuffix(filename, ".json"), file)
			if err != nil {
				return fmt.Errorf("save response: %w", err)
			}
			status := "WebSocket snapshot"
			if file.Request.Method != "WS" {
				status = fmt.Sprintf("HTTP %d", file.Status)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved: %s\nResponse: %s; platform success: %t; empty: %t\nReview before promotion to evidence; synthetic tests are independent and business mapping has not been accepted.\n", path, status, captureErr == nil, recording.Empty(file))
		}
		if captureErr != nil {
			return captureErr
		}
		if file == nil {
			return errors.New("request produced no safe response")
		}
		return nil
	}}
	cmd.Flags().StringVar(&platformFlag, "platform", "", "Platform to record")
	cmd.Flags().StringSliceVar(&only, "only", nil, "one read-only interface identifier")
	cmd.Flags().StringVar(&variant, "variant", "", "semantic filename suffix; does not change account configuration")
	cmd.Flags().StringArrayVar(&query, "query", nil, "business query key=value; repeat for multiple values")
	cmd.Flags().StringArrayVar(&form, "form", nil, "business form key=value; repeat for multiple values")
	cmd.Flags().BoolVar(&list, "list-requests", false, "list read-only interfaces and required parameters without keys or network")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the exact request and filename without keys or network")
	cmd.Flags().StringArrayVar(&keepSymbols, "keep-symbol", nil, "retain a prepared instrument in local market trimming; never sent to the exchange")
	cmd.Flags().StringVar(&outFlag, "out", "tmp/evidence", "output parent inside gitignored tmp")
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

// recorderCredentials reads the recording key set from config/record.env or PLUTUS_RECORD_*.
// It never reads gotest or runtime env files; public requests need no credentials.
func recorderCredentials(platform adapters.Platform, needPassphrase bool, catalogue recording.Catalogue, request fixture.Request) (adapters.Credentials, error) {
	prefix := "PLUTUS_RECORD_" + strings.ToUpper(string(platform)) + "_"
	var creds adapters.Credentials
	spot, futures := false, false
	for _, request := range []fixture.Request{request} {
		ep, ok := catalogue.Find(request)
		if !ok || !ep.Signed {
			continue
		}
		if platform == adapters.PlatformKraken && request.Service != "spot" {
			futures = true
		} else {
			spot = true
		}
	}
	if !spot && !futures {
		return creds, nil
	}
	var missing []string
	type variable struct {
		suffix string
		dst    *string
	}
	var fields []variable
	if spot {
		fields = []variable{{"APIKEY", &creds.APIKey}, {"SECRET", &creds.APISecret}}
	}
	if platform == adapters.PlatformKraken {
		fields = nil
		if spot {
			fields = append(fields, variable{"SPOT_APIKEY", &creds.APIKey}, variable{"SPOT_SECRET", &creds.APISecret})
		}
		if futures {
			creds.Futures = &adapters.KeyPair{}
			fields = append(fields, variable{"FUTURES_APIKEY", &creds.Futures.APIKey}, variable{"FUTURES_SECRET", &creds.Futures.APISecret})
		}
	}
	if needPassphrase {
		fields = append(fields, variable{"PASSPHRASE", &creds.Passphrase})
	}
	for _, field := range fields {
		value, err := config.LookupRecording(prefix + field.suffix)
		if err != nil {
			return adapters.Credentials{}, err
		}
		*field.dst = strings.TrimSpace(value)
		if *field.dst == "" {
			missing = append(missing, prefix+field.suffix)
		}
	}
	if len(missing) > 0 {
		return adapters.Credentials{}, fmt.Errorf("%s not set in the environment or config/record.env (copy config/record-example.env)", strings.Join(missing, ", "))
	}
	return creds, nil
}
