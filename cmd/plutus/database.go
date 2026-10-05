package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/db/seeds"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/valuation"
)

func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Load deterministic development data or reset a database",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newDBSeedCommand(), newDBResetCommand())
	return cmd
}

func newDBSeedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Insert development data into an empty database and rebuild its Valuation Points",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, cfg, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			return seedDatabase(cmd.Context(), db, &cfg, cmd.OutOrStdout())
		},
	}
	return cmd
}

func newDBResetCommand() *cobra.Command {
	var seed bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop every table, migrate, and optionally load development data",
		Long: "db reset drops every table in the configured database and reapplies the migrations.\n" +
			"With --seed it then inserts deterministic development data. Without --seed\n" +
			"the database is left empty and migrated, which is how the test environment is wiped before\n" +
			"real read-only keys are configured through the UI. It refuses production-like database names.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, cfg, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if !cfg.Database.AllowReset {
				return errors.New("db reset requires [database].allow_reset = true in the selected config file")
			}
			ctx := cmd.Context()
			if seed {
				if err := validateSeedConfig(&cfg); err != nil {
					return err
				}
			}
			name, err := mysql.GuardDevReset(ctx, db)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resetting database %q\n", name)
			if err := mysql.DropAllTables(ctx, db); err != nil {
				return err
			}
			if err := mysql.Up(db); err != nil {
				return err
			}
			if !seed {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrated an empty database; create a user with `plutus user create`")
				return nil
			}
			return seedDatabase(ctx, db, &cfg, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&seed, "seed", false, "load development data after migrating (omit to leave the database empty)")
	return cmd
}

// seedDatabase runs the seed SQL, activates live Identities, and rebuilds every Valuation Point.
func seedDatabase(ctx context.Context, db *sql.DB, cfg *config.Config, out io.Writer) error {
	if err := validateSeedConfig(cfg); err != nil {
		return err
	}
	started := time.Now()
	seed, err := seeds.Build()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "generated source facts in %s\n", time.Since(started).Round(time.Millisecond))
	stage := time.Now()
	if err := seeds.Apply(ctx, db, seed); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "inserted source facts in %s\n", time.Since(stage).Round(time.Millisecond))
	stage = time.Now()
	if err := activateSeedIdentities(ctx, db, cfg, seed.Live, out); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "initialized connections in %s\n", time.Since(stage).Round(time.Millisecond))
	svc := valuation.NewService(db)
	for _, portfolioID := range seed.Portfolios {
		stage = time.Now()
		count, err := svc.Rebuild(ctx, portfolioID)
		if err != nil {
			return fmt.Errorf("rebuild valuation points for %s: %w", portfolioID, err)
		}
		_, _ = fmt.Fprintf(out, "portfolio %s: %d valuation points in %s\n", portfolioID, count, time.Since(stage).Round(time.Millisecond))
	}
	_, _ = fmt.Fprintf(out, "loaded development seed in %s\n", time.Since(started).Round(time.Millisecond))
	_, _ = fmt.Fprintln(out, "scratch login: alice / Alice=123456")
	return nil
}

// activateSeedIdentities seals deterministic credentials and enables each live Identity through the
// configured adapters, so the scheduler collects them like User-configured Connections.
func activateSeedIdentities(ctx context.Context, db *sql.DB, cfg *config.Config, live []seeds.LiveIdentity, out io.Writer) error {
	identities, err := identity.NewService(db, identity.Options{MasterKey: cfg.Crypto.MasterKey, Adapters: buildAdapters(cfg), ValidateTimeout: cfg.Collection.ConnectionValidationTimeout})
	if err != nil {
		return err
	}
	return seeds.ActivateConnections(ctx, identities, live, out)
}

func validateSeedConfig(cfg *config.Config) error {
	if !cfg.Collection.FakeAdapter || !cfg.Crypto.HasMasterKey() {
		return errors.New("development seeds require fake_adapter = true and a configured master key")
	}
	return nil
}
