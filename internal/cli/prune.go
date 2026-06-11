package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
)

func newPruneCmd(getCtx func() *Ctx) *cobra.Command {
	var keep int
	var all bool
	var dryRun bool
	var yes bool
	var force bool

	cmd := &cobra.Command{
		Use:   "prune [project]",
		Short: "Remove finished sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if keep < 0 {
				keep = c.Config.Pruning.KeepDefault
			}
			projectFilter := ""
			if len(args) == 1 {
				projectFilter = args[0]
			}

			lifecycle := c.NewLifecycle()

			// Preview candidates.
			preview, err := lifecycle.Prune(sessionlifecycle.PruneInput{
				ProjectID:   projectFilter,
				Keep:        keep,
				AllProjects: all,
				DryRun:      true,
			})
			if err != nil {
				return err
			}

			if dryRun {
				for _, id := range preview.Pruned {
					fmt.Fprintln(cmd.OutOrStdout(), id)
				}
				return nil
			}

			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "prune %d session(s)? [y/N] ", len(preview.Pruned))
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}

			result, err := lifecycle.Prune(sessionlifecycle.PruneInput{
				ProjectID:   projectFilter,
				Keep:        keep,
				AllProjects: all,
				Force:       force,
			})
			if err != nil {
				return err
			}
			for _, w := range result.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", w)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pruned %d session(s)\n", len(result.Pruned))
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", -1, "keep N most recent finished per project (default config)")
	cmd.Flags().BoolVar(&all, "all", false, "across all projects")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without removing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	cmd.Flags().BoolVar(&force, "force", false, "remove dirty worktrees too instead of skipping their sessions")
	return cmd
}
