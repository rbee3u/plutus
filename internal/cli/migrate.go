package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Inspect or apply explicit MySQL migrations",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newMigrateStatusCommand(), newMigrateUpCommand())
	return cmd
}

func newMigrateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show applied and pending Goose migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, _, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			return mysql.Status(db, cmd.OutOrStdout())
		},
	}
}

func newMigrateUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending Goose migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, _, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if err := mysql.Up(db); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrations applied")
			return nil
		},
	}
}
