package tmux

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SessionLabel is the identity Cleo paints into a managed tmux session's status
// bar and window list. Session is the tmux target (the full session ID, which is
// what tmux's own #S expands to); the other fields are the parts of that ID worth
// reading, so the status bar can show `project · agent · name` instead of the
// 34-character compound ID that every user's status-left-length truncates.
type SessionLabel struct {
	Session string // tmux session name — the target the options are set on
	Project string
	Agent   string
	Name    string
	Color   string // agent colour, e.g. "#10A37F"; empty falls back to bold-default
}

// Label styling. Cleo owns the label's own colours (dim project, agent colour,
// bright name) but never the surrounding status-style — every option below is set
// on the Cleo session only, so a user's global theme still paints their own
// sessions untouched.
const (
	labelSeparator   = " · "
	labelDimStyle    = "#[fg=colour244]"
	labelSepStyle    = "#[fg=colour240]"
	labelNameStyle   = "#[fg=colour252,bold]"
	labelResetStyle  = "#[default]"
	labelAgentStyle  = "#[bold]"
	maxProjectRunes  = 16 // project IDs are the one unbounded segment; keep the bar short
	maxSegmentRunes  = 24
	labelEllipsis    = "…"
	statusLeftMinLen = 10 // tmux's own default; never shrink a user's budget below it
)

// StatusLeft renders the styled tmux status-left format for the label. Pure so
// the format string is testable without a live server, mirroring attachArgs.
func StatusLeft(l SessionLabel) string {
	var b strings.Builder
	b.WriteString(" ")
	sep := labelSepStyle + labelSeparator
	if project := truncateRunes(l.Project, maxProjectRunes); project != "" {
		b.WriteString(labelDimStyle + project + sep)
	}
	if agent := truncateRunes(l.Agent, maxSegmentRunes); agent != "" {
		b.WriteString(agentStyle(l.Color) + agent + "#[nobold]" + sep)
	}
	b.WriteString(labelNameStyle + truncateRunes(l.displayName(), maxSegmentRunes))
	b.WriteString(labelResetStyle + " ")
	return b.String()
}

// statusLeftLength is the status-left-length to pair with StatusLeft. tmux trims
// status-left to this many characters, and Cleo cannot assume the user's own
// budget (default 10) fits the label. The whole format string length is a safe
// over-estimate: tmux skips #[...] style sequences when it measures, so this can
// only ever be too generous, and an oversized budget costs nothing because the
// string itself is short and the status bar is not padded out to the limit.
func statusLeftLength(format string) int {
	if n := utf8.RuneCountInString(format); n > statusLeftMinLen {
		return n
	}
	return statusLeftMinLen
}

func agentStyle(color string) string {
	if color == "" {
		return labelAgentStyle
	}
	return fmt.Sprintf("#[fg=%s,bold]", color)
}

// displayName falls back to the tmux session name so a label is never blank.
func (l SessionLabel) displayName() string {
	if l.Name != "" {
		return l.Name
	}
	return l.Session
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-1]) + labelEllipsis
}

// sessionLabelCmds builds the ordered tmux argument lists that apply the label —
// session-scoped status options, then the window-scoped ones for windowTarget
// (empty when the window could not be resolved, which skips the window commands).
// Pure, mirroring sendKeysCmds, so the whole option set is testable offline.
func sessionLabelCmds(l SessionLabel) [][]string {
	format := StatusLeft(l)
	return [][]string{
		{"set-option", "-t", l.Session, "status-left", format},
		{"set-option", "-t", l.Session, "status-left-length", fmt.Sprintf("%d", statusLeftLength(format))},
	}
}

// WindowFormat is one window-status format option and the value a window
// currently resolves to for it.
type WindowFormat struct {
	Option string
	Value  string
}

// windowFormatOptions are the two formats that render the window list. Both are
// window options, not session options: tmux resolves them per window and new
// windows inherit the global value, so they have to be set window by window.
var windowFormatOptions = []string{"window-status-format", "window-status-current-format"}

// windowFormatCmds builds the tmux argument lists that hide target's
// automatically-derived window name, skipping formats that need no rewrite (which
// makes re-applying the label on every attach a no-op). Pure.
func windowFormatCmds(target string, formats []WindowFormat) [][]string {
	var cmds [][]string
	for _, wf := range formats {
		hidden := hideAutoWindowNames(wf.Value)
		if hidden == wf.Value {
			continue
		}
		cmds = append(cmds, []string{"set-option", "-w", "-t", target, wf.Option, hidden})
	}
	return cmds
}

// autoRenameCond opens the tmux conditional that tests a window's
// automatic-rename option. Its presence in a format also marks it as already
// rewritten, which keeps re-applying the label on every attach idempotent.
const autoRenameCond = "#{?automatic-rename,"

// hideAutoWindowNames rewrites a window-status format so an automatically named
// window shows only its index — `1  2  3`. A tmux-derived name says nothing about
// the window — Claude Code's versioned binary renders as "2.1.220" — and with the
// session's own name already in the status bar, repeating it per window is noise.
// A window somebody renamed has automatic-rename off, so its name survives.
//
// The rewrite wraps the existing format rather than replacing it, so whatever
// padding, colours, and window flags the user's theme puts around the name stay.
// A format with no #W at all is returned untouched.
func hideAutoWindowNames(format string) string {
	if strings.Contains(format, autoRenameCond) {
		return format // already rewritten
	}
	// Drop the separator along with the name, so an index-only window reads "1"
	// rather than "1:". Formats that write the name some other way fall through to
	// hiding just the name.
	for _, name := range []string{":#W", " #W", "#W"} {
		if strings.Contains(format, name) {
			return strings.ReplaceAll(format, name, autoRenameCond+","+name+"}")
		}
	}
	return format
}

// ApplySessionLabel paints the compact label onto one Cleo-managed session and
// hides the derived window names in its window list. Every option is session- or
// window-scoped, never global. Best-effort by contract: it reports the first
// failure so callers can log it, but a display option that does not take should
// never fail a spawn or an attach — callers ignore the error the way NewSession
// ignores allow-passthrough.
//
// Windows opened after the label was applied keep tmux's naming until the next
// attach re-applies it, because window options cannot be scoped to a session.
func (c *Client) ApplySessionLabel(l SessionLabel) error {
	if l.Session == "" {
		return nil
	}
	cmds := sessionLabelCmds(l)
	for _, window := range c.windowStates(l.Session) {
		cmds = append(cmds, windowFormatCmds(window.Target, window.Formats)...)
	}
	var firstErr error
	for _, args := range cmds {
		if err := c.cmd(args...).Run(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("tmux %s: %w", args[0], err)
		}
	}
	return firstErr
}

// WindowState is one window of a session together with the window-status formats
// it currently resolves to.
type WindowState struct {
	Target  string // window ID (@N), stable under the renumbering `renumber-windows on` does
	Formats []WindowFormat
}

// windowStateSep separates the fields of one list-windows row. A unit separator
// byte cannot appear in a tmux theme, so it never collides with a format value.
const windowStateSep = "\x1f"

// windowStates lists the session's windows with the window-status formats each one
// resolves to. The formats are read through a format expansion, not
// `show-options -w`: an option the window has not overridden is empty in
// show-options but inherits the global value from the user's tmux config, and it
// is that inherited value the rewrite has to start from.
func (c *Client) windowStates(session string) []WindowState {
	fields := append([]string{"#{window_id}"}, formatRefs(windowFormatOptions)...)
	out, err := c.cmd("list-windows", "-t", session, "-F", strings.Join(fields, windowStateSep)).Output()
	if err != nil {
		return nil
	}
	return parseWindowStates(string(out))
}

func formatRefs(options []string) []string {
	refs := make([]string, 0, len(options))
	for _, option := range options {
		refs = append(refs, "#{"+option+"}")
	}
	return refs
}

// parseWindowStates reads the rows windowStates asked for. Pure, so the field
// splitting is testable without a live server. Rows that do not carry a value for
// every option are skipped: a format Cleo cannot read is a format it leaves alone.
func parseWindowStates(out string) []WindowState {
	var states []WindowState
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		fields := strings.Split(line, windowStateSep)
		if len(fields) != len(windowFormatOptions)+1 || fields[0] == "" {
			continue
		}
		state := WindowState{Target: fields[0]}
		for i, option := range windowFormatOptions {
			state.Formats = append(state.Formats, WindowFormat{Option: option, Value: fields[i+1]})
		}
		states = append(states, state)
	}
	return states
}
