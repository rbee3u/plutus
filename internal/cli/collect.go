package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/worker"
)

func newCollectCommand() *cobra.Command {
	var portfolioFlag string
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Run one collection cycle for a portfolio synchronously and write its valuation point",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			portfolioID, err := uuid.Parse(portfolioFlag)
			if err != nil {
				return fmt.Errorf("--portfolio must be a UUID: %w", err)
			}
			db, cfg, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			logger := runtimeFrom(cmd).logs.App
			services, err := app.New(db, &cfg, buildAdapters(&cfg), logger)
			if err != nil {
				return err
			}
			return runCollect(cmd.Context(), services, portfolioID, cmd.OutOrStdout(), logger)
		},
	}
	cmd.Flags().StringVar(&portfolioFlag, "portfolio", "", "Portfolio UUID to collect")
	_ = cmd.MarkFlagRequired("portfolio")
	return cmd
}

func runCollect(ctx context.Context, services *app.Services, portfolioID uuid.UUID, out io.Writer, logger *slog.Logger) error {
	cycle, created, err := services.Cycles.Enqueue(ctx, portfolioID, cycles.TriggerUser)
	if err != nil {
		if errors.Is(err, cycles.ErrNotFound) {
			return fmt.Errorf("portfolio %s not found", portfolioID)
		}
		return err
	}
	if !created {
		_, _ = fmt.Fprintf(out, "Reusing active cycle %s (%s)\n", cycle.ID, cycle.Status)
	}
	loop := worker.New(&worker.Options{Cycles: services.Cycles, Collection: services.Collection, Logger: logger, Poll: 200 * time.Millisecond, Lease: 2 * time.Minute, ID: "collect-cli"})
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := loop.ProcessOnce(ctx); err != nil {
			return err
		}
		current, err := services.Cycles.Get(ctx, portfolioID, cycle.ID)
		if err != nil {
			return err
		}
		if !current.Active() {
			_, _ = fmt.Fprintf(out, "Cycle %s %s", current.ID, current.Status)
			if current.Status != cycles.StatusSucceeded {
				if current.ErrorDetail != nil {
					_, _ = fmt.Fprintf(out, ": %s", *current.ErrorDetail)
				}
				_, _ = fmt.Fprintln(out)
				return fmt.Errorf("cycle %s ended with status %s", current.ID, current.Status)
			}
			point, err := services.Valuation.PointByCycle(ctx, portfolioID, current.ID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, ": total %s USD, %d accounts\n", point.TotalUSD, point.AccountCount)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return errors.New("the cycle did not finish within 10 minutes")
}
