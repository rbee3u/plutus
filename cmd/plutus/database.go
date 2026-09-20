package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/db/seeds"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
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
			if err := enqueueSeedRebuilds(cmd.Context(), db, cmd.OutOrStdout()); err != nil {
				return err
			}
			printSeedLogins(cmd.OutOrStdout(), profile)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", seeds.ProfileMinimal, "seed profile: minimal, edge-cases, or walkthrough")
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
			if err := enqueueSeedRebuilds(ctx, db, cmd.OutOrStdout()); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reset %q with %s source facts\n", name, profile)
			printSeedLogins(cmd.OutOrStdout(), profile)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", seeds.ProfileMinimal, "seed profile: minimal, edge-cases, or walkthrough")
	return cmd
}

func seedSQL(profile string) (string, error) {
	switch profile {
	case seeds.ProfileMinimal:
		return seeds.MinimalSQL()
	case seeds.ProfileEdgeCases:
		return seeds.EdgeCasesSQL()
	case seeds.ProfileWalkthrough:
		return seeds.WalkthroughSQL()
	default:
		return "", fmt.Errorf("unknown seed profile %q", profile)
	}
}

func enqueueSeedRebuilds(ctx context.Context, db *sql.DB, out io.Writer) error {
	rows, err := db.QueryContext(ctx, `SELECT id FROM portfolios ORDER BY created_at, id`)
	if err != nil {
		return fmt.Errorf("list portfolios: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []uuid.UUID
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return fmt.Errorf("scan portfolio id: %w", err)
		}
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return fmt.Errorf("portfolio id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list portfolios: %w", err)
	}

	svc := jobs.NewService(db)
	for _, id := range ids {
		job, err := svc.EnqueueRebuild(ctx, id, nil, "seed")
		if err != nil {
			return fmt.Errorf("enqueue rebuild for %s: %w", id, err)
		}
		_, _ = fmt.Fprintf(out, "enqueued %s job %s for portfolio %s\n", job.Kind, job.ID, job.PortfolioID)
	}
	return nil
}

func printSeedLogins(out io.Writer, profile string) {
	_, _ = fmt.Fprintf(out, "seeded %s source facts\n", profile)
	_, _ = fmt.Fprintln(out, "scratch logins: alice / Alice=123456 ; bob / Bob=123456")
}
