package main

import (
	"fmt"
	"os"

	"github.com/rbee3u/plutus/internal/services"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:           "plutus",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		cmdAccount(),
	)

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bind(cmd *cobra.Command, svc services.Service) *cobra.Command {
	svc.Parse(cmd)

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := svc.Pre(cmd.Context()); err != nil {
			return fmt.Errorf("failed to call pre: %w", err)
		}

		return nil
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := svc.Run(cmd.Context()); err != nil {
			return fmt.Errorf("failed to call run: %w", err)
		}

		return nil
	}

	cmd.PostRunE = func(cmd *cobra.Command, _ []string) error {
		if err := svc.Post(cmd.Context()); err != nil {
			return fmt.Errorf("failed to call post: %w", err)
		}

		return nil
	}

	return cmd
}
