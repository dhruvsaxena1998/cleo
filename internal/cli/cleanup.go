package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newCleanupCmd(getCtx func() *Ctx) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "cleanup",
		Aliases: []string{"uninstall"},
		Short:   "Remove Cleo hooks from agent config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = getCtx()
			home, _ := os.UserHomeDir()

			selected := []string{hookClaude, hookCodex}
			if !yes {
				br := bufio.NewReader(cmd.InOrStdin())
				if err := promptCleanupSelection(cmd.OutOrStdout(), br, &selected); err != nil {
					return err
				}
			}

			var results []cleanupResult
			for _, h := range selected {
				switch h {
				case hookClaude:
					path := filepath.Join(home, ".claude", "settings.json")
					removed, err := hooks.CleanupClaude(path)
					if err != nil {
						return err
					}
					results = append(results, cleanupResult{
						Name:    "Claude Code",
						Path:    path,
						Removed: removed,
					})
				case hookCodex:
					path := filepath.Join(home, ".codex", "hooks.json")
					removed, err := hooks.CleanupCodex(path)
					if err != nil {
						return err
					}
					results = append(results, cleanupResult{
						Name:    "Codex",
						Path:    path,
						Removed: removed,
						Notes: []string{
							"left ~/.codex/config.toml [features].hooks unchanged; that flag may be used by other Codex hooks",
						},
					})
				}
			}
			printCleanupSummary(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "clean up all supported hook systems without prompting")
	return cmd
}

type cleanupResult struct {
	Name    string
	Path    string
	Removed int
	Notes   []string
}

func printCleanupSummary(w io.Writer, results []cleanupResult) {
	fmt.Fprintln(w, "Cleo hook cleanup complete")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Updated:")
	for _, result := range results {
		fmt.Fprintf(w, "  - %s\n", result.Name)
		fmt.Fprintf(w, "    hooks: %s\n", result.Path)
		fmt.Fprintf(w, "    removed: %d Cleo hook command(s)\n", result.Removed)
		for _, note := range result.Notes {
			fmt.Fprintf(w, "    note: %s\n", note)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  - Restart any open agent sessions so they stop using the removed hooks.")
	fmt.Fprintln(w, "  - Run cleo init again if you want to reinstall Cleo hooks later.")
}

func promptCleanupSelection(w io.Writer, br *bufio.Reader, selected *[]string) error {
	fmt.Fprintln(w, "Which hook systems to clean up?")
	type hookOpt struct {
		key    string
		label  string
		defYes bool
	}
	opts := []hookOpt{
		{hookClaude, "Claude Code  (~/.claude/settings.json)", true},
		{hookCodex, "Codex        (~/.codex/hooks.json)", true},
	}
	var out []string
	for _, o := range opts {
		yes, err := promptYN(w, br, o.label, o.defYes)
		if err != nil {
			return err
		}
		if yes {
			out = append(out, o.key)
		}
	}
	*selected = out
	return nil
}
