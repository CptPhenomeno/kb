// Command kb is a terminal kanban board for developers and coding agents.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/CptPhenomeno/kb/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, os.Args[1:], cli.DefaultApp())
	stop()
	os.Exit(code)
}
