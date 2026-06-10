package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// runsEnableMouse reports whether cmd, when run, produces bubbletea's
// enable-mouse message — the exact effect of the startup WithMouseCellMotion
// option that ExecProcess tears down and never restores on its own.
func runsEnableMouse(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	return cmd() == tea.EnableMouseCellMotion()
}

// TestResumeMouseCmdGatedOnConfig locks in the gating: the re-arm fires only
// when the user has mouse support enabled, and is a no-op for anyone who opted
// out (so we never enable mouse — and break native text selection — behind
// their back).
func TestResumeMouseCmdGatedOnConfig(t *testing.T) {
	on := New(newTestCtx(t)) // Mouse.Enabled defaults true
	if !runsEnableMouse(on.resumeMouseCmd()) {
		t.Error("mouse enabled: resumeMouseCmd should re-enable mouse tracking")
	}

	off := New(newTestCtxWithConfig(t, "[ui.mouse]\n  enabled = false\n"))
	if cmd := off.resumeMouseCmd(); cmd != nil {
		t.Errorf("mouse disabled: resumeMouseCmd should be a no-op, got %#v", cmd())
	}
}

// TestAttachExitReenablesMouse is the regression for the reported bug: after a
// double-click attach and detach, the dashboard went mouse-dead because
// ExecProcess disables mouse and bubbletea's RestoreTerminal never re-arms it.
// Handling attachExitedMsg must re-enable mouse on resume — and stay a no-op
// when mouse is off.
func TestAttachExitReenablesMouse(t *testing.T) {
	m := New(newTestCtx(t))
	if _, cmd := m.Update(attachExitedMsg{}); !runsEnableMouse(cmd) {
		t.Error("attach exit should re-enable mouse tracking on resume")
	}

	off := New(newTestCtxWithConfig(t, "[ui.mouse]\n  enabled = false\n"))
	if _, cmd := off.Update(attachExitedMsg{}); cmd != nil {
		t.Errorf("attach exit with mouse off should not touch mouse, got %#v", cmd())
	}
}

// TestEditorExitReenablesMouse covers the second ExecProcess site: a terminal
// editor (vim/nano) tears mouse down the same way, so a clean editorFinishedMsg
// must also re-arm it on resume.
func TestEditorExitReenablesMouse(t *testing.T) {
	m := New(newTestCtx(t))
	if _, cmd := m.Update(editorFinishedMsg{}); !runsEnableMouse(cmd) {
		t.Error("editor exit should re-enable mouse tracking on resume")
	}
}
