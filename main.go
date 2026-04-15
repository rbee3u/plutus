package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/cmd/demo"
)

// version is the CLI release string shown by --version.
const version = "dev"

// main builds and executes the root CLI; it prints errors to stderr and exits with status 1 on failure.
func main() {
	cmd := &cobra.Command{
		Use:   "plutus",
		Short: "CLI playground for personal investment tools",
		Long: "plutus is a personal CLI workspace for investment-related tooling, including\n" +
			"research and automation experiments such as funding rate arbitrage.",
		Example:       "  plutus demo",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(
		demo.NewCmd(),
	)

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}
