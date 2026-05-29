package cli

import "github.com/dhruvsaxena1998/cleo/internal/tmux"

// TmuxClient is the production tmux adapter's full surface. It is a superset of
// sessionlifecycle.Tmux, so Ctx.Tmux satisfies the lifecycle seam by structural
// subtyping with no extra wiring.
type TmuxClient interface {
	NewSession(o tmux.NewSessionOpts) error
	HasSession(name string) (bool, error)
	LsPrefix(prefix string) ([]string, error)
	Kill(name string) error
	BindDetachKey(detachKey string) error
	InstallFocusHooks(cleoBin string) error
	CapturePane(name string, lines int) (string, error)
	SendKeys(name string, text string) error
	RenameSession(from, to string) error
}
