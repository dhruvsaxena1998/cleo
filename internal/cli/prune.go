package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func newPruneCmd(getCtx func() *Ctx) *cobra.Command {
	var keep int
	var all bool
	var dryRun bool
	var yes bool

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
			sessions, _ := c.State.List()
			candidates := []string{}
			byProj := map[string][]int{}
			for i, s := range sessions {
				if !s.State.IsFinished() {
					continue
				}
				if !all && projectFilter != "" && s.ProjectID != projectFilter {
					continue
				}
				byProj[s.ProjectID] = append(byProj[s.ProjectID], i)
			}
			for _, idxs := range byProj {
				sort.Slice(idxs, func(i, j int) bool {
					return sessions[idxs[i]].LastEventAt.After(sessions[idxs[j]].LastEventAt)
				})
				for i, idx := range idxs {
					if i < keep {
						continue
					}
					candidates = append(candidates, sessions[idx].ID)
				}
			}
			if dryRun {
				for _, id := range candidates {
					fmt.Fprintln(cmd.OutOrStdout(), id)
				}
				return nil
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "prune %d session(s)? [y/N] ", len(candidates))
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			for _, id := range candidates {
				_ = events.Archive(c.Paths.EventsLog(id), c.Paths.ArchiveDir())
				_ = c.State.Delete(id)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pruned %d session(s)\n", len(candidates))
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", -1, "keep N most recent finished per project (default config)")
	cmd.Flags().BoolVar(&all, "all", false, "across all projects")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without removing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
