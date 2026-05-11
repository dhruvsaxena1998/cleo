package tmux

import (
	"fmt"
	"strings"
)

// focusHooks maps tmux hook event names to focus direction ("in" or "out").
// Exported as a var so tests can verify all hooks are present.
var focusHooks = map[string]string{
	"client-attached":  "in",
	"client-focus-in":  "in",
	"client-detached":  "out",
	"client-focus-out": "out",
}

func (c *Client) InstallFocusHooks(cleoBin string) error {
	if cleoBin == "" {
		return nil
	}
	if err := c.cmd("set-option", "-g", "focus-events", "on").Run(); err != nil {
		return err
	}
	for hook, direction := range focusHooks {
		shellCommand := fmt.Sprintf("%s focus %s %s",
			shellQuote(cleoBin),
			shellQuote(direction),
			shellQuote("#{client_session}"),
		)
		command := fmt.Sprintf("run-shell -b %s", shellQuote(shellCommand))
		if err := c.cmd("set-hook", "-g", hook+"[900]", command).Run(); err != nil {
			return err
		}
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
