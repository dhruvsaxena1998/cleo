package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFocusCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:    "focus <in|out> <session-id>",
		Short:  "Internal: record tmux client focus state",
		Args:   cobra.ExactArgs(2),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			focused, err := parseFocusDirection(args[0])
			if err != nil {
				return err
			}
			c := getCtx()
			return c.Focus.Set(args[1], focused)
		},
	}
}

func parseFocusDirection(direction string) (bool, error) {
	switch direction {
	case "in":
		return true, nil
	case "out":
		return false, nil
	default:
		return false, fmt.Errorf("unknown focus direction %q", direction)
	}
}
