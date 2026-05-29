package cli

import (
	"bufio"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newCleanupCmd(getCtx func() *Ctx) *cobra.Command {
	var agentsFlag string

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove Cleo hooks from agent config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --agents BEFORE touching any files on disk.
			var selected []string
			if cmd.Flags().Changed("agents") {
				parsed, err := parseAgentsFlag(agentsFlag)
				if err != nil {
					return err
				}
				selected = parsed
			}

			_ = getCtx()

			if selected == nil {
				br := bufio.NewReader(cmd.InOrStdin())
				if err := promptCleanupSelection(cmd.OutOrStdout(), br, &selected); err != nil {
					return err
				}
			}

			var results []cleanupResult
			for _, name := range selected {
				p, ok := hooks.ProtocolByName(name)
				if !ok {
					continue // already validated by parseAgentsFlag
				}
				outcome, err := p.Cleanup()
				if err != nil {
					return err
				}
				results = append(results, cleanupResult{
					Name:    p.DisplayName(),
					Outcome: outcome,
					Notes:   outcome.Notes,
				})
			}
			printCleanupSummary(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentsFlag, "agents", "", "comma-separated list of agents to clean up (claude, codex, opencode, pi); skips interactive prompts")
	return cmd
}

type cleanupResult struct {
	Name    string
	Outcome hooks.CleanupOutcome
	Notes   []string
}

func printCleanupSummary(w io.Writer, results []cleanupResult) {
	fmt.Fprintln(w, "Cleo hook cleanup complete")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Updated:")
	for _, result := range results {
		fmt.Fprintf(w, "  - %s\n", result.Name)
		fmt.Fprintf(w, "    hooks: %s\n", result.Outcome.Path)
		switch result.Outcome.Status {
		case hooks.CleanupStatusRemoved:
			fmt.Fprintln(w, "    removed Cleo hook entries")
		case hooks.CleanupStatusMissing:
			fmt.Fprintln(w, "    nothing to remove")
		case hooks.CleanupStatusSkippedModified:
			fmt.Fprintln(w, "    skipped modified file")
		}
		for _, note := range result.Notes {
			fmt.Fprintf(w, "    note: %s\n", note)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  - Restart any open agent sessions so they stop using the removed hooks.")
	fmt.Fprintln(w, "  - Run cleo hooks init again if you want to reinstall Cleo hooks later.")
}

func promptCleanupSelection(w io.Writer, br *bufio.Reader, selected *[]string) error {
	fmt.Fprintln(w, initAgentStyle.Render("Which hook systems to clean up?"))
	// Locked design: cleanup defaults all agents to Y. Removing is safe; if the
	// user is cleaning up, scrub everything cleo touched.
	var out []string
	for _, p := range hooks.Protocols() {
		yes, err := promptYN(w, br, agentPromptLabel(p), true)
		if err != nil {
			return err
		}
		if yes {
			out = append(out, p.Name())
		}
	}
	*selected = out
	return nil
}
