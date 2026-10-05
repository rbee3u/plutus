// Command plutus runs the personal read-only portfolio observer.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/rbee3u/plutus/internal/cli"
)

// version is the CLI release string shown by --version. `make build` sets it
// through -ldflags "-X main.version=..." from `git describe`; a plain `go run` reports "dev".
var version = "dev"

func main() {
	if err := cli.NewRootCommand(version).Execute(); err != nil {
		var reported *cli.ReportedError
		if !errors.As(err, &reported) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
