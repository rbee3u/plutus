package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the CLI release string shown by --version. `make build` and `make dist` set it
// through -ldflags "-X main.version=..." from `git describe`; a plain `go run` reports "dev".
var version = "dev"

func main() {
	if err := newRootCommand().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plutus",
		Short: "Personal read-only portfolio observation",
		Long: "plutus collects read-only Account Reports from crypto Platforms, writes one Valuation Point\n" +
			"per Collection Cycle, and presents holdings, history, and value changes.",
		Example:       "  plutus --help",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().String("config", "", "path to a non-secret TOML config file")
	cmd.PersistentFlags().String("env-file", "", "path to an env file used instead of walking config/local.env")
	cmd.AddCommand(newServeCommand(), newMigrateCommand(), newUserCommand(), newCollectCommand(), newDatabaseCommand(), newAdapterCommand())
	return cmd
}
