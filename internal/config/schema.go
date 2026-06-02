package config

import "time"

type Config struct {
	DefaultAgent string              `toml:"default_agent"`
	Tmux         Tmux                `toml:"tmux"`
	Sound        Sound               `toml:"sound"`
	Agents       map[string]Agent    `toml:"agents"`
	UI           UI                  `toml:"ui"`
	Timeouts     Timeouts            `toml:"timeouts"`
	Pruning      Pruning             `toml:"pruning"`
	Keybinds     map[string][]string `toml:"keybinds,omitempty"`
	Keymap       Keymap              `toml:"-"`
	Warnings     []string            `toml:"-"`
}

type Tmux struct {
	DetachKey string `toml:"detach_key"`
}

type Sound struct {
	Enabled bool                  `toml:"enabled"`
	Volume  float64               `toml:"volume"`
	Events  map[string]SoundEvent `toml:"events"`
}

type SoundEvent struct {
	File    string `toml:"file"`
	Enabled bool   `toml:"enabled"`
}

type Agent struct {
	Command string `toml:"command"`
	Label   string `toml:"label"`
	Color   string `toml:"color"`
}

type UI struct {
	Theme         string      `toml:"theme"`
	Editor        string      `toml:"editor"`
	SidebarWidth  int         `toml:"sidebar_width"`
	EventLogLines int         `toml:"event_log_lines"`
	PanePreview   PanePreview `toml:"pane_preview"`
}

type PanePreview struct {
	Enabled  bool          `toml:"enabled"`
	Lines    int           `toml:"lines"`
	Interval time.Duration `toml:"interval"`
}

type Timeouts struct {
	IdleToCompletedTimeout time.Duration `toml:"idle_to_completed_timeout"`
	SpawningTimeout        time.Duration `toml:"spawning_timeout"`
}

type Pruning struct {
	HintThreshold int `toml:"hint_threshold"`
	KeepDefault   int `toml:"keep_default"`
}
