package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newAttachCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			installFocusHooks(c)
			markFocused(c, args[0], true)
			t := exec.Command("tmux", "attach", "-t", args[0])
			t.Stdin = os.Stdin
			t.Stdout = os.Stdout
			t.Stderr = os.Stderr
			err := t.Run()
			markFocused(c, args[0], false)
			return err
		},
	}
}
