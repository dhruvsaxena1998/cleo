package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newKillCmd(getCtx func() *Ctx) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "kill %q? [y/N] ", id)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			c := getCtx()
			_ = c.Tmux.Kill(id)
			return c.State.Delete(id)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
