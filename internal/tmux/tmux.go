package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Client struct{ socket string }

// NewClient with a custom socket name; pass "" for default tmux.
func NewClient(socket string) *Client { return &Client{socket: socket} }

func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

type NewSessionOpts struct {
	Name string
	Cwd  string
	Cmd  string
	Env  map[string]string
}

func (c *Client) cmd(args ...string) *exec.Cmd {
	full := []string{}
	if c.socket != "" {
		full = append(full, "-L", c.socket)
	}
	full = append(full, args...)
	return exec.Command("tmux", full...)
}

func (c *Client) BindDetachKey(detachKey string) error {
	parts := strings.Fields(detachKey)
	if len(parts) < 2 {
		return nil
	}
	return c.cmd("bind-key", parts[len(parts)-1], "detach-client").Run()
}

func (c *Client) NewSession(o NewSessionOpts) error {
	if o.Name == "" {
		return errors.New("tmux: empty session name")
	}
	args := []string{"new-session", "-d", "-s", o.Name}
	if o.Cwd != "" {
		args = append(args, "-c", o.Cwd)
	}
	for k, v := range o.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if o.Cmd != "" {
		args = append(args, o.Cmd)
	}
	out, err := c.cmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = c.cmd("set-option", "-pt", o.Name, "allow-passthrough", "on").Run()
	return nil
}

func (c *Client) HasSession(name string) (bool, error) {
	err := c.cmd("has-session", "-t", name).Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func (c *Client) LsPrefix(prefix string) ([]string, error) {
	out, err := c.cmd("ls", "-F", "#{session_name}").Output()
	if err != nil {
		// "no server running" means zero sessions; treat as empty list.
		if ee, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(ee.Stderr), "no server") {
				return nil, nil
			}
		}
		return nil, err
	}
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

func (c *Client) Kill(name string) error {
	return c.cmd("kill-session", "-t", name).Run()
}

func (c *Client) KillServer() error {
	return c.cmd("kill-server").Run()
}

// attachArgs builds the argv for attaching the caller to sessionID. Inside an
// existing tmux client we switch-client so we don't nest a tmux inside the
// current pane; otherwise we attach-session. Pure so the decision is testable
// in isolation, mirroring capturePaneArgs.
func attachArgs(sessionID string, insideTmux bool) []string {
	if insideTmux {
		return []string{"switch-client", "-t", sessionID}
	}
	return []string{"attach-session", "-t", sessionID}
}

// capturePaneArgs builds the argv for `tmux capture-pane` honoring the lines
// parameter via -S -<lines> (start N lines back from the bottom of history).
// Falls back to 30 lines when lines <= 0.
func capturePaneArgs(name string, lines int) []string {
	if lines <= 0 {
		lines = 30
	}
	return []string{"capture-pane", "-e", "-p", "-S", fmt.Sprintf("-%d", lines), "-t", name + ":."}
}

func (c *Client) CapturePane(name string, lines int) (string, error) {
	out, err := c.cmd(capturePaneArgs(name, lines)...).Output()
	return string(out), err
}

// sendKeysSettle is the pause between delivering a line's literal text and the
// Enter that submits it. Glued into one send-keys, the carriage return arrives
// in the same pane read as the text, which a paste-aware client (Claude Code)
// treats as a literal newline rather than a submit — the message then sits
// unsent in the input box until the user attaches and presses Enter. The gap
// makes the Enter land in its own read as a discrete keystroke. 40ms is
// imperceptible for a manual send and holds even while the client is rendering.
const sendKeysSettle = 40 * time.Millisecond

// sendKeysCmds builds the ordered tmux argument lists that deliver text to a
// pane — one entry per command, run in sequence. Text is sent with -l so a line
// that matches a tmux key name ("C-c", "Enter", "Space") is typed verbatim
// instead of executed as that key. Each line's submit is its own Enter command,
// never glued onto the text in a single send-keys (see sendKeysSettle). Pure,
// mirroring capturePaneArgs/attachArgs, so the sequencing is testable without a
// live server.
func sendKeysCmds(name, text string) [][]string {
	var cmds [][]string
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			cmds = append(cmds, []string{"send-keys", "-t", name, "-l", line})
		}
		cmds = append(cmds, []string{"send-keys", "-t", name, "Enter"})
	}
	return cmds
}

// SendKeys sends text followed by Enter to a tmux session. Each line's literal
// text and its submitting Enter go as separate commands with a settle pause
// between them so the Enter is seen as a discrete keystroke, not a pasted
// newline.
func (c *Client) SendKeys(name string, text string) error {
	for i, args := range sendKeysCmds(name, text) {
		if i > 0 {
			time.Sleep(sendKeysSettle)
		}
		out, err := c.cmd(args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tmux send-keys: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (c *Client) RenameSession(from, to string) error {
	return c.cmd("rename-session", "-t", from, to).Run()
}

// SessionPIDs returns the process IDs of every pane in the named tmux session.
func (c *Client) SessionPIDs(name string) ([]int, error) {
	out, err := c.cmd("list-panes", "-t", name, "-F", "#{pane_pid}").Output()
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// AttachCmd builds — but does not run — the command that attaches the caller to
// sessionID. It honors the configured socket via cmd() and reads $TMUX here, in
// the adapter, to choose switch-client (inside tmux) or attach-session (outside)
// via the pure attachArgs helper. It returns the unstarted command with stdio
// unset and no error: unlike every other method it only builds, because the CLI
// and the TUI own their terminals differently and must run it themselves.
func (c *Client) AttachCmd(sessionID string) *exec.Cmd {
	insideTmux := os.Getenv("TMUX") != ""
	return c.cmd(attachArgs(sessionID, insideTmux)...)
}
