package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// typeIntoFinder drives the finder's Update one key at a time, delivering space
// as tea.KeySpace exactly the way Bubble Tea does — that is the keypress the
// hand-rolled finder used to drop.
func typeIntoFinder(p FinderPopup, s string) FinderPopup {
	var model tea.Model = p
	for _, r := range s {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		if r == ' ' {
			msg.Type = tea.KeySpace
		}
		updated, _ := model.Update(msg)
		model = updated
	}
	return model.(FinderPopup)
}

func finderTestSessions() []state.Session {
	return []state.Session{
		{ID: "a1", Name: "alpha beta", ProjectID: "proj", Agent: "claude", State: state.Running},
		{ID: "b1", Name: "gamma", ProjectID: "proj", Agent: "claude", State: state.Running},
	}
}

// TestFinderAcceptsSpace pins the bug fix: a space in the query must reach the
// field (previously KeySpace was dropped) and must filter as a literal space.
func TestFinderAcceptsSpace(t *testing.T) {
	c := newTestCtx(t)
	p := NewFinderPopup(c, Resolve("catppuccin-mocha"), finderTestSessions())

	p = typeIntoFinder(p, "alpha beta")

	if got := p.input.Value(); got != "alpha beta" {
		t.Fatalf("query = %q, want %q (space was dropped)", got, "alpha beta")
	}
	if got := p.matchCount(); got != 1 {
		t.Fatalf("matchCount = %d, want 1 after spaced query", got)
	}
	sel, ok := p.selected()
	if !ok || sel.ID != "a1" {
		t.Fatalf("selected = %+v ok=%v, want session a1", sel, ok)
	}
}

// TestFinderCtrlUClearsField confirms the textinput's standard editing keys are
// now live: ctrl+u (delete-to-start) is the terminal-native "clear field".
func TestFinderCtrlUClearsField(t *testing.T) {
	c := newTestCtx(t)
	p := NewFinderPopup(c, Resolve("catppuccin-mocha"), finderTestSessions())

	p = typeIntoFinder(p, "gamma")
	if p.matchCount() != 1 {
		t.Fatalf("matchCount = %d, want 1 after typing gamma", p.matchCount())
	}

	model, _ := tea.Model(p).Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	p = model.(FinderPopup)

	if got := p.input.Value(); got != "" {
		t.Fatalf("ctrl+u left query = %q, want empty", got)
	}
	if got := p.matchCount(); got != 2 {
		t.Fatalf("matchCount = %d, want 2 (all sessions) after clearing", got)
	}
	if p.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after query change", p.cursor)
	}
}
