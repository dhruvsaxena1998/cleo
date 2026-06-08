package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/dhruvsaxena1998/cleo/internal/config"
)

func newTestSettings(cfg config.Config) SettingsPopup {
	return NewSettingsPopup(cfg, Resolve(cfg.UI.Theme), []string{"claude", "codex"})
}

func fieldIndexSec(p SettingsPopup, section, label string) int {
	for i, f := range p.fields {
		if f.section == section && f.label == label {
			return i
		}
	}
	return -1
}

func settingsKey(s string) tea.KeyMsg {
	switch s {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// step drives one key through the popup and returns the updated popup plus any
// message its command emitted (nil if none).
func step(p SettingsPopup, k tea.KeyMsg) (SettingsPopup, tea.Msg) {
	updated, cmd := p.Update(k)
	var msg tea.Msg
	if cmd != nil {
		msg = cmd()
	}
	return updated.(SettingsPopup), msg
}

func TestSettingsThemeCyclesAndPreviewsLive(t *testing.T) {
	p := newTestSettings(config.Defaults_()) // theme = catppuccin-mocha (sorts first)
	p.cursor = fieldIndexSec(p, "Appearance", "theme")

	p, msg := step(p, settingsKey("right"))

	changed, ok := msg.(SettingsChanged)
	if !ok {
		t.Fatalf("expected SettingsChanged, got %T", msg)
	}
	if changed.Config.UI.Theme == "catppuccin-mocha" {
		t.Fatalf("theme should have advanced off the default, got %q", changed.Config.UI.Theme)
	}
	if p.draft.UI.Theme != changed.Config.UI.Theme {
		t.Fatalf("draft theme %q != emitted theme %q", p.draft.UI.Theme, changed.Config.UI.Theme)
	}
	// The popup must re-resolve its own theme so its borders track the preview.
	if p.theme.Name != p.draft.UI.Theme {
		t.Fatalf("popup theme = %q, want %q", p.theme.Name, p.draft.UI.Theme)
	}
}

func TestSettingsThemeWrapsAround(t *testing.T) {
	names := ThemeNames()
	cfg := config.Defaults_()
	cfg.UI.Theme = names[len(names)-1] // last theme
	p := newTestSettings(cfg)
	p.cursor = fieldIndexSec(p, "Appearance", "theme")

	_, msg := step(p, settingsKey("right"))
	changed := msg.(SettingsChanged)
	if changed.Config.UI.Theme != names[0] {
		t.Fatalf("theme should wrap to %q, got %q", names[0], changed.Config.UI.Theme)
	}
}

func TestSettingsIntClampsAtMin(t *testing.T) {
	cfg := config.Defaults_()
	cfg.UI.SidebarWidth = config.MinSidebarWidth + 1
	p := newTestSettings(cfg)
	p.cursor = fieldIndexSec(p, "Appearance", "sidebar width")

	p, _ = step(p, settingsKey("left"))   // min+1 -> min
	_, msg := step(p, settingsKey("left")) // min -> clamped at min
	changed := msg.(SettingsChanged)
	if changed.Config.UI.SidebarWidth != config.MinSidebarWidth {
		t.Fatalf("sidebar width = %d, want clamp at %d", changed.Config.UI.SidebarWidth, config.MinSidebarWidth)
	}
}

func TestSettingsBoolToggles(t *testing.T) {
	cfg := config.Defaults_() // Sound.Enabled = true
	p := newTestSettings(cfg)
	p.cursor = fieldIndexSec(p, "Sound", "enabled")

	_, msg := step(p, settingsKey(" "))
	changed := msg.(SettingsChanged)
	if changed.Config.Sound.Enabled {
		t.Fatalf("sound.enabled should toggle off, got true")
	}
}

func TestSettingsFloatSnapsToStepGrid(t *testing.T) {
	cfg := config.Defaults_() // Sound.Volume = 0.7, step 0.05
	p := newTestSettings(cfg)
	p.cursor = fieldIndexSec(p, "Sound", "volume")

	_, msg := step(p, settingsKey("right"))
	changed := msg.(SettingsChanged)
	if got := changed.Config.Sound.Volume; got < 0.749 || got > 0.751 {
		t.Fatalf("volume = %v, want ~0.75 (snapped to step grid)", got)
	}
}

func TestSettingsEditorFieldTyping(t *testing.T) {
	p := newTestSettings(config.Defaults_()) // UI.Editor = ""
	// Navigate from default-agent (0) down to the editor string field, which
	// focuses the text input.
	p.cursor = fieldIndexSec(p, "General", "default agent")
	for p.cursor != fieldIndexSec(p, "General", "editor") {
		p, _ = step(p, settingsKey("down"))
	}
	for _, r := range "vim" {
		p, _ = step(p, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if p.draft.UI.Editor != "vim" {
		t.Fatalf("editor = %q, want %q", p.draft.UI.Editor, "vim")
	}
}

func TestSettingsViewHasConsistentWidth(t *testing.T) {
	p := newTestSettings(config.Defaults_())
	for _, line := range strings.Split(p.View(), "\n") {
		if got := lipgloss.Width(line); got != 60 {
			t.Fatalf("line width = %d, want 60: %q", got, ansi.Strip(line))
		}
	}
}

func TestSettingsEnterSaves(t *testing.T) {
	cfg := config.Defaults_()
	p := newTestSettings(cfg)
	// Edit the theme, then save.
	p.cursor = fieldIndexSec(p, "Appearance", "theme")
	p, _ = step(p, settingsKey("right"))
	_, msg := step(p, settingsKey("enter"))

	saved, ok := msg.(SettingsSaved)
	if !ok {
		t.Fatalf("expected SettingsSaved, got %T", msg)
	}
	if saved.Config.UI.Theme != p.draft.UI.Theme {
		t.Fatalf("saved theme %q != draft theme %q", saved.Config.UI.Theme, p.draft.UI.Theme)
	}
}
