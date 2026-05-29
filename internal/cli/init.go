package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)

const (
	hookClaude   = "claude"
	hookCodex    = "codex"
	hookPi       = "pi"
	hookOpenCode = "opencode"
)

var (
	initHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1"))
	initAgentStyle  = lipgloss.NewStyle().Bold(true)
	initOkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	initWarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	initDimStyle    = lipgloss.NewStyle().Faint(true)
	initLabelWidth  = 13
)

func newInitCmd(getCtx func() *Ctx) *cobra.Command {
	var force bool
	var agentsFlag string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install hooks into agent config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --agents BEFORE any side effects so an invalid
			// list cannot trigger sound extraction or context setup.
			var selected []string
			if cmd.Flags().Changed("agents") {
				parsed, err := parseAgentsFlag(agentsFlag)
				if err != nil {
					return err
				}
				selected = parsed
			}

			c := getCtx()
			if err := sound.ExtractDefaults(c.Paths.SoundsDir()); err != nil {
				return err
			}
			cleoBin, err := os.Executable()
			if err != nil {
				return err
			}
			cleoBin, _ = filepath.Abs(cleoBin)

			if selected == nil {
				br := bufio.NewReader(cmd.InOrStdin())
				if err := promptHookSelection(cmd.OutOrStdout(), br, &selected); err != nil {
					return err
				}
			}

			var results []initInstallResult
			for _, name := range selected {
				p, ok := hooks.ProtocolByName(name)
				if !ok {
					continue // already validated by parseAgentsFlag
				}
				report, err := p.Install(cleoBin, force)
				if err != nil {
					return err
				}
				results = append(results, initResultFor(p, report))
			}
			printInitSummary(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite conflicting hooks")
	cmd.Flags().StringVar(&agentsFlag, "agents", "", "comma-separated list of agents to install (claude, codex, opencode, pi); skips interactive prompts")
	return cmd
}

// initResultFor builds the render row for one installed protocol from its
// registry-declared identity (DisplayName, Locations, Events) and the dynamic
// InstallReport (the manual-approval follow-up, if any).
func initResultFor(p hooks.Protocol, report hooks.InstallReport) initInstallResult {
	var files []string
	for _, loc := range p.Locations() {
		files = append(files, fmt.Sprintf("%s: %s", loc.Label, loc.Path))
	}
	res := initInstallResult{
		Name:           p.DisplayName(),
		Files:          files,
		InstalledHooks: p.Events(),
	}
	if report.ManualReview != nil {
		res.NeedsCodexReview = true
		res.ReviewHooks = report.ManualReview.Hooks
		res.ReviewCommand = report.ManualReview.Command
	}
	return res
}

type initInstallResult struct {
	Name             string
	Files            []string
	InstalledHooks   []string
	NeedsCodexReview bool
	ReviewHooks      []string
	ReviewCommand    string
}

func printInitSummary(w io.Writer, results []initInstallResult) {
	fmt.Fprintln(w, initHeaderStyle.Render("✓ Cleo hooks initialized"))
	fmt.Fprintln(w)

	for _, result := range results {
		fmt.Fprintf(w, "  %s\n", initAgentStyle.Render(result.Name))
		for _, file := range result.Files {
			label, path, found := strings.Cut(file, ": ")
			if !found {
				fmt.Fprintf(w, "  %s %s\n", initOkStyle.Render("✓"), file)
				continue
			}
			fmt.Fprintf(w, "  %s %-*s %s\n",
				initOkStyle.Render("✓"),
				initLabelWidth, label,
				initDimStyle.Render(path),
			)
		}
		if len(result.InstalledHooks) > 0 {
			const perRow = 4
			for i := 0; i < len(result.InstalledHooks); i += perRow {
				end := min(i+perRow, len(result.InstalledHooks))
				chunk := strings.Join(result.InstalledHooks[i:end], " · ")
				if i == 0 {
					fmt.Fprintf(w, "  %s %-*s %s\n",
						initOkStyle.Render("✓"),
						initLabelWidth, fmt.Sprintf("%d events", len(result.InstalledHooks)),
						chunk,
					)
				} else {
					fmt.Fprintf(w, "  %s %-*s %s\n", " ", initLabelWidth+4, "", chunk)
				}
			}
		}
		fmt.Fprintln(w)
	}

	for _, result := range results {
		if !result.NeedsCodexReview {
			continue
		}
		fmt.Fprintf(w, "%s %s requires manual hook approval\n",
			initWarnStyle.Render("⚠"), result.Name)
		fmt.Fprintf(w, "   Open %s, run /hooks, and approve entries starting with:\n", result.Name)
		fmt.Fprintf(w, "   %s\n", initDimStyle.Render(result.ReviewCommand))
		fmt.Fprintln(w, "   Restart any open sessions first so they pick up the updated config.")
		fmt.Fprintln(w)
	}
}

func promptYN(w io.Writer, br *bufio.Reader, label string, defaultYes bool) (bool, error) {
	var bracket string
	if defaultYes {
		bracket = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Render("[Y/n]")
	} else {
		bracket = initDimStyle.Render("[y/N]")
	}
	fmt.Fprintf(w, "  %s %s ", bracket, label)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y":
		return true, nil
	case "n":
		return false, nil
	default:
		return defaultYes, nil
	}
}

func promptHookSelection(w io.Writer, br *bufio.Reader, selected *[]string) error {
	fmt.Fprintln(w, initAgentStyle.Render("Which hook systems to install?"))
	var out []string
	for _, p := range hooks.Protocols() {
		yes, err := promptYN(w, br, agentPromptLabel(p), installDefaultSelected(p.Name()))
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

// installDefaultSelected reports whether an agent is pre-selected (Y) in the
// interactive init prompt. Claude and Codex are the common case; the
// extension/plugin agents (pi, opencode) default off.
func installDefaultSelected(name string) bool {
	return name == hookClaude || name == hookCodex
}

// agentPromptLabel renders one prompt line: the display name, padded, with a
// home-abbreviated hint of the primary file the agent owns.
func agentPromptLabel(p hooks.Protocol) string {
	hint := ""
	if locs := p.Locations(); len(locs) > 0 {
		hint = fmt.Sprintf("  (%s)", tildePath(locs[0].Path))
	}
	return fmt.Sprintf("%-12s%s", p.DisplayName(), hint)
}

// tildePath abbreviates the user's home directory to ~ for display.
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
