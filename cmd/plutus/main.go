package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the CLI release string shown by --version.
const version = "dev"

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
		Long: "plutus collects normalized account observations, persists their history,\n" +
			"and presents balances, positions, valuations, and directional exposure.",
		Example:       "  plutus --help",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().String("config", "", "path to a non-secret TOML config file")
	cmd.PersistentFlags().String("env-file", "", "path to an env file used instead of walking config/local.env")
	cmd.AddCommand(newServeCommand(), newMigrateCommand(), newUserCommand(), newProjectionCommand(), newDatabaseCommand())
	return cmd
}
