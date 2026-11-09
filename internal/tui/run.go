package tui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// NewBackend reads the board through the core service and watches the
// database for writes made by other processes.
func NewBackend(svc *core.Service, db interface {
	DataVersion(context.Context) (int64, error)
}) Backend {
	return &serviceBackend{svc: svc, db: db}
}

type serviceBackend struct {
	svc *core.Service
	db  interface {
		DataVersion(context.Context) (int64, error)
	}
}

func (b *serviceBackend) Board(ctx context.Context) (*core.Snapshot, error) { return b.svc.Board(ctx) }
func (b *serviceBackend) Log(ctx context.Context, f core.LogFilter) ([]core.Event, error) {
	return b.svc.Log(ctx, f)
}
func (b *serviceBackend) DataVersion(ctx context.Context) (int64, error) {
	return b.db.DataVersion(ctx)
}

// Run starts the TUI and blocks until the user quits or ctx is cancelled.
func Run(ctx context.Context, be Backend, opts Options) error {
	_, err := tea.NewProgram(New(ctx, be, opts), tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}
