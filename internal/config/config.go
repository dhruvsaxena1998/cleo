package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Defaults  Defaults         `toml:"defaults"`
	Sound     Sound            `toml:"sound"`
	Agents    map[string]Agent `toml:"agents"`
	UI        UI               `toml:"ui"`
	Retention Retention        `toml:"retention"`
}

type Defaults struct {
	DetachKey    string `toml:"detach_key"`
	DefaultAgent string `toml:"default_agent"`
}

type Sound struct {
	Enabled      bool              `toml:"enabled"`
	Volume       float64           `toml:"volume"`
	Events       map[string]string `toml:"events"`
	EventEnabled map[string]bool   `toml:"event_enabled"`
}

type Agent struct {
	Command string `toml:"command"`
	Label   string `toml:"label"`
	Color   string `toml:"color"`
	Hooks   string `toml:"hooks"` // "claude" | "codex" | "none"
}

type UI struct {
	ShowPanePreview     bool          `toml:"show_pane_preview"`
	PanePreviewLines    int           `toml:"pane_preview_lines"`
	PanePreviewInterval time.Duration `toml:"pane_preview_interval"`
	EventLogLines       int           `toml:"event_log_lines"`
	SidebarWidth        int           `toml:"sidebar_width"`
	Theme               string        `toml:"theme"` // catppuccin-mocha | gruvbox-dark | onedark | void | synthwave
}

type Retention struct {
	HintThreshold          int           `toml:"hint_threshold"`
	PruneKeepDefault       int           `toml:"prune_keep_default"`
	IdleToCompletedTimeout time.Duration `toml:"idle_to_completed_timeout"`
	SpawningTimeout        time.Duration `toml:"spawning_timeout"`
}

// Load reads from path; if not present, writes defaults and returns them.
func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c := Defaults_()
		if err := Save(path, c); err != nil {
			return Config{}, err
		}
		return c, nil
	}
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	mergeDefaults(&c)
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func (c Config) SoundEventEnabled(event string) bool {
	if !c.Sound.Enabled {
		return false
	}
	if c.Sound.EventEnabled == nil {
		return true
	}
	enabled, ok := c.Sound.EventEnabled[event]
	if !ok {
		return true
	}
	return enabled
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
