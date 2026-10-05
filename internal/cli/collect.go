package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/domain/jobs"
	"github.com/rbee3u/plutus/internal/worker"
)

func newCollectCommand() *cobra.Command {
	var portfolioFlag string
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Run one Collection Cycle for a Portfolio synchronously and write its Valuation Point",
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

func runCollect(ctx context.Context, services *app.Services, portfolioID uuid.UUID, out interface{ Write([]byte) (int, error) }, logger *slog.Logger) error {
	job, created, err := services.Jobs.EnqueueCycle(ctx, portfolioID, jobs.TriggerManual, nil)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			return fmt.Errorf("portfolio %s not found", portfolioID)
		}
		return err
	}
	if !created {
		_, _ = fmt.Fprintf(out, "reusing active cycle %s (%s)\n", job.ID, job.Status)
	}
	loop := worker.New(&worker.Options{Jobs: services.Jobs, Collection: services.Collection, Valuation: services.Valuation, Logger: logger, Poll: 200 * time.Millisecond, Lease: 2 * time.Minute, ID: "collect-cli"})
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := loop.ProcessOnce(ctx); err != nil {
			return err
		}
		current, err := services.Jobs.Get(ctx, portfolioID, job.ID)
		if err != nil {
			return err
		}
		if !current.Active() {
			_, _ = fmt.Fprintf(out, "cycle %s %s", current.ID, current.Status)
			if current.LastDetail != nil {
				_, _ = fmt.Fprintf(out, ": %s", *current.LastDetail)
			}
			_, _ = fmt.Fprintln(out)
			if current.Status != jobs.StatusSucceeded {
				return fmt.Errorf("cycle %s ended with status %s", current.ID, current.Status)
			}
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
