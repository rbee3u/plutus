package cli

import (
	"database/sql"
	"errors"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// loadProcessConfig reads the configuration selected by the persistent flags.
func loadProcessConfig(cmd *cobra.Command) (config.Config, error) {
	configPath, err := flagString(cmd, "config")
	if err != nil {
		return config.Config{}, err
	}
	envFile, err := flagString(cmd, "env-file")
	if err != nil {
		return config.Config{}, err
	}
	return config.Load(config.Options{Path: configPath, EnvFile: envFile})
}

func flagString(cmd *cobra.Command, name string) (string, error) {
	if f := cmd.Flags().Lookup(name); f != nil {
		return cmd.Flags().GetString(name)
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		return cmd.InheritedFlags().GetString(name)
	}
	return cmd.Root().PersistentFlags().GetString(name)
}

// openConfiguredDB opens the configured database for the command. The caller
// owns the returned handle.
func openConfiguredDB(proc *process) (*sql.DB, error) {
	if proc.cfg.Database.DSN == "" {
		return nil, errors.New("database DSN is required through PLUTUS_DSN or PLUTUS_DSN_FILE in the process environment or selected env file")
	}
	return mysql.Open(proc.cfg.Database.DSN, mysql.Loggers{App: proc.logs.App, SQL: proc.logs.SQL})
}
