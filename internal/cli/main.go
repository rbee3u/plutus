package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand builds the plutus command tree. version is the release
// string shown by --version; cmd/plutus supplies it from -ldflags.
func NewRootCommand(version string) *cobra.Command {
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
	for _, runtimeCmd := range []*cobra.Command{newServeCommand(), newMigrateCommand(), newUserCommand(), newCollectCommand(), newDatabaseCommand()} {
		withRuntime(runtimeCmd)
		cmd.AddCommand(runtimeCmd)
	}
	cmd.AddCommand(newAdapterCommand())
	return cmd
}
