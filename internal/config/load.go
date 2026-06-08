package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

var validThemes = map[string]bool{
	"catppuccin-mocha": true,
	"gruvbox-dark":     true,
	"onedark":          true,
	"void":             true,
	"synthwave":        true,
}

// Clamp bounds enforced by validate. Exported so the in-app settings editor can
// constrain its steppers to the same limits the validator enforces, keeping the
// two from drifting apart.
const (
	MinSoundVolume = 0.0
	MaxSoundVolume = 1.0

	MinPanePreviewInterval = 100 * time.Millisecond
	MinPanePreviewLines    = 1

	MinEventLogLines = 10

	MinSidebarWidth = 10
	MaxSidebarWidth = 200

	MinStatusTimeoutSeconds = 0.5
	MaxStatusTimeoutSeconds = 10.0

	MinTimeout = 100 * time.Millisecond
)

// Normalize clamps out-of-range values and recomputes the derived Keymap,
// Warnings, and Diagnostics — the same pass Load runs after decoding. Callers
// that mutate a Config in memory (the in-app settings editor) run this before
// Save so persisted values match what the next Load would have produced.
func Normalize(c *Config) { validate(c) }

// Load reads from path; if not present, writes defaults and returns them.
func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c := Defaults_()
		validate(&c)
		if err := Save(path, c); err != nil {
			return Config{}, err
		}
		return c, nil
	}
	c := Defaults_()
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	mergeMaps(&c, Defaults_())
	validate(&c)
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
	soundEvent, ok := c.Sound.Events[event]
	if !ok {
		return true
	}
	return soundEvent.Enabled
}

func mergeMaps(c *Config, defaults Config) {
	if c.Sound.Events == nil {
		c.Sound.Events = defaults.Sound.Events
	} else {
		for event, defaultEvent := range defaults.Sound.Events {
			current, ok := c.Sound.Events[event]
			if !ok {
				c.Sound.Events[event] = defaultEvent
				continue
			}
			if current.File == "" {
				current.File = defaultEvent.File
			}
			c.Sound.Events[event] = current
		}
	}
	if c.Agents == nil {
		c.Agents = defaults.Agents
	} else {
		for name, defaultAgent := range defaults.Agents {
			current, ok := c.Agents[name]
			if !ok {
				c.Agents[name] = defaultAgent
				continue
			}
			if current.Command == "" {
				current.Command = defaultAgent.Command
			}
			if current.Label == "" {
				current.Label = defaultAgent.Label
			}
			if current.Color == "" {
				current.Color = defaultAgent.Color
			}
			c.Agents[name] = current
		}
	}
}

// adjust records a setting that was changed from what the config requested: it
// feeds both the doctor warning list and the boot popup's ✗ diagnostics.
func (c *Config) adjust(detail string) {
	c.Warnings = append(c.Warnings, detail)
	c.Diagnostics = append(c.Diagnostics, Diagnostic{OK: false, Detail: detail})
}

func validate(c *Config) {
	defaults := Defaults_()
	c.Warnings = nil
	c.Diagnostics = nil
	if c.Sound.Volume < MinSoundVolume {
		c.adjust("sound.volume below 0; clamped to 0")
		c.Sound.Volume = MinSoundVolume
	}
	if c.Sound.Volume > MaxSoundVolume {
		c.adjust("sound.volume above 1; clamped to 1")
		c.Sound.Volume = MaxSoundVolume
	}
	if c.UI.PanePreview.Interval < MinPanePreviewInterval {
		c.adjust("ui.pane_preview.interval below 100ms; clamped to 100ms")
		c.UI.PanePreview.Interval = MinPanePreviewInterval
	}
	if c.UI.PanePreview.Lines < MinPanePreviewLines {
		c.adjust("ui.pane_preview.lines below 1; clamped to 1")
		c.UI.PanePreview.Lines = MinPanePreviewLines
	}
	if c.UI.EventLogLines < MinEventLogLines {
		c.adjust("ui.event_log_lines below 10; clamped to 10")
		c.UI.EventLogLines = MinEventLogLines
	}
	if c.UI.SidebarWidth < MinSidebarWidth {
		c.adjust("ui.sidebar_width below 10; clamped to 10")
		c.UI.SidebarWidth = MinSidebarWidth
	}
	if c.UI.SidebarWidth > MaxSidebarWidth {
		c.adjust("ui.sidebar_width above 200; clamped to 200")
		c.UI.SidebarWidth = MaxSidebarWidth
	}
	if c.UI.StatusTimeoutSeconds < MinStatusTimeoutSeconds {
		c.adjust("ui.status_timeout_seconds below 0.5s; clamped to 0.5s")
		c.UI.StatusTimeoutSeconds = MinStatusTimeoutSeconds
	}
	if c.UI.StatusTimeoutSeconds > MaxStatusTimeoutSeconds {
		c.adjust("ui.status_timeout_seconds above 10s; clamped to 10s")
		c.UI.StatusTimeoutSeconds = MaxStatusTimeoutSeconds
	}
	if c.UI.Theme == "" || !validThemes[c.UI.Theme] {
		c.Warnings = append(c.Warnings, fmt.Sprintf("ui.theme %q is unknown; using %q", c.UI.Theme, defaults.UI.Theme))
		c.Diagnostics = append(c.Diagnostics,
			Diagnostic{OK: false, Detail: fmt.Sprintf("ui.theme %q is unknown", c.UI.Theme)},
			Diagnostic{OK: true, Detail: fmt.Sprintf("ui.theme falls back to %q", defaults.UI.Theme)},
		)
		c.UI.Theme = defaults.UI.Theme
	}
	if c.Timeouts.IdleToCompletedTimeout < MinTimeout {
		c.adjust("timeouts.idle_to_completed_timeout below 100ms; clamped to 100ms")
		c.Timeouts.IdleToCompletedTimeout = MinTimeout
	}
	if c.Timeouts.SpawningTimeout < MinTimeout {
		c.adjust("timeouts.spawning_timeout below 100ms; clamped to 100ms")
		c.Timeouts.SpawningTimeout = MinTimeout
	}
	for name, agent := range c.Agents {
		if agent.Command == "" {
			agent.Command = name
			c.Agents[name] = agent
		}
	}
	km, keybindDiags := resolveKeymap(c.Keybinds)
	c.Keymap = km
	c.Diagnostics = append(c.Diagnostics, keybindDiags...)
	for _, d := range keybindDiags {
		if !d.OK {
			c.Warnings = append(c.Warnings, d.Detail)
		}
	}
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
