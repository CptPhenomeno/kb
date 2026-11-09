package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// User preferences live outside the project, in $XDG_CONFIG_HOME/kb (or the
// platform's user config directory): they are personal, unlike the
// versioned .kanban/config.toml.

func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kb", "config.toml"), nil
}

// LoadUserTheme returns the theme saved in the user preferences, or "".
func LoadUserTheme() string {
	path, err := userConfigPath()
	if err != nil {
		return ""
	}
	var prefs struct {
		Theme string `toml:"theme"`
	}
	if _, err := toml.DecodeFile(path, &prefs); err != nil {
		return ""
	}
	return prefs.Theme
}

// SaveUserTheme stores the theme in the user preferences.
func SaveUserTheme(name string) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("# kb user preferences (not tied to a project).\ntheme = %q\n", name)
	return os.WriteFile(path, []byte(body), 0o644)
}
