package config

import "time"

func Defaults_() Config {
	return Config{
		Defaults: Defaults{DetachKey: "C-b d", DefaultAgent: "claude"},
		Sound: Sound{
			Enabled: true,
			Volume:  0.7,
			Events: map[string]string{
				"session_start":     "start.wav",
				"needs_input":       "attention.wav",
				"session_idle":      "done.wav",
				"session_completed": "done.wav",
				"session_error":     "error.wav",
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
		},
		Retention: Retention{
			HintThreshold:          6,
			PruneKeepDefault:       5,
			IdleToCompletedTimeout: 10 * time.Minute,
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
	if c.Sound.Volume == 0 {
		c.Sound.Volume = d.Sound.Volume
	}
	if c.Sound.Events == nil {
		c.Sound.Events = d.Sound.Events
	}
	if c.Agents == nil {
		c.Agents = d.Agents
	}
	if c.UI.SidebarWidth == 0 {
		c.UI = d.UI
	}
	if c.Retention.HintThreshold == 0 {
		c.Retention = d.Retention
	}
}
