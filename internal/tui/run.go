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

func (b *serviceBackend) Move(ctx context.Context, actor, id string, to core.Status, ifVersion int) (*core.MoveResult, error) {
	return b.svc.Move(ctx, actor, id, to, ifVersion)
}

func (b *serviceBackend) ArrangeColumn(ctx context.Context, actor string, status core.Status, ids []string) error {
	return b.svc.ArrangeColumn(ctx, actor, status, ids)
}

func (b *serviceBackend) Reorder(ctx context.Context, actor, id string, index, ifVersion int) (*core.Item, error) {
	return b.svc.Reorder(ctx, actor, id, index, ifVersion)
}

func (b *serviceBackend) SetCriterion(ctx context.Context, actor, id string, n int, done bool, ifVersion int) (*core.Item, error) {
	return b.svc.SetCriterion(ctx, actor, id, n, done, ifVersion)
}

func (b *serviceBackend) AddCriterion(ctx context.Context, actor, id, text string, ifVersion int) (*core.Item, error) {
	return b.svc.AddCriterion(ctx, actor, id, text, ifVersion)
}

func (b *serviceBackend) EditCriterion(ctx context.Context, actor, id string, n int, text string, ifVersion int) (*core.Item, error) {
	return b.svc.EditCriterion(ctx, actor, id, n, text, ifVersion)
}

func (b *serviceBackend) Remove(ctx context.Context, actor, id string, recursive bool) ([]string, error) {
	return b.svc.Remove(ctx, actor, id, recursive)
}

func (b *serviceBackend) Claim(ctx context.Context, actor, id string, ifVersion int) (*core.Item, error) {
	return b.svc.Claim(ctx, actor, id, ifVersion)
}

func (b *serviceBackend) Release(ctx context.Context, actor, id string, ifVersion int) (*core.Item, error) {
	return b.svc.Release(ctx, actor, id, ifVersion)
}

func (b *serviceBackend) Link(ctx context.Context, actor string, r core.Relation) error {
	return b.svc.Link(ctx, actor, r)
}

func (b *serviceBackend) Unlink(ctx context.Context, actor string, r core.Relation) error {
	return b.svc.Unlink(ctx, actor, r)
}

func (b *serviceBackend) Create(ctx context.Context, actor string, it core.Item) (*core.Item, error) {
	return b.svc.Create(ctx, actor, it)
}

func (b *serviceBackend) Edit(ctx context.Context, actor, id string, p core.Patch, ifVersion int) (*core.Item, error) {
	return b.svc.Edit(ctx, actor, id, p, ifVersion)
}
