package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/db/seeds"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Load deterministic source facts or reset a development database",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newDBSeedCommand(), newDBResetCommand())
	return cmd
}

func newDBSeedCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Insert a deterministic source-fact profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sqlText, err := seedSQL(profile)
			if err != nil {
				return err
			}
			db, _, err := openConfiguredDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if err := mysql.ExecSQLStatements(cmd.Context(), db, sqlText); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "seeded %s source facts\n", profile)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", seeds.ProfileMinimal, "seed profile: minimal or edge-cases")
	return cmd
}

func newDBResetCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop development tables, migrate, and seed source facts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sqlText, err := seedSQL(profile)
			if err != nil {
				return err
			}
			db, cfg, err := openConfiguredDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if !cfg.Database.AllowReset {
				return errors.New("db reset requires [database].allow_reset = true in the local plutus.toml")
			}
			ctx := cmd.Context()
			name, err := mysql.GuardDevReset(ctx, db)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resetting development database %q\n", name)
			if err := mysql.DropAllTables(ctx, db); err != nil {
				return err
			}
			if err := mysql.Up(db); err != nil {
				return err
			}
			if err := mysql.ExecSQLStatements(ctx, db, sqlText); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reset %q with %s source facts\n", name, profile)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", seeds.ProfileMinimal, "seed profile: minimal or edge-cases")
	return cmd
}

func seedSQL(profile string) (string, error) {
	switch profile {
	case seeds.ProfileMinimal:
		return seeds.MinimalSQL()
	case seeds.ProfileEdgeCases:
		return seeds.EdgeCasesSQL()
	default:
		return "", fmt.Errorf("unknown seed profile %q", profile)
	}
}
