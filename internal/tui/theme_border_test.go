package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestPopupBorderHasExplicitBackground verifies that popup borders have an
// explicit background color (the theme's Base), which prevents them from
// losing their background during overlay splicing.
func TestPopupBorderHasExplicitBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	mocha := Resolve("catppuccin-mocha")
	gruvbox := Resolve("gruvbox-dark")

	mochaBorder := popupBorderStyle(mocha).Render("│")
	gruvboxBorder := popupBorderStyle(gruvbox).Render("│")

	t.Logf("mocha popup border: %q", mochaBorder)
	t.Logf("gruvbox popup border: %q", gruvboxBorder)

	// Both should have explicit background (48;2;...)
	if !strings.Contains(mochaBorder, "48;2;30;30;46") {
		t.Error("mocha popup border should have mocha Base background")
	}
	if !strings.Contains(gruvboxBorder, "48;2;40;40;40") {
		t.Error("gruvbox popup border should have gruvbox Base background")
	}
	// Foreground should differ (Mauve changes)
	if mochaBorder == gruvboxBorder {
		t.Error("popup borders should differ between themes")
	}
}

// TestPopupFillInheritsBorderBackground verifies that when a popup's fill
// style has no explicit background, drawFrame inherits the border's background.
func TestPopupFillInheritsBorderBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	mocha := Resolve("catppuccin-mocha")

	// A popup frame with no explicit fill (zero value)
	popup := drawFrame(frameSpec{
		Width:    40,
		Title:    "Test",
		Hint:     "hint",
		Border:   popupBorderStyle(mocha),
		Sections: [][]string{{"content"}},
	})

	t.Logf("popup frame: %q", popup)

	// The content area should have the Base background (inherited from border)
	if !strings.Contains(popup, "48;2;30;30;46") {
		t.Error("popup content area should have mocha Base background (inherited from border)")
	}
}

// TestThemeSwitchBorderColors verifies that border colors change between themes
// at the dashboard level (panel borders).
func TestThemeSwitchBorderColors(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	c := newTestCtx(t)
	m := New(c)
	m.width, m.height = 120, 40

	view1 := m.View()
	if !strings.Contains(view1, "38;2;69;71;89") {
		t.Fatal("Initial view should have mocha border color")
	}

	// Switch theme
	c.Config.UI.Theme = "gruvbox-dark"
	updatedM, _ := m.Update(SettingsChanged{Config: c.Config})
	m = updatedM.(Model)

	view2 := m.View()
	if !strings.Contains(view2, "38;2;80;73;69") {
		t.Error("After theme switch, view should have gruvbox border color")
	}
	if strings.Contains(view2, "38;2;69;71;89") {
		t.Error("After theme switch, view should NOT have mocha border color")
	}
}

// TestPopupUsesGruvboxColorsOnGruvboxTheme verifies that a popup opened on a
// gruvbox-themed model uses gruvbox's Mauve and Base, not catppuccin's.
// This catches the bug where popup border colours stay on the previous theme
// after a theme switch.
func TestPopupUsesGruvboxColorsOnGruvboxTheme(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	c := newTestCtxWithConfig(t, "[ui]\n  theme = \"gruvbox-dark\"\n")
	m := New(c)
	m.width, m.height = 120, 40

	if m.theme.Name != "gruvbox-dark" {
		t.Fatalf("expected model theme gruvbox-dark, got %s", m.theme.Name)
	}

	// Open a help popup - should use gruvbox colours
	m.popup = NewHelpPopup(m.theme, m.ctx.Config.Keymap, m.ctx.Config.Tmux.DetachKey)
	m.mode = ModePopup

	popupView := m.popup.View()

	// gruvbox.Mauve = #b16286 → RGB(177,97,134)
	gruvboxMauveFG := "38;2;177;97;134"
	// catppuccin.Mauve = #cba6f7 → RGB(203,166,247)
	mochaMauveFG := "38;2;203;166;247"
	// gruvbox.Base = #282828 → RGB(40,40,40)
	gruvboxBaseBG := "48;2;40;40;40"
	// catppuccin.Base = #1e1e2e → RGB(30,30,46)
	mochaBaseBG := "48;2;30;30;46"

	if !strings.Contains(popupView, gruvboxMauveFG) {
		t.Errorf("popup should contain gruvbox Mauve (%s)", gruvboxMauveFG)
	}
	if strings.Contains(popupView, mochaMauveFG) {
		t.Errorf("popup should NOT contain catppuccin Mauve (%s)", mochaMauveFG)
	}
	if !strings.Contains(popupView, gruvboxBaseBG) {
		t.Errorf("popup should contain gruvbox Base (%s)", gruvboxBaseBG)
	}
	if strings.Contains(popupView, mochaBaseBG) {
		t.Errorf("popup should NOT contain catppuccin Base (%s)", mochaBaseBG)
	}

	// Also verify the overlay-spliced full view uses gruvbox colours
	fullView := m.View()
	if !strings.Contains(fullView, gruvboxMauveFG) {
		t.Errorf("full view should contain gruvbox Mauve (%s)", gruvboxMauveFG)
	}
	if strings.Contains(fullView, mochaMauveFG) {
		t.Errorf("full view should NOT contain catppuccin Mauve (%s)", mochaMauveFG)
	}
}

// TestApplyThemePropagatesToPopup verifies that applyTheme updates the theme
// of any open popup, not just the model's own theme. This catches the bug
// where a live theme switch in settings leaves non-settings popups with the
// old palette.
func TestApplyThemePropagatesToPopup(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	c := newTestCtx(t) // starts with default catppuccin-mocha
	m := New(c)
	m.width, m.height = 120, 40

	// Open a help popup - starts with catppuccin-mocha theme
	m.popup = NewHelpPopup(m.theme, m.ctx.Config.Keymap, m.ctx.Config.Tmux.DetachKey)
	m.mode = ModePopup

	// Verify initial popup theme is catppuccin
	hp := m.popup.(HelpPopup)
	if hp.theme.Name != "catppuccin-mocha" {
		t.Fatalf("initial popup theme should be catppuccin-mocha, got %s", hp.theme.Name)
	}

	// Change theme to gruvbox-dark via applyTheme (simulates SettingsChanged path)
	m.applyTheme("gruvbox-dark")

	// Model's theme should be gruvbox
	if m.theme.Name != "gruvbox-dark" {
		t.Errorf("model theme should be gruvbox-dark after applyTheme, got %s", m.theme.Name)
	}

	// Popup's theme should also be gruvbox
	hp = m.popup.(HelpPopup)
	if hp.theme.Name != "gruvbox-dark" {
		t.Errorf("popup theme should be gruvbox-dark after applyTheme, got %s", hp.theme.Name)
	}

	// Popup's View should reflect the new theme colours
	popupView := m.popup.View()
	gruvboxMauveFG := "38;2;177;97;134"
	mochaMauveFG := "38;2;203;166;247"
	if !strings.Contains(popupView, gruvboxMauveFG) {
		t.Errorf("popup should use gruvbox Mauve after theme change, missing %s", gruvboxMauveFG)
	}
	if strings.Contains(popupView, mochaMauveFG) {
		t.Errorf("popup should NOT use catppuccin Mauve after theme change, found %s", mochaMauveFG)
	}
}

// TestSettingsPopupLivePreviewThemeChange verifies the real UI flow: pressing
// right arrow on the theme field updates both the popup and model themes, and
// both use the new theme's colour palette in the rendered output.
func TestSettingsPopupLivePreviewThemeChange(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	c := newTestCtx(t) // starts with default catppuccin-mocha
	m := New(c)
	m.width, m.height = 120, 40

	// Open settings popup (starts on catppuccin-mocha)
	m, _ = m.openSettingsPopup()

	sp, ok := m.popup.(SettingsPopup)
	if !ok {
		t.Fatal("expected settings popup")
	}
	if sp.theme.Name != "catppuccin-mocha" {
		t.Fatalf("initial popup theme should be catppuccin-mocha, got %s", sp.theme.Name)
	}

	// Navigate to the theme field
	themeIdx := -1
	for i, f := range sp.fields {
		if f.label == "theme" {
			themeIdx = i
			break
		}
	}
	if themeIdx == -1 {
		t.Fatal("could not find theme field")
	}
	sp.cursor = themeIdx
	m.popup = sp

	// Press right arrow to cycle theme (catppuccin-mocha → gruvbox-dark)
	updatedPopup, cmd := m.popup.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.popup = updatedPopup

	// Popup's internal theme should now be gruvbox
	settingsPopup := m.popup.(SettingsPopup)
	if settingsPopup.theme.Name != "gruvbox-dark" {
		t.Errorf("after right-arrow, popup theme should be gruvbox-dark, got %s", settingsPopup.theme.Name)
	}

	// Extract and process the SettingsChanged message
	if cmd == nil {
		t.Fatal("expected a command after theme change")
	}
	msg := cmd()
	sc, ok := msg.(SettingsChanged)
	if !ok {
		t.Fatalf("expected SettingsChanged, got %T", msg)
	}

	// Process SettingsChanged through the model
	updatedModel, _ := m.Update(sc)
	m = updatedModel.(Model)

	// Model's theme should be gruvbox
	if m.theme.Name != "gruvbox-dark" {
		t.Errorf("after SettingsChanged, model theme should be gruvbox-dark, got %s", m.theme.Name)
	}

	// Popup's View should use gruvbox colours
	popupView := m.popup.View()
	gruvboxMauveFG := "38;2;177;97;134"
	mochaMauveFG := "38;2;203;166;247"
	gruvboxBaseBG := "48;2;40;40;40"

	if !strings.Contains(popupView, gruvboxMauveFG) {
		t.Errorf("settings popup should contain gruvbox Mauve (%s)", gruvboxMauveFG)
	}
	if strings.Contains(popupView, mochaMauveFG) {
		t.Errorf("settings popup should NOT contain catppuccin Mauve (%s)", mochaMauveFG)
	}
	if !strings.Contains(popupView, gruvboxBaseBG) {
		t.Errorf("settings popup should contain gruvbox Base (%s)", gruvboxBaseBG)
	}
}

// TestPopupColorsAcrossColorProfiles verifies that popup borders produce
// different output for different themes across all color profiles. In ANSI
// 16-color mode the themes are indistinguishable (both map Mauve to bright
// magenta), but TrueColor and ANSI256 should differentiate them.
func TestPopupColorsAcrossColorProfiles(t *testing.T) {
	profiles := []struct {
		name    string
		profile termenv.Profile
		sameOK  bool // true if identical output across themes is expected
	}{
		{"TrueColor", termenv.TrueColor, false},
		{"ANSI256", termenv.ANSI256, false},
		{"ANSI", termenv.ANSI, true},
	}

	for _, pp := range profiles {
		t.Run(pp.name, func(t *testing.T) {
			lipgloss.SetColorProfile(pp.profile)

			mocha := Resolve("catppuccin-mocha")
			gruvbox := Resolve("gruvbox-dark")

			mochaBorder := popupBorderStyle(mocha).Render("│")
			gruvboxBorder := popupBorderStyle(gruvbox).Render("│")

			t.Logf("%s mocha border: %q", pp.name, mochaBorder)
			t.Logf("%s gruvbox border: %q", pp.name, gruvboxBorder)

			if mochaBorder == gruvboxBorder && !pp.sameOK {
				t.Errorf("%s: mocha and gruvbox borders should differ", pp.name)
			}
		})
	}
}