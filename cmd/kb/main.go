// Command kb is a terminal kanban board for developers and coding agents.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/CptPhenomeno/kb/internal/cli"
	"github.com/CptPhenomeno/kb/internal/store"
	"github.com/CptPhenomeno/kb/internal/tui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	app := cli.DefaultApp()
	app.LaunchTUI = func(ctx context.Context, s *cli.Session) error {
		theme := tui.ResolveTheme(os.Getenv("KB_THEME"), store.LoadUserTheme(), s.Config.Theme)
		return tui.Run(ctx, tui.NewBackend(s.Service, s.DB), tui.Options{
			Project: filepath.Base(s.Root), Actor: s.Actor, Now: s.Service.Now, Theme: theme,
			SaveTheme: store.SaveUserTheme,
		})
	}
	code := cli.Run(ctx, os.Args[1:], app)
	stop()
	os.Exit(code)
}
