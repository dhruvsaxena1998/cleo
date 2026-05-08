package cli

import (
	"os"
	"path/filepath"
)

func installFocusHooks(c *Ctx) {
	installer, ok := c.Tmux.(TmuxFocusInstaller)
	if !ok {
		return
	}
	cleoBin, err := os.Executable()
	if err != nil {
		return
	}
	cleoBin, _ = filepath.Abs(cleoBin)
	_ = installer.InstallFocusHooks(cleoBin)
}

func markFocused(c *Ctx, sessionID string, focused bool) {
	if c == nil || c.Focus == nil {
		return
	}
	_ = c.Focus.Set(sessionID, focused)
}
