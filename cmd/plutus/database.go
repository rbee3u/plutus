package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

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
	seed := seeds.Build()
	if err := mysql.ExecSQLStatements(ctx, db, seed.SQL); err != nil {
		return err
	}
	if err := activateSeedIdentities(ctx, db, cfg, seed.Live, out); err != nil {
		return err
	}
	svc := valuation.NewService(db)
	for _, portfolioID := range seed.Portfolios {
		count, err := svc.Rebuild(ctx, portfolioID)
		if err != nil {
			return fmt.Errorf("rebuild valuation points for %s: %w", portfolioID, err)
		}
		_, _ = fmt.Fprintf(out, "portfolio %s: %d valuation points\n", portfolioID, count)
	}
	_, _ = fmt.Fprintln(out, "loaded development seed")
	_, _ = fmt.Fprintln(out, "scratch logins: alice / Alice=123456 ; bob / Bob=123456")
	return nil
}

// activateSeedIdentities seals deterministic credentials and enables each live Identity through the
// configured adapters, so the scheduler collects them like User-configured Connections.
func activateSeedIdentities(ctx context.Context, db *sql.DB, cfg *config.Config, live []seeds.LiveIdentity, out io.Writer) error {
	if len(live) == 0 {
		return nil
	}
	if !cfg.Crypto.HasMasterKey() {
		_, _ = fmt.Fprintln(out, "no master key configured: seeded identities stay unconfigured")
		return nil
	}
	registry := buildAdapters(cfg)
	identities, err := identity.NewService(db, identity.Options{MasterKey: cfg.Crypto.MasterKey, Adapters: registry, ValidateTimeout: cfg.Collection.ConnectionValidationTimeout})
	if err != nil {
		return err
	}
	for _, item := range live {
		current, err := identities.Get(ctx, item.PortfolioID, item.IdentityID)
		if err != nil {
			return fmt.Errorf("load seeded identity %s: %w", item.IdentityID, err)
		}
		stored, err := identities.PutCredentials(ctx, item.PortfolioID, item.IdentityID, current.Version, identity.ActionInput{}, item.Credentials)
		if err != nil {
			return fmt.Errorf("store credentials for %s: %w", current.DisplayName, err)
		}
		enabled, _, err := identities.Enable(ctx, item.PortfolioID, item.IdentityID, stored.Version, identity.ActionInput{})
		switch {
		case errors.Is(err, identity.ErrValidationFailed):
			_, _ = fmt.Fprintf(out, "identity %q: validation failed, connection is %s\n", current.DisplayName, enabled.Connection.Status)
		case errors.Is(err, identity.ErrAdapterUnavailable):
			_, _ = fmt.Fprintf(out, "identity %q: no adapter for %s (set [collection] fake_adapter = true for offline lanes); credentials stored, connection disabled\n", current.DisplayName, current.Platform)
		case err != nil:
			return fmt.Errorf("enable %s: %w", current.DisplayName, err)
		default:
			_, _ = fmt.Fprintf(out, "identity %q: connection enabled\n", current.DisplayName)
		}
	}
	return nil
}
