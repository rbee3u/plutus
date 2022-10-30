package main

import (
	"github.com/rbee3u/plutus/internal/services"
	"github.com/spf13/cobra"
)

func cmdAccount() *cobra.Command {
	cmd := &cobra.Command{Use: "account", Args: cobra.NoArgs}

	cmd.AddCommand(
		cmdAccountList(),
	)

	return cmd
}

func cmdAccountList() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Args: cobra.NoArgs}

	bind(cmd, &services.AccountListService{})

	return cmd
}
