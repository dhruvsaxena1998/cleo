package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
)

func newRmCmd(getCtx func() *Ctx) *cobra.Command {
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "rm <project>",
		Short: "Unregister a project (running sessions keep running unless --force)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			proj, err := resolveProject(c, args[0])
			if err != nil {
				return err
			}

			sessions, _ := c.State.List()
			var activeSessions []string
			var inactiveSessions []string
			for _, s := range sessions {
				if s.ProjectID != proj.ID {
					continue
				}
				if s.State.IsFinished() {
					inactiveSessions = append(inactiveSessions, s.ID)
				} else {
					activeSessions = append(activeSessions, s.ID)
				}
			}

			if len(activeSessions) > 0 && !force {
				return fmt.Errorf("%d active session(s) in %q — kill them first or use --force", len(activeSessions), proj.ID)
			}

			if !yes {
				total := len(activeSessions) + len(inactiveSessions)
				fmt.Fprintf(cmd.OutOrStdout(), "remove project %q and %d session record(s)? [y/N] ", proj.ID, total)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return fmt.Errorf("aborted")
				}
			}

			for _, id := range inactiveSessions {
				_ = c.State.Delete(id)
			}
			for _, id := range activeSessions {
				_ = c.State.Delete(id)
			}
			if err := c.Projects.Remove(proj.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed project %q\n", proj.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "remove even if active sessions exist")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func resolveProject(c *Ctx, arg string) (projects.Project, error) {
	p, err := c.Projects.Get(arg)
	if err == nil {
		return p, nil
	}
	abs, absErr := filepath.Abs(arg)
	if absErr != nil {
		return projects.Project{}, err
	}
	all, listErr := c.Projects.List()
	if listErr != nil {
		return projects.Project{}, listErr
	}
	for _, proj := range all {
		if proj.Path == abs {
			return proj, nil
		}
	}
	return projects.Project{}, projects.ErrNotFound
}
