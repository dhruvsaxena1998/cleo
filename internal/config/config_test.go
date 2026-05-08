package config

import (
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
	if !c.Sound.Enabled {
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
	if err := Save(path, Config{
		Sound: Sound{
			Enabled: true,
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
