package app

import (
	"fmt"
	"os"

	"github.com/janburzinski/portk/internal/cli"
	"github.com/janburzinski/portk/internal/tui"
)

// Run executes portk and returns a process exit code.
func Run(args []string) int {
	showAll, args := cli.ExtractAllFlag(args)

	if len(args) == 0 {
		if err := tui.Run(showAll); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
		return 0
	}

	return cli.Execute(args, showAll)
}
