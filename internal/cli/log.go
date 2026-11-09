package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/CptPhenomeno/kb/internal/core"
)

// followInterval is how often `kb log --follow` checks for new events.
const followInterval = 200 * time.Millisecond

func (c *cli) logCmd() *cobra.Command {
	var since string
	var limit int
	var follow bool
	cmd := &cobra.Command{
		Use:   "log [id]",
		Short: "Show the event log (JSON Lines with --json)",
		Args:  args(0, 1),
		RunE: func(cmd *cobra.Command, a []string) error {
			var f core.LogFilter
			if since != "" {
				t, err := parseSince(since, c.app.Now())
				if err != nil {
					return err
				}
				f.Since = t
			}
			f.Limit = limit
			return c.with(func(s *Session) error {
				if len(a) == 1 {
					id, err := resolveID(s, c, a[0])
					if errors.Is(err, core.ErrNotFound) {
						id, err = a[0], nil // deleted items keep their history
					}
					if err != nil {
						return err
					}
					f.Item = id
				}
				evs, err := s.Service.Log(c.ctx, f)
				if err != nil {
					return err
				}
				last := c.printEvents(evs, f.AfterID)
				if !follow {
					return nil
				}
				f.Limit = 0
				return c.follow(cmd, s, f, last)
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only events since a time (RFC 3339) or a duration ago (10m, 2h)")
	cmd.Flags().IntVar(&limit, "limit", 0, "only the newest N events")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing new events until interrupted")
	return cmd
}

// follow polls the database for writes by other processes and prints the
// new events, until the command's context is cancelled (SIGINT).
func (c *cli) follow(cmd *cobra.Command, s *Session, f core.LogFilter, last int64) error {
	ctx := cmd.Context()
	version, err := s.DB.DataVersion(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		v, err := s.DB.DataVersion(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if v == version {
			continue
		}
		version = v
		f.AfterID = last
		evs, err := s.Service.Log(ctx, f)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		last = c.printEvents(evs, last)
	}
}

// printEvents prints events and returns the ID of the last one (or prev).
func (c *cli) printEvents(evs []core.Event, prev int64) int64 {
	switch {
	case c.g.json:
		enc := json.NewEncoder(c.app.Stdout)
		enc.SetEscapeHTML(false)
		for _, e := range evs {
			_ = enc.Encode(newEventView(e))
		}
	case c.g.quiet:
		for _, e := range evs {
			fmt.Fprintln(c.app.Stdout, e.ID)
		}
	default:
		t := newTable(c.app.Stdout)
		for _, e := range evs {
			fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", e.At.UTC().Format("2006-01-02 15:04"), e.Actor, e.Item, describeEvent(e))
		}
		t.Flush()
	}
	if len(evs) > 0 {
		return evs[len(evs)-1].ID
	}
	return prev
}

// parseSince accepts an RFC 3339 time or a duration back from now.
func parseSince(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(s); err == nil && d >= 0 {
		return now.Add(-d), nil
	}
	return time.Time{}, &usageError{msg: fmt.Sprintf("invalid --since %q: use a time like 2026-11-09T14:00:00Z or a duration like 10m", s)}
}
