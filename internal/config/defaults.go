package config

import "time"

func Defaults_() Config {
	return Config{
		DefaultAgent: "claude",
		Tmux:         Tmux{DetachKey: "C-b d"},
		Sound: Sound{
			Enabled: true,
			Volume:  0.7,
			Events: map[string]SoundEvent{
				"session_start":     {File: "start.wav", Enabled: true},
				"needs_input":       {File: "attention.wav", Enabled: true},
				"session_idle":      {File: "done.wav", Enabled: true},
				"session_completed": {File: "done.wav", Enabled: true},
				"session_error":     {File: "error.wav", Enabled: true},
			},
		},
		Agents: map[string]Agent{
			"claude":   {Command: "claude", Label: "cl", Color: "#CC785C"},
			"codex":    {Command: "codex", Label: "cx", Color: "#10A37F"},
			"opencode": {Command: "opencode", Label: "oc", Color: "#FF6B35"},
			"pi":       {Command: "pi", Label: "pi", Color: "#7C3AED"},
		},
		UI: UI{
			Theme:                "catppuccin-mocha",
			Editor:               "",
			SidebarWidth:         48,
			EventLogLines:        200,
			StatusTimeoutSeconds: 3,
			PanePreview: PanePreview{
				Enabled:  true,
				Lines:    30,
				Interval: 2000 * time.Millisecond,
			},
			Mouse: Mouse{Enabled: true},
		},
		Timeouts: Timeouts{
			IdleToCompletedTimeout: 10 * time.Minute,
			SpawningTimeout:        30 * time.Second,
		},
		Pruning: Pruning{
			HintThreshold: 6,
			KeepDefault:   5,
		},
	}
}
