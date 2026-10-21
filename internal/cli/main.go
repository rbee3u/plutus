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
		Long: "The plutus command collects read-only account reports from crypto platforms, writes one\n" +
			"valuation point per collection cycle, and presents holdings, history, and value changes.",
		Example:       "  plutus --help",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().String("config", "", "Non-secret TOML file (default config/local.toml; adapter record uses config/record.toml)")
	cmd.PersistentFlags().String("env-file", "", "Runtime secrets env file instead of walking config/local.env (ignored by adapter record)")
	cmd.AddCommand(newServeCommand(), newMigrateCommand(), newUserCommand(), newCollectCommand(), newDatabaseCommand(), newAdapterCommand())
	return cmd
}
