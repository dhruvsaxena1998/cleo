package cli

import (
	"fmt"
	"os"

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
			lifecycle := c.NewLifecycle()

			plan, err := lifecycle.Attach(args[0])
			if err != nil {
				return err
			}
			switch plan.Action {
			case sessionlifecycle.AttachBlocked:
				return fmt.Errorf("session %q is %s; cannot attach", args[0], plan.Session.State)
			case sessionlifecycle.AttachMarkedDead:
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is no longer running; marked dead\n", args[0])
				return nil
			}

			plan.Cmd.Stdin = os.Stdin
			plan.Cmd.Stdout = os.Stdout
			plan.Cmd.Stderr = os.Stderr
			err = plan.Cmd.Run()
			plan.Done()
			return err
		},
	}
}
