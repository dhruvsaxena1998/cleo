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
	Diagnostics  []Diagnostic        `toml:"-"`
}

// Diagnostic is one boot-time resolution outcome surfaced in the warnings
// popup. OK entries (✓) describe what ended up active; non-OK entries (✗)
// describe what the config requested that did not take effect, and why.
type Diagnostic struct {
	OK     bool
	Detail string
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
	Theme                string      `toml:"theme"`
	Editor               string      `toml:"editor"`
	SidebarWidth         int         `toml:"sidebar_width"`
	EventLogLines        int         `toml:"event_log_lines"`
	StatusTimeoutSeconds float64     `toml:"status_timeout_seconds"`
	PanePreview          PanePreview `toml:"pane_preview"`
}

// StatusTimeout returns the Dashboard status message timeout as a duration.
// The setting is stored in seconds (fractional allowed) so users do not need
// Go duration syntax; this is the one place that knows the unit.
func (u UI) StatusTimeout() time.Duration {
	return time.Duration(u.StatusTimeoutSeconds * float64(time.Second))
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
