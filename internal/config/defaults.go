package config

import "time"

func Defaults_() Config {
	enabled := true
	return Config{
		Defaults: Defaults{DetachKey: "C-b d", DefaultAgent: "claude"},
		Sound: Sound{
			Enabled: &enabled,
			Volume:  0.7,
			Events: map[string]string{
				"session_start":     "start.wav",
				"needs_input":       "attention.wav",
				"session_idle":      "done.wav",
				"session_completed": "done.wav",
				"session_error":     "error.wav",
			},
			EventEnabled: map[string]bool{
				"session_start":     true,
				"needs_input":       true,
				"session_idle":      true,
				"session_completed": true,
				"session_error":     true,
			},
		},
		Agents: map[string]Agent{
			"claude":   {Command: "claude", Label: "cl", Color: "#CC785C", Hooks: "claude"},
			"codex":    {Command: "codex", Label: "cx", Color: "#10A37F", Hooks: "codex"},
			"opencode": {Command: "opencode", Label: "oc", Color: "#FF6B35", Hooks: "none"},
			"pi":       {Command: "pi", Label: "pi", Color: "#7C3AED", Hooks: "none"},
		},
		UI: UI{
			ShowPanePreview:     true,
			PanePreviewLines:    30,
			PanePreviewInterval: 1500 * time.Millisecond,
			EventLogLines:       200,
			SidebarWidth:        32,
			Theme:               "catppuccin-mocha",
		},
		Retention: Retention{
			HintThreshold:          6,
			PruneKeepDefault:       5,
			IdleToCompletedTimeout: 10 * time.Minute,
			SpawningTimeout:        30 * time.Second,
		},
	}
}

// mergeDefaults fills missing fields on a partially-specified config.
func mergeDefaults(c *Config) {
	d := Defaults_()
	if c.Defaults.DefaultAgent == "" {
		c.Defaults.DefaultAgent = d.Defaults.DefaultAgent
	}
	if c.Defaults.DetachKey == "" {
		c.Defaults.DetachKey = d.Defaults.DetachKey
	}
	if c.Sound.Enabled == nil {
		enabled := true
		c.Sound.Enabled = &enabled
	}
	if c.Sound.Volume == 0 {
		c.Sound.Volume = d.Sound.Volume
	}
	if c.Sound.Events == nil {
		c.Sound.Events = d.Sound.Events
	}
	if c.Sound.EventEnabled == nil {
		c.Sound.EventEnabled = d.Sound.EventEnabled
	} else {
		for ev, enabled := range d.Sound.EventEnabled {
			if _, ok := c.Sound.EventEnabled[ev]; !ok {
				c.Sound.EventEnabled[ev] = enabled
			}
		}
	}
	if c.Agents == nil {
		c.Agents = d.Agents
	}
	if c.UI.SidebarWidth == 0 {
		userTheme := c.UI.Theme
		c.UI = d.UI
		if userTheme != "" {
			c.UI.Theme = userTheme
		}
	}
	if c.UI.Theme == "" {
		c.UI.Theme = d.UI.Theme
	}
	if c.Retention.HintThreshold == 0 {
		c.Retention = d.Retention
	}
	if c.Retention.SpawningTimeout == 0 {
		c.Retention.SpawningTimeout = d.Retention.SpawningTimeout
	}
}
