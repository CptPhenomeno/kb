package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

// App holds everything the CLI takes from its environment, so tests can
// run it in-process with fake input, output, clock and variables.
type App struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	Cwd       string
	Now       func() time.Time
	GitUser   func() string
	LaunchTUI func(context.Context, *Session) error
}

// Session is an open board ready to be used by a command or the TUI.
type Session struct {
	Service *core.Service
	DB      *store.DB
	Config  store.Config
	Root    string
	Actor   string
}

// DefaultApp returns the App of the real process. KB_NOW, an RFC 3339
// time, freezes the clock; it exists for reproducible end-to-end tests.
func DefaultApp() *App {
	cwd, _ := os.Getwd()
	now := time.Now
	if v := os.Getenv("KB_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			now = func() time.Time { return t }
		}
	}
	return &App{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Getenv: os.Getenv, Cwd: cwd, Now: now,
		GitUser: func() string {
			out, err := exec.Command("git", "config", "user.name").Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		},
	}
}

// globals are the flags shared by every command.
type globals struct {
	json  bool
	quiet bool
	as    string
}

// cli is one invocation of kb.
type cli struct {
	app *App
	g   globals
	ctx context.Context
}

// Run executes kb with args and returns the process exit code.
func Run(ctx context.Context, args []string, app *App) int {
	c := &cli{app: app, ctx: ctx}
	root := c.rootCmd()
	root.SetArgs(args)
	root.SetIn(app.Stdin)
	root.SetOut(app.Stdout)
	root.SetErr(app.Stderr)
	err := root.ExecuteContext(ctx)
	if err != nil {
		c.printError(err)
	}
	return ExitCode(err)
}

// ExitCode maps an error to the exit codes of SPEC.md §6.1.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, core.ErrNotFound):
		return 2
	case errors.Is(err, core.ErrRule):
		return 3
	case errors.Is(err, core.ErrConflict):
		return 4
	case errors.Is(err, core.ErrClaimed):
		return 5
	default:
		return 1
	}
}

// usageError is a wrong invocation; it is printed with the usage line.
type usageError struct {
	msg, usage string
}

func (e *usageError) Error() string { return e.msg }

func (c *cli) printError(err error) {
	var re *core.RuleError
	isRule := errors.As(err, &re)
	if c.g.json {
		out := map[string]any{"error": errorKind(err), "message": err.Error()}
		if isRule {
			out["message"] = re.Msg
			if len(re.Unmet) > 0 {
				out["unmet"] = re.Unmet
			}
		}
		enc := json.NewEncoder(c.app.Stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(out)
		return
	}
	w := c.app.Stderr
	switch {
	case isRule:
		fmt.Fprintf(w, "✗ %s\n", re.Msg)
		for _, u := range re.Unmet {
			fmt.Fprintf(w, "  · %s\n", u)
		}
	default:
		fmt.Fprintf(w, "✗ %s\n", err)
	}
	var ue *usageError
	if errors.As(err, &ue) && ue.usage != "" {
		fmt.Fprintf(w, "usage: %s\n", ue.usage)
	}
}

func errorKind(err error) string {
	var re *core.RuleError
	switch {
	case errors.As(err, &re) && len(re.Unmet) > 0:
		return "requirements"
	case errors.Is(err, core.ErrRule):
		return "rule"
	case errors.Is(err, core.ErrNotFound):
		return "not_found"
	case errors.Is(err, core.ErrConflict):
		return "conflict"
	case errors.Is(err, core.ErrClaimed):
		return "claimed"
	default:
		return "error"
	}
}

// open resolves the project root and opens the board.
func (c *cli) open() (*Session, error) {
	root, err := store.ResolveRoot(c.app.Cwd, c.app.Getenv)
	if err != nil {
		return nil, err
	}
	db, cfg, err := store.OpenBoard(root)
	if err != nil {
		return nil, err
	}
	svc := &core.Service{Store: db, Now: c.app.Now, Prefix: cfg.Prefix, ClaimTTL: cfg.ClaimTTL}
	return &Session{Service: svc, DB: db, Config: cfg, Root: root, Actor: c.actor()}, nil
}

func (c *cli) actor() string {
	osUser := ""
	if u, err := user.Current(); err == nil {
		osUser = u.Username
	}
	gitUser := ""
	if c.app.GitUser != nil {
		gitUser = c.app.GitUser()
	}
	return core.ResolveActor(c.g.as, c.app.Getenv("KB_ACTOR"), gitUser, osUser)
}

// with opens the board, runs fn and closes the board.
func (c *cli) with(fn func(*Session) error) error {
	s, err := c.open()
	if err != nil {
		return err
	}
	defer s.DB.Close()
	return fn(s)
}

// args returns a cobra validator that reports wrong arity as a usage error.
func args(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, a []string) error {
		if len(a) < min || (max >= 0 && len(a) > max) {
			return &usageError{msg: fmt.Sprintf("%s: wrong number of arguments", cmd.CommandPath()), usage: cmd.UseLine()}
		}
		return nil
	}
}

// unknownCommand reports a positional argument of the root as an unknown
// command, with Cobra's suggestions when a command is close enough.
func unknownCommand(cmd *cobra.Command, a []string) error {
	if len(a) == 0 {
		return nil
	}
	msg := fmt.Sprintf("unknown command %q for %q", a[0], cmd.CommandPath())
	if s := closest(a[0], cmd.SuggestionsFor(a[0])); len(s) > 0 {
		q := make([]string, len(s))
		for i, name := range s {
			q[i] = strconv.Quote(name)
		}
		alt := q[len(q)-1]
		if len(q) > 1 {
			alt = strings.Join(q[:len(q)-1], ", ") + " or " + alt
		}
		msg += fmt.Sprintf("; did you mean %s?", alt)
	}
	return &usageError{msg: msg, usage: cmd.CommandPath() + " [command]"}
}

// closest keeps the names nearest to typed, counting a swap of two adjacent
// letters as one edit, so "lsit" suggests "list" and not every 4-letter
// command; names that start with typed come first, as Cobra suggests them too.
func closest(typed string, names []string) []string {
	typed = strings.ToLower(typed)
	best, out := -1, []string(nil)
	for _, n := range names {
		d := 0
		if !strings.HasPrefix(strings.ToLower(n), typed) {
			d = editDistance(typed, strings.ToLower(n))
		}
		switch {
		case best < 0 || d < best:
			best, out = d, []string{n}
		case d == best:
			out = append(out, n)
		}
	}
	return out
}

// editDistance is the optimal string alignment distance between a and b.
func editDistance(a, b string) int {
	s, t := []rune(a), []rune(b)
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			cost := 1
			if s[i-1] == t[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && s[i-1] == t[j-2] && s[i-2] == t[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(s)][len(t)]
}

func (c *cli) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kb",
		Short:         "Terminal kanban board for developers and coding agents",
		Long:          "kb tracks the work of a project on a kanban board shared by the developer (TUI)\nand coding agents (CLI). Run `kb guide` for the agent workflow.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          unknownCommand,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.with(func(s *Session) error {
				if c.app.LaunchTUI == nil {
					return errors.New("the TUI is not available in this build")
				}
				return c.app.LaunchTUI(cmd.Context(), s)
			})
		},
	}
	// Cobra applies its default distance only to its own error message.
	root.SuggestionsMinimumDistance = 2
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{msg: err.Error(), usage: cmd.UseLine()}
	})
	pf := root.PersistentFlags()
	pf.BoolVar(&c.g.json, "json", false, "print machine-readable JSON")
	pf.BoolVarP(&c.g.quiet, "quiet", "q", false, "print only item IDs")
	pf.StringVar(&c.g.as, "as", "", "act as this actor (default: $KB_ACTOR, then @git-user)")
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		c.versionCmd(), c.initCmd(), c.addCmd(), c.showCmd(), c.listCmd(), c.treeCmd(),
		c.editCmd(), c.moveCmd(), c.linkCmd(true), c.linkCmd(false), c.nextCmd(),
		c.claimCmd(), c.releaseCmd(), c.touchCmd(), c.phaseCmd(), c.testCmd(), c.acceptCmd(),
		c.noteCmd(), c.rmCmd(), c.logCmd(), c.exportCmd(), c.guideCmd(), c.importCmd(),
	)
	return root
}
