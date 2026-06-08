package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultsWritesNewConfigShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.DefaultAgent != "claude" {
		t.Errorf("default agent: %q", c.DefaultAgent)
	}
	if c.Tmux.DetachKey != "C-b d" {
		t.Errorf("detach key: %q", c.Tmux.DetachKey)
	}
	if !c.Sound.Enabled {
		t.Error("sound should default enabled")
	}
	if c.Sound.Events["session_completed"].File != "done.wav" {
		t.Errorf("session_completed file: %q", c.Sound.Events["session_completed"].File)
	}
	if !c.Sound.Events["session_completed"].Enabled {
		t.Error("session_completed should default enabled")
	}
	if c.Agents["claude"].Label != "cl" {
		t.Errorf("claude label: %q", c.Agents["claude"].Label)
	}
	if c.UI.Editor != "" {
		t.Errorf("ui.editor = %q, want empty", c.UI.Editor)
	}
	if c.UI.PanePreview.Interval != 2000*time.Millisecond {
		t.Errorf("interval: %v", c.UI.PanePreview.Interval)
	}
	if c.UI.StatusTimeoutSeconds != 3 {
		t.Errorf("status_timeout_seconds = %v, want 3", c.UI.StatusTimeoutSeconds)
	}
	if c.Timeouts.IdleToCompletedTimeout != 10*time.Minute {
		t.Errorf("idle timeout: %v", c.Timeouts.IdleToCompletedTimeout)
	}
	if c.Pruning.HintThreshold != 6 {
		t.Errorf("hint threshold: %d", c.Pruning.HintThreshold)
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("default config should not warn: %v", c.Warnings)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "[defaults]") || strings.Contains(text, "[retention]") || strings.Contains(text, "event_enabled") || strings.Contains(text, "hooks") {
		t.Fatalf("default config used old shape:\n%s", text)
	}
	for _, want := range []string{
		`default_agent = "claude"`,
		"[tmux]",
		"[sound.events.session_completed]",
		"[ui.pane_preview]",
		"[timeouts]",
		"[pruning]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("default config missing %q:\n%s", want, text)
		}
	}
}

func TestPartialSoundEventOverridePreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sound.events.session_completed]
  enabled = false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.SoundEventEnabled("session_completed") {
		t.Error("session_completed should remain disabled")
	}
	if got := c.Sound.Events["session_completed"].File; got != "done.wav" {
		t.Errorf("session_completed file = %q, want default", got)
	}
	if !c.SoundEventEnabled("session_start") {
		t.Error("missing sibling event should remain enabled")
	}
	if got := c.Sound.Events["session_start"].File; got != "start.wav" {
		t.Errorf("session_start file = %q, want default", got)
	}
}

func TestPartialAgentOverridePreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[agents.claude]
  color = "#ffffff"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	claude := c.Agents["claude"]
	if claude.Color != "#ffffff" {
		t.Errorf("claude color = %q", claude.Color)
	}
	if claude.Command != "claude" {
		t.Errorf("claude command = %q, want default", claude.Command)
	}
	if claude.Label != "cl" {
		t.Errorf("claude label = %q, want default", claude.Label)
	}
	if c.Agents["codex"].Command != "codex" {
		t.Errorf("codex sibling missing: %#v", c.Agents["codex"])
	}
}

func TestLoadOverlaysEveryConfigSetting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
default_agent = "codex"

[tmux]
  detach_key = "C-b x"

[sound]
  enabled = false
  volume = 0.25
  [sound.events.session_start]
    file = "error.wav"
    enabled = false

[agents.claude]
  command = "claude --debug"
  label = "zz"
  color = "#ffffff"

[ui]
  theme = "gruvbox-dark"
  editor = "code --reuse-window"
  sidebar_width = 60
  event_log_lines = 42
  [ui.pane_preview]
    enabled = false
    lines = 7
    interval = "2s"

[timeouts]
  idle_to_completed_timeout = "3s"
  spawning_timeout = "4s"

[pruning]
  hint_threshold = 2
  keep_default = 1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.DefaultAgent != "codex" {
		t.Errorf("default_agent = %q", c.DefaultAgent)
	}
	if c.Tmux.DetachKey != "C-b x" {
		t.Errorf("tmux.detach_key = %q", c.Tmux.DetachKey)
	}
	if c.Sound.Enabled {
		t.Error("sound.enabled should be false")
	}
	if c.Sound.Volume != 0.25 {
		t.Errorf("sound.volume = %f", c.Sound.Volume)
	}
	if got := c.Sound.Events["session_start"]; got.File != "error.wav" || got.Enabled {
		t.Errorf("sound.events.session_start = %#v", got)
	}
	if got := c.Agents["claude"]; got.Command != "claude --debug" || got.Label != "zz" || got.Color != "#ffffff" {
		t.Errorf("agents.claude = %#v", got)
	}
	if c.UI.Theme != "gruvbox-dark" {
		t.Errorf("ui.theme = %q", c.UI.Theme)
	}
	if c.UI.Editor != "code --reuse-window" {
		t.Errorf("ui.editor = %q", c.UI.Editor)
	}
	if c.UI.SidebarWidth != 60 {
		t.Errorf("ui.sidebar_width = %d", c.UI.SidebarWidth)
	}
	if c.UI.EventLogLines != 42 {
		t.Errorf("ui.event_log_lines = %d", c.UI.EventLogLines)
	}
	if c.UI.PanePreview.Enabled {
		t.Error("ui.pane_preview.enabled should be false")
	}
	if c.UI.PanePreview.Lines != 7 {
		t.Errorf("ui.pane_preview.lines = %d", c.UI.PanePreview.Lines)
	}
	if c.UI.PanePreview.Interval != 2*time.Second {
		t.Errorf("ui.pane_preview.interval = %v", c.UI.PanePreview.Interval)
	}
	if c.Timeouts.IdleToCompletedTimeout != 3*time.Second {
		t.Errorf("timeouts.idle_to_completed_timeout = %v", c.Timeouts.IdleToCompletedTimeout)
	}
	if c.Timeouts.SpawningTimeout != 4*time.Second {
		t.Errorf("timeouts.spawning_timeout = %v", c.Timeouts.SpawningTimeout)
	}
	if c.Pruning.HintThreshold != 2 {
		t.Errorf("pruning.hint_threshold = %d", c.Pruning.HintThreshold)
	}
	if c.Pruning.KeepDefault != 1 {
		t.Errorf("pruning.keep_default = %d", c.Pruning.KeepDefault)
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("valid config should not warn: %v", c.Warnings)
	}
}

func TestValidationClampsAndWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sound]
  volume = 2.5

[ui]
  theme = "missing"
  sidebar_width = 2
  event_log_lines = 1
  [ui.pane_preview]
    lines = 0
    interval = "5ms"

[timeouts]
  idle_to_completed_timeout = "1ms"
  spawning_timeout = "1ms"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Sound.Volume != 1 {
		t.Errorf("volume = %f, want clamped to 1", c.Sound.Volume)
	}
	if c.UI.Theme != "catppuccin-mocha" {
		t.Errorf("theme = %q, want fallback", c.UI.Theme)
	}
	if c.UI.SidebarWidth != 10 {
		t.Errorf("sidebar width = %d, want 10", c.UI.SidebarWidth)
	}
	if c.UI.EventLogLines != 10 {
		t.Errorf("event log lines = %d, want 10", c.UI.EventLogLines)
	}
	if c.UI.PanePreview.Lines != 1 {
		t.Errorf("pane preview lines = %d, want 1", c.UI.PanePreview.Lines)
	}
	if c.UI.PanePreview.Interval != 100*time.Millisecond {
		t.Errorf("pane preview interval = %v, want 100ms", c.UI.PanePreview.Interval)
	}
	if c.Timeouts.IdleToCompletedTimeout != 100*time.Millisecond {
		t.Errorf("idle timeout = %v, want 100ms", c.Timeouts.IdleToCompletedTimeout)
	}
	if c.Timeouts.SpawningTimeout != 100*time.Millisecond {
		t.Errorf("spawning timeout = %v, want 100ms", c.Timeouts.SpawningTimeout)
	}
	if len(c.Warnings) < 6 {
		t.Fatalf("warnings = %v, want validation warnings", c.Warnings)
	}
}

func TestStatusTimeoutSecondsReadAndClamp(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		wantSecs float64
		wantWarn bool
	}{
		{"fractional accepted", "0.5", 0.5, false},
		{"plain accepted", "7", 7, false},
		{"below min clamps", "0.1", 0.5, true},
		{"above max clamps", "25", 10, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			content := "[ui]\n  status_timeout_seconds = " + tc.value + "\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if c.UI.StatusTimeoutSeconds != tc.wantSecs {
				t.Errorf("status_timeout_seconds = %v, want %v", c.UI.StatusTimeoutSeconds, tc.wantSecs)
			}
			want := time.Duration(tc.wantSecs * float64(time.Second))
			if c.UI.StatusTimeout() != want {
				t.Errorf("StatusTimeout() = %v, want %v", c.UI.StatusTimeout(), want)
			}
			gotWarn := false
			for _, w := range c.Warnings {
				if strings.Contains(w, "status_timeout_seconds") {
					gotWarn = true
				}
			}
			if gotWarn != tc.wantWarn {
				t.Errorf("status_timeout_seconds warning = %v, want %v (warnings: %v)", gotWarn, tc.wantWarn, c.Warnings)
			}
		})
	}
}

func TestRoundTripPreservesUserOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.DefaultAgent = "codex"
	c.Sound.Volume = 0.5
	c.Sound.Events["session_completed"] = SoundEvent{File: "custom.wav", Enabled: false}
	c.Agents["claude"] = Agent{Command: "claude --debug", Label: "cl", Color: "#ffffff"}
	c.UI.Editor = "nvim"
	c.UI.PanePreview.Enabled = false
	c.Pruning.KeepDefault = 9

	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.DefaultAgent != "codex" {
		t.Errorf("default agent = %q", got.DefaultAgent)
	}
	if got.Sound.Volume != 0.5 {
		t.Errorf("volume = %f", got.Sound.Volume)
	}
	if got.SoundEventEnabled("session_completed") {
		t.Error("session_completed should remain disabled")
	}
	if got.Sound.Events["session_completed"].File != "custom.wav" {
		t.Errorf("session_completed file = %q", got.Sound.Events["session_completed"].File)
	}
	if got.Agents["claude"].Command != "claude --debug" {
		t.Errorf("claude command = %q", got.Agents["claude"].Command)
	}
	if got.UI.Editor != "nvim" {
		t.Errorf("ui.editor = %q", got.UI.Editor)
	}
	if got.UI.PanePreview.Enabled {
		t.Error("pane preview should remain disabled")
	}
	if got.Pruning.KeepDefault != 9 {
		t.Errorf("keep default = %d", got.Pruning.KeepDefault)
	}
}

func TestNormalizeClampsInMemory(t *testing.T) {
	c := Defaults_()
	c.UI.SidebarWidth = 9999
	c.Sound.Volume = 5
	c.UI.StatusTimeoutSeconds = 0

	Normalize(&c)

	if c.UI.SidebarWidth != MaxSidebarWidth {
		t.Errorf("sidebar width = %d, want clamp to %d", c.UI.SidebarWidth, MaxSidebarWidth)
	}
	if c.Sound.Volume != MaxSoundVolume {
		t.Errorf("volume = %v, want clamp to %v", c.Sound.Volume, MaxSoundVolume)
	}
	if c.UI.StatusTimeoutSeconds != MinStatusTimeoutSeconds {
		t.Errorf("status timeout = %v, want clamp to %v", c.UI.StatusTimeoutSeconds, MinStatusTimeoutSeconds)
	}
	if len(c.Warnings) == 0 {
		t.Error("Normalize should record warnings for clamped values")
	}
}
