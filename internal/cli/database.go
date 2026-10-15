package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/seeds"
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
		Short: "Insert development data and initialize its valuation points",
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
		Long: "The db reset command drops every table in the configured database and reapplies the migrations.\n" +
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resetting database %q\n", name)
			if err := mysql.DropAllTables(ctx, db); err != nil {
				return err
			}
			if err := mysql.Up(db); err != nil {
				return err
			}
			if !seed {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Migrated an empty database; create a user with `plutus user create`")
				return nil
			}
			return seedDatabase(ctx, db, &cfg, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&seed, "seed", false, "Load development data after migrating (omit to leave the database empty)")
	return cmd
}

// seedDatabase initializes source facts and valuation points, then activates live identities.
func seedDatabase(ctx context.Context, db *sql.DB, cfg *config.Config, out io.Writer) error {
	if err := validateSeedConfig(cfg); err != nil {
		return err
	}
	started := time.Now()
	seed, err := seeds.Build()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Generated source facts in %s\n", time.Since(started).Round(time.Millisecond))
	stage := time.Now()
	if err := seeds.Apply(ctx, db, seed); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Initialized source facts and valuation points in %s\n", time.Since(stage).Round(time.Millisecond))
	stage = time.Now()
	if err := activateSeedIdentities(ctx, db, cfg, seed.Live, out); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Initialized connections in %s\n", time.Since(stage).Round(time.Millisecond))
	_, _ = fmt.Fprintf(out, "Loaded development seed in %s\n", time.Since(started).Round(time.Millisecond))
	_, _ = fmt.Fprintln(out, "Scratch login: alice / Alice=123456")
	return nil
}

// activateSeedIdentities seals deterministic credentials and enables each live identity through the
// configured adapters, so the scheduler collects them like user-configured connections.
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
