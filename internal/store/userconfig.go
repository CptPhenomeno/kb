package store

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// User preferences live outside the project, in $XDG_CONFIG_HOME/kb (or the
// platform's user config directory): they are personal, unlike the
// versioned .kanban/config.toml.

// UserPrefs are the personal preferences of the TUI user.
type UserPrefs struct {
	Theme       string `toml:"theme,omitempty"`
	ShowParents *bool  `toml:"show_parents,omitempty"` // epics and stories on the board
	BoardSort   string `toml:"board_sort,omitempty"`   // "recent" or "manual"
}

func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kb", "config.toml"), nil
}

// LoadUserPrefs reads the user preferences; missing or unreadable
// preferences are empty.
func LoadUserPrefs() UserPrefs {
	var p UserPrefs
	if path, err := userConfigPath(); err == nil {
		_, _ = toml.DecodeFile(path, &p)
	}
	return p
}

// SaveUserPrefs writes the user preferences.
func SaveUserPrefs(p UserPrefs) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# kb user preferences (not tied to a project).\n")
	if err := toml.NewEncoder(&buf).Encode(p); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// LoadUserTheme returns the theme saved in the user preferences, or "".
func LoadUserTheme() string { return LoadUserPrefs().Theme }

// SaveUserTheme stores the theme, keeping the other preferences.
func SaveUserTheme(name string) error {
	p := LoadUserPrefs()
	p.Theme = name
	return SaveUserPrefs(p)
}

// SaveUserShowParents stores whether epics and stories are shown on the
// board, keeping the other preferences.
func SaveUserShowParents(show bool) error {
	p := LoadUserPrefs()
	p.ShowParents = &show
	return SaveUserPrefs(p)
}

// SaveUserBoardSort stores the order of the board columns ("recent" or
// "manual"), keeping the other preferences.
func SaveUserBoardSort(order string) error {
	p := LoadUserPrefs()
	p.BoardSort = order
	return SaveUserPrefs(p)
}
