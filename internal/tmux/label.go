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
func sessionLabelCmds(l SessionLabel, windowTarget string) [][]string {
	format := StatusLeft(l)
	cmds := [][]string{
		{"set-option", "-t", l.Session, "status-left", format},
		{"set-option", "-t", l.Session, "status-left-length", fmt.Sprintf("%d", statusLeftLength(format))},
	}
	if windowTarget == "" {
		return cmds
	}
	// Agents rename their own window — Claude Code's versioned binary shows up as
	// "2.1.220" under automatic-rename, Codex sets "codex-window" — so name the
	// window after the session and take renaming away from the pane.
	return append(cmds,
		[]string{"rename-window", "-t", windowTarget, l.displayName()},
		[]string{"set-option", "-w", "-t", windowTarget, "automatic-rename", "off"},
		[]string{"set-option", "-w", "-t", windowTarget, "allow-rename", "off"},
	)
}

// ApplySessionLabel paints the compact label onto one Cleo-managed session. Every
// option is session- or window-scoped, never global. Best-effort by contract: it
// reports the first failure so callers can log it, but a status option that does
// not take should never fail a spawn or an attach — callers ignore the error the
// way NewSession ignores allow-passthrough.
func (c *Client) ApplySessionLabel(l SessionLabel) error {
	if l.Session == "" {
		return nil
	}
	target, err := c.firstWindowID(l.Session)
	if err != nil {
		target = "" // no window to rename; still fix the status bar
	}
	var firstErr error
	for _, args := range sessionLabelCmds(l, target) {
		if err := c.cmd(args...).Run(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("tmux %s: %w", args[0], err)
		}
	}
	return firstErr
}

// firstWindowID returns the window_id of the session's lowest-indexed window —
// the one tmux created with the session, so the one running the agent. Targeting
// by ID rather than index keeps the rename off whatever window the user happens
// to have current (a shell they opened later) and survives a base-index of 0 or 1.
func (c *Client) firstWindowID(session string) (string, error) {
	out, err := c.cmd("list-windows", "-t", session, "-F", "#{window_id}").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("tmux list-windows: no windows in %q", session)
}
