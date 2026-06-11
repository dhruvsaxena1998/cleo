package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func newRmCmd(getCtx func() *Ctx) *cobra.Command {
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "rm <project|session-id>",
		Short: "Unregister a project, or remove one session record and its worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()

			// A session ID names exactly one record; try that first so session
			// removal and project removal share one verb.
			if sess, err := c.State.Get(args[0]); err == nil {
				return removeSession(cmd, c, sess, force, yes)
			}

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

			lifecycle := c.NewLifecycle()

			result, err := lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
				ProjectID: proj.ID,
				Force:     force,
			})
			if errors.Is(err, sessionlifecycle.ErrDirtyWorktreesBlock) {
				fmt.Fprintf(cmd.ErrOrStderr(), "sessions with uncommitted worktree changes:\n")
				for _, id := range result.DirtyWorktreeSessionIDs {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", id)
				}
				return fmt.Errorf("%w — commit or discard the work, or use --force", err)
			}
			if err != nil {
				return err
			}
			for _, w := range result.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", w)
			}

			if err := c.Projects.Remove(proj.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed project %q\n", proj.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "remove even if sessions are active or worktrees are dirty")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func removeSession(cmd *cobra.Command, c *Ctx, sess state.Session, force, yes bool) error {
	if !yes {
		what := fmt.Sprintf("remove session %q", sess.ID)
		if sess.HasWorktree() {
			what += " and its worktree"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s? [y/N] ", what)
		ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) != "y" {
			return fmt.Errorf("aborted")
		}
	}

	lifecycle := c.NewLifecycle()
	result, err := lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: sess.ID,
		Force:     force,
	})
	if errors.Is(err, sessionlifecycle.ErrWorktreeDirty) || errors.Is(err, sessionlifecycle.ErrSessionActive) {
		return fmt.Errorf("%w (use --force to override)", err)
	}
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", w)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed session %q\n", sess.ID)
	if sess.HasWorktree() {
		fmt.Fprintf(cmd.OutOrStdout(), "removed worktree %s (branch %s kept)\n", sess.WorktreePath, sess.WorktreeBranch)
	}
	return nil
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
