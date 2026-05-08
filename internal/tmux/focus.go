package tmux

import (
	"fmt"
	"strings"
)

func (c *Client) InstallFocusHooks(cleoBin string) error {
	if cleoBin == "" {
		return nil
	}
	if err := c.cmd("set-option", "-g", "focus-events", "on").Run(); err != nil {
		return err
	}
	hooks := map[string]string{
		"client-attached": "in",
		"client-focus-in": "in",
		"client-detached": "out",
	}
	for hook, direction := range hooks {
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
