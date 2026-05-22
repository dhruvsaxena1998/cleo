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

func validate(c *Config) {
	defaults := Defaults_()
	c.Warnings = nil
	if c.Sound.Volume < 0 {
		c.Warnings = append(c.Warnings, "sound.volume below 0; clamped to 0")
		c.Sound.Volume = 0
	}
	if c.Sound.Volume > 1 {
		c.Warnings = append(c.Warnings, "sound.volume above 1; clamped to 1")
		c.Sound.Volume = 1
	}
	if c.UI.PanePreview.Interval < 100*time.Millisecond {
		c.Warnings = append(c.Warnings, "ui.pane_preview.interval below 100ms; clamped to 100ms")
		c.UI.PanePreview.Interval = 100 * time.Millisecond
	}
	if c.UI.PanePreview.Lines < 1 {
		c.Warnings = append(c.Warnings, "ui.pane_preview.lines below 1; clamped to 1")
		c.UI.PanePreview.Lines = 1
	}
	if c.UI.EventLogLines < 10 {
		c.Warnings = append(c.Warnings, "ui.event_log_lines below 10; clamped to 10")
		c.UI.EventLogLines = 10
	}
	if c.UI.SidebarWidth < 10 {
		c.Warnings = append(c.Warnings, "ui.sidebar_width below 10; clamped to 10")
		c.UI.SidebarWidth = 10
	}
	if c.UI.SidebarWidth > 200 {
		c.Warnings = append(c.Warnings, "ui.sidebar_width above 200; clamped to 200")
		c.UI.SidebarWidth = 200
	}
	if c.UI.Theme == "" || !validThemes[c.UI.Theme] {
		c.Warnings = append(c.Warnings, fmt.Sprintf("ui.theme %q is unknown; using %q", c.UI.Theme, defaults.UI.Theme))
		c.UI.Theme = defaults.UI.Theme
	}
	if c.Timeouts.IdleToCompletedTimeout < 100*time.Millisecond {
		c.Warnings = append(c.Warnings, "timeouts.idle_to_completed_timeout below 100ms; clamped to 100ms")
		c.Timeouts.IdleToCompletedTimeout = 100 * time.Millisecond
	}
	if c.Timeouts.SpawningTimeout < 100*time.Millisecond {
		c.Warnings = append(c.Warnings, "timeouts.spawning_timeout below 100ms; clamped to 100ms")
		c.Timeouts.SpawningTimeout = 100 * time.Millisecond
	}
	for name, agent := range c.Agents {
		if agent.Command == "" {
			agent.Command = name
			c.Agents[name] = agent
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
