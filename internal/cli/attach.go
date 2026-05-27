package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
)

func newAttachCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
				Config:   c.Config,
				Projects: c.Projects,
				State:    c.State,
				Tmux:     c.Tmux,
				Paths:    c.Paths,
				Focus:    c.Focus,
			})

			result, err := lifecycle.PrepareAttach(args[0])
			if err != nil {
				return err
			}
			switch result.Action {
			case sessionlifecycle.AttachBlocked:
				return fmt.Errorf("session %q is %s; cannot attach", args[0], result.Session.State)
			case sessionlifecycle.AttachMarkedDead:
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is no longer running; marked dead\n", args[0])
				return nil
			}

			installFocusHooks(c)
			lifecycle.SetFocused(args[0], true)
			t := exec.Command("tmux", "attach", "-t", args[0])
			t.Stdin = os.Stdin
			t.Stdout = os.Stdout
			t.Stderr = os.Stderr
			err = t.Run()
			lifecycle.SetFocused(args[0], false)
			return err
		},
	}
}
