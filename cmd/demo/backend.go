// Package demo provides a minimal command used to verify the CLI wiring.
package demo

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/pkg/demo"
)

type backend struct{}

// NewCmd returns the demo subcommand.
func NewCmd() *cobra.Command {
	b := &backend{}
	cmd := &cobra.Command{
		Use:     "demo",
		Short:   "Print a placeholder message",
		Example: "plutus demo",
		Args:    cobra.NoArgs,
		RunE:    b.runE,
	}
	return cmd
}

func (b *backend) runE(_ *cobra.Command, _ []string) error {
	fmt.Println(demo.Result())
	return nil
}
