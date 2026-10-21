package cli

import (
	"errors"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/logging"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// process is the per-command runtime: the configuration selected by the
// persistent flags and the log sinks opened for this command. Leaf commands
// receive it as a parameter instead of looking it up.
type process struct {
	cfg  config.Config
	logs *logging.System
}

// withProcess wraps a leaf handler with the process lifecycle. It loads
// configuration once, opens the log sinks, and closes them after the handler
// returns. Cobra's post-run hooks are skipped when a command fails, so the
// lifecycle lives in the handler rather than in those hooks.
func withProcess(run func(*cobra.Command, *process) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) (result error) {
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
		mysql.SetMigrationLogger(logs.App)
		defer func() {
			if result != nil {
				logs.App.ErrorContext(cmd.Context(), "command failed", slog.String("command", cmd.CommandPath()), slog.Any("err", result))
				result = &ReportedError{error: result}
			}
			mysql.SetMigrationLogger(nil)
			if err := logs.Close(); err != nil {
				logs.App.Error("close logs failed", slog.Any("err", err))
				result = &ReportedError{error: errors.Join(result, err)}
			}
		}()
		return run(cmd, &process{cfg: cfg, logs: logs})
	}
}

// ReportedError prevents main from printing an error already mirrored by app.
type ReportedError struct{ error }

func (e *ReportedError) Unwrap() error { return e.error }
