package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/logging"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

type runtimeKey struct{}

type commandRuntime struct {
	config config.Config
	logs   *logging.System
}

// withRuntime wraps each runtime leaf rather than relying on Cobra's post-run
// hooks, which are skipped when a command fails.
func withRuntime(cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		withRuntime(child)
	}
	if cmd.RunE == nil {
		return
	}
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) (result error) {
		cfg, err := loadProcessConfig(cmd)
		if err != nil {
			return err
		}
		var level slog.Level
		if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
			return err
		}
		logs, err := logging.New(logging.Options{
			Directory: cfg.Logging.Directory, Level: level, MaxSizeMB: cfg.Logging.MaxSizeMB,
			MaxBackups: cfg.Logging.MaxBackups, MaxAgeDays: cfg.Logging.MaxAgeDays,
			Compress: cfg.Logging.Compress, SQLEnabled: cfg.Logging.SQLEnabled,
		}, cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		previous := cmd.Context()
		ctx := context.WithValue(previous, runtimeKey{}, &commandRuntime{config: cfg, logs: logs})
		cmd.SetContext(ctx)
		mysql.SetMigrationLogger(logs.App)
		defer func() {
			if result != nil {
				logs.App.ErrorContext(ctx, "command failed", slog.String("command", cmd.CommandPath()), slog.Any("err", result))
				result = &reportedError{error: result}
			}
			mysql.SetMigrationLogger(nil)
			if err := logs.Close(); err != nil {
				logs.App.Error("close logs failed", slog.Any("err", err))
				result = &reportedError{error: errors.Join(result, err)}
			}
			cmd.SetContext(previous)
		}()
		return run(cmd, args)
	}
}

func runtimeFrom(cmd *cobra.Command) *commandRuntime {
	runtime, _ := cmd.Context().Value(runtimeKey{}).(*commandRuntime)
	return runtime
}

// reportedError prevents main from printing an error already mirrored by app.
type reportedError struct{ error }

func (e *reportedError) Unwrap() error { return e.error }
