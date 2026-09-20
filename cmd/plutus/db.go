package main

import (
	"database/sql"
	"errors"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func openConfiguredDB() (*sql.DB, config.Config, error) {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return nil, config.Config{}, err
	}
	if cfg.Database.DSN == "" {
		return nil, config.Config{}, errors.New("database DSN is required in plutus.toml or PLUTUS_DATABASE_DSN / PLUTUS_DATABASE_DSN_FILE")
	}
	db, err := mysql.Open(cfg.Database.DSN)
	if err != nil {
		return nil, config.Config{}, err
	}
	return db, cfg, nil
}
