package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	httpserver "github.com/rbee3u/plutus/internal/transport/http"
)

func newServeCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API and embedded web process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, configPath)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "path to a non-secret TOML config file")
	return cmd
}

func runServe(cmd *cobra.Command, configPath string) error {
	logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(config.Options{Path: configPath})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ready, cleanup := newReadyChecker(cfg.Database.DSN)
	defer cleanup()

	engine := httpserver.Engine(logger, ready, cfg.Auth.RegistrationEnabled)
	return httpserver.Run(ctx, &cfg, logger, engine)
}

func newReadyChecker(dsn string) (httpserver.ReadyChecker, func()) {
	noop := func() {}
	if dsn == "" {
		return httpserver.StaticReady{Err: errors.New("database DSN is not configured")}, noop
	}
	db, err := mysql.Open(dsn)
	if err != nil {
		return httpserver.StaticReady{Err: err}, noop
	}
	return httpserver.FuncReady(func(callCtx context.Context) error {
		return mysql.CheckReady(callCtx, db)
	}), func() { _ = db.Close() }
}
