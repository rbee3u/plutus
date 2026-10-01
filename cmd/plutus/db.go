package main

import (
	"database/sql"
	"errors"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

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

func openConfiguredDB(cmd *cobra.Command) (*sql.DB, config.Config, error) {
	cfg, err := loadProcessConfig(cmd)
	if err != nil {
		return nil, config.Config{}, err
	}
	if cfg.Database.DSN == "" {
		return nil, config.Config{}, errors.New("database DSN is required in the selected env file, PLUTUS_DSN / PLUTUS_DSN_FILE, or the selected TOML file")
	}
	db, err := mysql.Open(cfg.Database.DSN)
	if err != nil {
		return nil, config.Config{}, err
	}
	return db, cfg, nil
}
