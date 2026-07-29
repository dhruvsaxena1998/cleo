package tmux

import (
	"reflect"
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

func TestSessionLabelCmdsAreSessionScoped(t *testing.T) {
	cmds := sessionLabelCmds(testLabel())
	if len(cmds) != 2 {
		t.Fatalf("commands = %v, want status-left and status-left-length", cmds)
	}
	for _, args := range cmds {
		if args[0] != "set-option" || args[1] != "-t" || args[2] != "cleo-pickup-api-codex-lucid-turing" {
			t.Errorf("command is not scoped to the Cleo session: %v", args)
		}
		for _, arg := range args {
			if arg == "-g" {
				t.Fatalf("label must never set a global option: %v", args)
			}
		}
	}
	if cmds[0][3] != "status-left" || cmds[1][3] != "status-left-length" {
		t.Errorf("commands set the wrong options: %v", cmds)
	}
}

func TestWindowFormatCmdsRewriteOnlyWhatNeedsIt(t *testing.T) {
	cmds := windowFormatCmds("@3", []WindowFormat{
		{Option: "window-status-format", Value: " #I:#W "},
		{Option: "window-status-current-format", Value: " #I "}, // nothing to hide
	})
	if len(cmds) != 1 {
		t.Fatalf("commands = %v, want only the format that has a name to hide", cmds)
	}
	got := cmds[0]
	if got[0] != "set-option" || got[1] != "-w" || got[2] != "-t" || got[3] != "@3" {
		t.Errorf("command is not window-scoped on @3: %v", got)
	}
	if got[4] != "window-status-format" || !strings.Contains(got[5], autoRenameCond) {
		t.Errorf("command = %v, want a rewritten window-status-format", got)
	}
	for _, arg := range got {
		if arg == "-g" {
			t.Fatalf("label must never set a global option: %v", got)
		}
	}
}

func TestHideAutoWindowNamesKeepsTheUsersStylingAndDropsTheSeparator(t *testing.T) {
	cases := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "tmux default",
			format: " #I:#W ",
			want:   " #I#{?automatic-rename,,:#W} ",
		},
		{
			name:   "styled current-window format",
			format: "#[bold,fg=colour234,bg=colour39] #I:#W #[default]",
			want:   "#[bold,fg=colour234,bg=colour39] #I#{?automatic-rename,,:#W} #[default]",
		},
		{
			name:   "flags kept outside the conditional",
			format: "#I:#W#{?window_flags,#{window_flags},}",
			want:   "#I#{?automatic-rename,,:#W}#{?window_flags,#{window_flags},}",
		},
		{
			name:   "space separated name",
			format: " #I #W ",
			want:   " #I#{?automatic-rename,, #W} ",
		},
		{
			name:   "no name to hide",
			format: " #I ",
			want:   " #I ",
		},
		{
			name:   "empty",
			format: "",
			want:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hideAutoWindowNames(tc.format); got != tc.want {
				t.Errorf("hideAutoWindowNames(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

func TestHideAutoWindowNamesIsIdempotentSoReattachingDoesNotNest(t *testing.T) {
	once := hideAutoWindowNames(" #I:#W ")
	if twice := hideAutoWindowNames(once); twice != once {
		t.Errorf("second rewrite changed the format: %q -> %q", once, twice)
	}
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
	// Two windows: the agent's, named by tmux after whatever it runs, and one the
	// user opened and named.
	if err := c.cmd("new-window", "-d", "-t", label.Session).Run(); err != nil {
		t.Fatal(err)
	}
	if err := c.cmd("rename-window", "-t", label.Session+":2", "my-notes").Run(); err != nil {
		t.Fatal(err)
	}
	autoName := display(t, c, label.Session+":1", "#{window_name}")

	if err := c.ApplySessionLabel(label); err != nil {
		t.Fatal(err)
	}

	rendered := display(t, c, label.Session, "#{T;=/#{status-left-length}:status-left}")
	for _, want := range []string{"pickup-api", "codex", "lucid-turing"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered status-left %q lost %q to truncation", rendered, want)
		}
	}
	// The window list shows bare indexes, except for the window the user named.
	// Cleo never renames a window itself.
	windows := display(t, c, label.Session, "#{W:[#{T:window-status-format}]}")
	if strings.Contains(windows, autoName) {
		t.Errorf("window list %q still shows the derived name %q", windows, autoName)
	}
	if !strings.Contains(windows, "2:my-notes") {
		t.Errorf("window list %q dropped the name the user gave window 2", windows)
	}
	if got := display(t, c, label.Session+":1", "#{window_name}"); got != autoName {
		t.Errorf("window name = %q, want tmux's own %q left alone", got, autoName)
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

func TestParseWindowStatesKeepsFormatsPerWindowAndSkipsShortRows(t *testing.T) {
	out := strings.Join([]string{
		"@0" + windowStateSep + " #I:#W " + windowStateSep + "#[bold] #I:#W #[default]",
		"@1" + windowStateSep + " #I " + windowStateSep + " #I ",
		"@2" + windowStateSep + "missing the second format", // ignored
		"", // ignored
	}, "\n")

	got := parseWindowStates(out)
	if len(got) != 2 {
		t.Fatalf("states = %#v, want the two complete rows", got)
	}
	if got[0].Target != "@0" || got[1].Target != "@1" {
		t.Errorf("targets = %q, %q", got[0].Target, got[1].Target)
	}
	want := []WindowFormat{
		{Option: "window-status-format", Value: " #I:#W "},
		{Option: "window-status-current-format", Value: "#[bold] #I:#W #[default]"},
	}
	if !reflect.DeepEqual(got[0].Formats, want) {
		t.Errorf("formats = %#v, want %#v", got[0].Formats, want)
	}
}
