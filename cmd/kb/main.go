// Command kb is a terminal kanban board for developers and coding agents.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/CptPhenomeno/kb/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the command line and returns the process exit code.
// It is a temporary entry point until the cobra-based CLI lands in phase 4.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	}
	fmt.Fprintln(stderr, "usage: kb version")
	return 1
}
