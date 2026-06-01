package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestSendPopupViewIsAiry(t *testing.T) {
	popup := NewSendPopup("cleo-project-claude-1", Resolve("catppuccin-mocha"))
	view := popup.View()
	lines := strings.Split(view, "\n")

	if got, want := len(lines), 6; got != want {
		t.Fatalf("send popup line count = %d, want %d\n%s", got, want, view)
	}
	if strings.Contains(view, "Send Message") {
		t.Fatalf("send popup should use the minimal title, got legacy title\n%s", view)
	}
	if !strings.Contains(view, "Quick Message") {
		t.Fatalf("send popup should use the Quick Message title\n%s", view)
	}
	if strings.Trim(ansi.Strip(lines[1]), "│ ") != "" || strings.Trim(ansi.Strip(lines[3]), "│ ") != "" {
		t.Fatalf("send popup should include vertical breathing room\n%s", view)
	}
}

func TestSendPopupLongInputKeepsHintsPinned(t *testing.T) {
	popup := NewSendPopup("cleo-project-claude-1", Resolve("catppuccin-mocha"))
	var model tea.Model = popup
	for _, r := range strings.Repeat("a", 120) {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated
	}

	view := model.(SendPopup).View()
	lines := strings.Split(view, "\n")
	if got, want := len(lines), 6; got != want {
		t.Fatalf("send popup line count = %d, want %d\n%s", got, want, view)
	}
	for i, line := range lines {
		if got, want := lipgloss.Width(line), sendPopupWidth; got != want {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, want, ansi.Strip(line))
		}
	}
	body := ansi.Strip(lines[2])
	if strings.Contains(body, "enter send") || strings.Contains(body, "esc cancel") {
		t.Fatalf("send hints should move below the input, got input row %q", body)
	}
	if strings.Count(body, "a") < 70 {
		t.Fatalf("send input row should reserve most popup width for typing, got %q", body)
	}
	hints := ansi.Strip(lines[4])
	if !strings.Contains(hints, "enter send") || !strings.Contains(hints, "esc cancel") {
		t.Fatalf("send hints should stay visible below long input, got %q", hints)
	}
	if strings.Contains(body, ">") {
		t.Fatalf("send popup should not render the textinput default prompt, got %q", body)
	}
}
