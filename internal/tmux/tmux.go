package tmux

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
