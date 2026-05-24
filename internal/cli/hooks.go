package cli

import "github.com/spf13/cobra"

// newHooksCmd returns the `cleo hooks` group command that fans out to
// `init`, `cleanup`, and the hidden `invoke` runtime entry point.
func newHooksCmd(getCtx func() *Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage Cleo hooks across supported agents",
	}
	cmd.AddCommand(
		newInitCmd(getCtx),
		newCleanupCmd(getCtx),
		newHookCmd(getCtx),
	)
	return cmd
}
