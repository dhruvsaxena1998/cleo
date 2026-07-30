package tmux

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func testLabel() SessionLabel {
	return SessionLabel{
		Session: "cleo-pickup-api-codex-lucid-turing",
		Project: "pickup-api",
		Agent:   "codex",
		Name:    "lucid-turing",
		Color:   "#10A37F",
	}
}

func TestStatusLeftShowsProjectAgentAndNameInAgentColour(t *testing.T) {
	got := StatusLeft(testLabel())
	for _, want := range []string{"pickup-api", "codex", "lucid-turing", "fg=#10A37F"} {
		if !strings.Contains(got, want) {
			t.Errorf("status-left %q missing %q", got, want)
		}
	}
	// The compound session ID is what tmux's own #S would show; the point of the
	// label is that it does not appear.
	if strings.Contains(got, "cleo-pickup-api-codex-lucid-turing") {
		t.Errorf("status-left should not repeat the full session ID: %q", got)
	}
}

func TestStatusLeftFallsBackToSessionNameWhenNameEmpty(t *testing.T) {
	l := testLabel()
	l.Name = ""
	// Long session names still get the segment cap, so match the leading part.
	if got := StatusLeft(l); !strings.Contains(got, "cleo-pickup-api-codex") {
		t.Errorf("status-left %q should fall back to the session name", got)
	}
}

func TestStatusLeftUsesBoldWithoutColourWhenAgentHasNoColour(t *testing.T) {
	l := testLabel()
	l.Color = ""
	got := StatusLeft(l)
	if strings.Contains(got, "fg=#") {
		t.Errorf("status-left %q should not invent a colour", got)
	}
	if !strings.Contains(got, "#[bold]codex") {
		t.Errorf("status-left %q should still emphasise the agent", got)
	}
}

func TestStatusLeftTruncatesAnOverlongProject(t *testing.T) {
	l := testLabel()
	l.Project = "a-very-long-project-identifier"
	got := StatusLeft(l)
	if strings.Contains(got, l.Project) {
		t.Errorf("status-left %q should truncate the project segment", got)
	}
	if !strings.Contains(got, labelEllipsis) {
		t.Errorf("status-left %q should mark the truncation", got)
	}
	// The parts that identify the session are never sacrificed for the project.
	if !strings.Contains(got, "lucid-turing") || !strings.Contains(got, "codex") {
		t.Errorf("status-left %q dropped agent or name", got)
	}
}

func TestStatusLeftOmitsEmptySegmentsWithoutStraySeparators(t *testing.T) {
	got := StatusLeft(SessionLabel{Session: "s", Name: "lucid-turing"})
	if strings.Contains(got, labelSeparator) {
		t.Errorf("status-left %q should not emit a separator with nothing to separate", got)
	}
}

func TestStatusLeftLengthCoversTheWholeFormat(t *testing.T) {
	format := StatusLeft(testLabel())
	got := statusLeftLength(format)
	if want := utf8.RuneCountInString(format); got != want {
		t.Errorf("status-left-length = %d, want %d (a safe over-estimate of the visible width)", got, want)
	}
	if got < statusLeftMinLen {
		t.Errorf("status-left-length %d is below tmux's own default %d", got, statusLeftMinLen)
	}
}

func TestStatusLeftLengthNeverShrinksBelowTmuxDefault(t *testing.T) {
	if got := statusLeftLength("x"); got != statusLeftMinLen {
		t.Errorf("status-left-length = %d, want the %d floor", got, statusLeftMinLen)
	}
}

func TestSessionLabelCmdsAreSessionScopedAndNameTheWindow(t *testing.T) {
	cmds := sessionLabelCmds(testLabel(), "@3")
	var kinds []string
	for _, args := range cmds {
		kinds = append(kinds, strings.Join(args[:1], ""))
		for _, arg := range args {
			if arg == "-g" {
				t.Fatalf("label must never set a global option: %v", args)
			}
		}
	}
	want := []string{"set-option", "set-option", "rename-window", "set-option", "set-option"}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", kinds, want)
	}
	if got := cmds[0]; got[2] != "cleo-pickup-api-codex-lucid-turing" || got[3] != "status-left" {
		t.Errorf("first command targets the wrong session/option: %v", got)
	}
	if got := cmds[2]; got[2] != "@3" || got[3] != "lucid-turing" {
		t.Errorf("rename-window = %v, want window @3 named lucid-turing", got)
	}
	for _, args := range cmds[3:] {
		if args[2] != "-t" || args[1] != "-w" || args[3] != "@3" {
			t.Errorf("window option is not window-scoped on @3: %v", args)
		}
	}
	if !hasOption(cmds, "automatic-rename", "off") || !hasOption(cmds, "allow-rename", "off") {
		t.Errorf("label must take renaming away from the agent: %v", cmds)
	}
}

func TestSessionLabelCmdsSkipWindowCommandsWithoutATarget(t *testing.T) {
	cmds := sessionLabelCmds(testLabel(), "")
	if len(cmds) != 2 {
		t.Fatalf("commands = %v, want only the two session options", cmds)
	}
}

func hasOption(cmds [][]string, option, value string) bool {
	for _, args := range cmds {
		for i, arg := range args {
			if arg == option && i+1 < len(args) && args[i+1] == value {
				return true
			}
		}
	}
	return false
}

func TestApplySessionLabelRelabelsOnlyTheTargetSession(t *testing.T) {
	c := newTestClient(t)
	label := testLabel()
	if err := c.NewSession(NewSessionOpts{Name: label.Session, Cwd: "/tmp", Cmd: "sleep 60"}); err != nil {
		t.Fatal(err)
	}
	if err := c.NewSession(NewSessionOpts{Name: "someone-elses-session", Cwd: "/tmp", Cmd: "sleep 60"}); err != nil {
		t.Fatal(err)
	}
	// A tight global budget is the usual cause of the truncated status bar.
	if err := c.cmd("set-option", "-g", "status-left-length", "10").Run(); err != nil {
		t.Fatal(err)
	}

	if err := c.ApplySessionLabel(label); err != nil {
		t.Fatal(err)
	}

	rendered := display(t, c, label.Session, "#{T;=/#{status-left-length}:status-left}")
	for _, want := range []string{"pickup-api", "codex", "lucid-turing"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered status-left %q lost %q to truncation", rendered, want)
		}
	}
	if got := display(t, c, label.Session, "#{window_name}"); got != "lucid-turing" {
		t.Errorf("window name = %q, want lucid-turing", got)
	}
	if got := display(t, c, label.Session, "#{automatic-rename}"); got != "0" {
		t.Errorf("automatic-rename = %q, want off", got)
	}
	// The user's other sessions keep their own status bar.
	if got := display(t, c, "someone-elses-session", "#{status-left-length}"); got != "10" {
		t.Errorf("unmanaged session status-left-length = %q, want the global 10", got)
	}
	if got := display(t, c, "someone-elses-session", "#{status-left}"); strings.Contains(got, "lucid-turing") {
		t.Errorf("unmanaged session picked up the Cleo label: %q", got)
	}
}

func TestApplySessionLabelWithoutASessionIsANoop(t *testing.T) {
	c := newTestClient(t)
	if err := c.ApplySessionLabel(SessionLabel{Name: "lucid-turing"}); err != nil {
		t.Fatalf("empty session should be a no-op, got %v", err)
	}
}

func display(t *testing.T, c *Client, target, format string) string {
	t.Helper()
	out, err := c.cmd("display-message", "-p", "-t", target, format).Output()
	if err != nil {
		t.Fatalf("display-message %q: %v", format, err)
	}
	return strings.TrimSpace(string(out))
}
