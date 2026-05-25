package cli

import "github.com/dhruvsaxena1998/cleo/internal/tmux"

type TmuxClient interface {
	NewSession(o tmux.NewSessionOpts) error
	HasSession(name string) (bool, error)
	LsPrefix(prefix string) ([]string, error)
	Kill(name string) error
	CapturePane(name string, lines int) (string, error)
	SendKeys(name string, text string) error
	RenameSession(from, to string) error
}

type TmuxFocusInstaller interface {
	InstallFocusHooks(cleoBin string) error
}
