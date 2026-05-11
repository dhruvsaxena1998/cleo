package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Defaults.DefaultAgent != "claude" {
		t.Errorf("default agent: %q", c.Defaults.DefaultAgent)
	}
	if c.Sound.Enabled == nil || !*c.Sound.Enabled {
		t.Errorf("sound default disabled")
	}
	if c.Sound.Volume != 0.7 {
		t.Errorf("volume: %f", c.Sound.Volume)
	}
	if !c.Sound.EventEnabled["session_completed"] {
		t.Errorf("session_completed sound should default enabled")
	}
	if c.Agents["claude"].Label != "cl" {
		t.Errorf("claude label: %q", c.Agents["claude"].Label)
	}
	if c.Agents["claude"].Color != "#CC785C" {
		t.Errorf("claude color: %q", c.Agents["claude"].Color)
	}
	if c.UI.PanePreviewInterval != 1500*time.Millisecond {
		t.Errorf("interval: %v", c.UI.PanePreviewInterval)
	}
	if c.Retention.HintThreshold != 6 {
		t.Errorf("hint threshold: %d", c.Retention.HintThreshold)
	}
}

func TestPartialSoundEventEnabledMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	enabled := true
	if err := Save(path, Config{
		Sound: Sound{
			Enabled: &enabled,
			Volume:  0.5,
			Events: map[string]string{
				"session_completed": "done.wav",
			},
			EventEnabled: map[string]bool{
				"session_completed": false,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.SoundEventEnabled("session_completed") {
		t.Errorf("session_completed should remain disabled")
	}
	if !c.SoundEventEnabled("session_start") {
		t.Errorf("missing event toggle should default enabled")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Sound.Volume = 0.5
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Sound.Volume != 0.5 {
		t.Errorf("round trip lost volume: %f", c2.Sound.Volume)
	}
}

func TestSoundEnabledDefaultsToTrueWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// [sound] section without enabled key — toml decodes Enabled as nil (*bool)
	if err := os.WriteFile(path, []byte("[sound]\nvolume = 0.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.SoundEventEnabled("needs_input") {
		t.Error("sound should default to enabled when 'enabled' key is absent from config")
	}
}

func TestSoundEnabledFalseWhenExplicitlySet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("[sound]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.SoundEventEnabled("needs_input") {
		t.Error("sound should be disabled when enabled = false is set explicitly")
	}
}

func TestUISettingsPreservedWhenSidebarWidthAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// User sets event_log_lines but not sidebar_width.
	content := "[ui]\nevent_log_lines = 50\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.UI.EventLogLines != 50 {
		t.Errorf("event_log_lines = 50 should be preserved, got %d", c.UI.EventLogLines)
	}
	if c.UI.SidebarWidth == 0 {
		t.Error("sidebar_width should be filled from defaults when absent")
	}
}
