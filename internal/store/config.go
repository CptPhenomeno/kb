package store

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the board configuration stored in .kanban/config.toml.
type Config struct {
	Prefix   string        // Item ID prefix, e.g. "KB" for KB-17.
	ClaimTTL time.Duration // How long a claim lasts without writes.
	Theme    string        // TUI color theme; "" means the default. KB_THEME overrides it.
}

// DefaultConfig returns the configuration used when config.toml is missing
// or leaves a field unset.
func DefaultConfig() Config { return Config{Prefix: "KB", ClaimTTL: 15 * time.Minute} }

var prefixPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)

// ValidatePrefix checks an item ID prefix: upper-case letters and digits,
// starting with a letter.
func ValidatePrefix(p string) error {
	if !prefixPattern.MatchString(p) {
		return fmt.Errorf("invalid prefix %q: use upper-case letters and digits, starting with a letter", p)
	}
	return nil
}

// fileConfig mirrors the TOML file; pointers tell unset keys apart.
type fileConfig struct {
	Prefix   *string `toml:"prefix"`
	ClaimTTL *string `toml:"claim_ttl"`
	Theme    *string `toml:"theme"`
}

// LoadConfig reads config.toml at path. A missing file yields the defaults.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	var fc fileConfig
	if _, err := toml.DecodeFile(path, &fc); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if fc.Prefix != nil {
		if err := ValidatePrefix(*fc.Prefix); err != nil {
			return cfg, fmt.Errorf("%s: %w", path, err)
		}
		cfg.Prefix = *fc.Prefix
	}
	if fc.ClaimTTL != nil {
		ttl, err := time.ParseDuration(*fc.ClaimTTL)
		if err != nil || ttl <= 0 {
			return cfg, fmt.Errorf("%s: invalid claim_ttl %q: use a positive duration such as \"15m\"", path, *fc.ClaimTTL)
		}
		cfg.ClaimTTL = ttl
	}
	if fc.Theme != nil {
		cfg.Theme = *fc.Theme
	}
	return cfg, nil
}

// encode renders the configuration as config.toml content.
func (c Config) encode() string {
	return fmt.Sprintf("# kb board configuration (versioned with the project).\nprefix    = %q\nclaim_ttl = %q\n",
		c.Prefix, c.ClaimTTL.String())
}
