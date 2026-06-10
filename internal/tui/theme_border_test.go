package tui

import (
	"strings"
	"testing"

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

// TestThemeSwitchBorderColors verifies that border colors change between themes.
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
