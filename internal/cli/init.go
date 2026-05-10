package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)

const (
	hookClaude = "claude"
	hookCodex  = "codex"
	hookPi     = "pi"
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
	var yes bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install hooks into agent config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if err := sound.ExtractDefaults(c.Paths.SoundsDir()); err != nil {
				return err
			}
			cleoBin, err := os.Executable()
			if err != nil {
				return err
			}
			cleoBin, _ = filepath.Abs(cleoBin)
			home, _ := os.UserHomeDir()

			selected := []string{hookClaude, hookCodex}
			if !yes {
				if err := promptHookSelection(&selected); err != nil {
					return err
				}
			}

			var results []initInstallResult
			for _, h := range selected {
				switch h {
				case hookClaude:
					path := filepath.Join(home, ".claude", "settings.json")
					if err := hooks.InstallClaude(path, cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Claude Code",
						Files: []string{
							fmt.Sprintf("hooks: %s", path),
						},
						InstalledHooks: hooks.ClaudeEvents(),
					})
				case hookCodex:
					hooksPath := filepath.Join(home, ".codex", "hooks.json")
					configPath := filepath.Join(home, ".codex", "config.toml")
					if err := hooks.InstallCodex(hooksPath, configPath, cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Codex",
						Files: []string{
							fmt.Sprintf("hooks: %s", hooksPath),
							fmt.Sprintf("feature flag: %s ([features].hooks = true)", configPath),
						},
						InstalledHooks:   hooks.CodexEvents(),
						NeedsCodexReview: true,
						ReviewHooks:      hooks.CodexEvents(),
						ReviewCommand:    fmt.Sprintf("%s hook codex", cleoBin),
					})
				case hookPi:
					if err := (hooks.PiProtocol{}).Install(cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Pi",
						Files: []string{
							fmt.Sprintf("extension: %s", filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")),
						},
						InstalledHooks: hooks.PiEvents(),
					})
				}
			}
			printInitSummary(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite conflicting hooks")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "install all supported hook systems without prompting")
	return cmd
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

func promptHookSelection(selected *[]string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which hook systems would you like to install?").
				Options(
					huh.NewOption("Claude Code  (~/.claude/settings.json)", hookClaude),
					huh.NewOption("Codex        (~/.codex/hooks.json)", hookCodex),
					huh.NewOption("Pi           (~/.pi/agent/extensions/cleo.ts)", hookPi),
				).
				Value(selected),
		),
	).Run()
}
