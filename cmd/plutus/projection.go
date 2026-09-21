package main

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/jobs"
)

func newProjectionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projection",
		Short: "Enqueue projection rebuild work",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newProjectionRebuildCommand())
	return cmd
}

func newProjectionRebuildCommand() *cobra.Command {
	var portfolioID string
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Enqueue a full projection rebuild",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, err := uuid.Parse(portfolioID)
			if err != nil {
				return fmt.Errorf("portfolio: %w", err)
			}
			db, _, err := openConfiguredDB(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			job, err := jobs.NewService(db).EnqueueRebuild(cmd.Context(), id, nil, "cli_rebuild")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "enqueued %s job %s for portfolio %s\n", job.Kind, job.ID, job.PortfolioID)
			return nil
		},
	}
	cmd.Flags().StringVar(&portfolioID, "portfolio", "", "portfolio UUID")
	_ = cmd.MarkFlagRequired("portfolio")
	return cmd
}
