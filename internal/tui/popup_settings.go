package tui

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/config"
)

// SettingsPopup edits a curated set of scalar settings in place. Edits apply to
// a working draft and are emitted live (SettingsChanged) so the dashboard
// recolors/resizes behind the popup; nothing is written to disk until the user
// confirms (SettingsSaved on enter). Cancelling (esc) is handled by the parent,
// which restores the pre-open config — see Model.handleKey.
type SettingsPopup struct {
	draft       config.Config
	theme       Theme
	fields      []settingField
	cursor      int
	editorInput textinput.Model
}

// SettingsChanged carries the live draft after every edit; the parent mirrors it
// into ctx.Config (in memory only) for instant preview.
type SettingsChanged struct{ Config config.Config }

// SettingsSaved is emitted on enter; the parent normalizes and persists it.
type SettingsSaved struct{ Config config.Config }

type fieldKind int

const (
	fieldBool fieldKind = iota
	fieldEnum
	fieldInt
	fieldFloat
	fieldDuration
	fieldString
)

// settingField is one editable row. display renders the current value; step
// applies a ±1 adjustment (toggle/cycle/increment) returning the updated config.
// step is nil for string fields, which are edited through editorInput instead.
type settingField struct {
	section string
	label   string
	kind    fieldKind
	display func(c config.Config) string
	step    func(c config.Config, dir int) config.Config
}

func NewSettingsPopup(cfg config.Config, theme Theme, agentNames []string) SettingsPopup {
	ti := textinput.New()
	ti.Placeholder = "$EDITOR / $VISUAL"
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.Width = 30
	ti.SetValue(cfg.UI.Editor)

	agents := append([]string(nil), agentNames...)
	sort.Strings(agents)
	themes := ThemeNames()

	fields := []settingField{
		enumField("General", "default agent", agents,
			func(c config.Config) string { return c.DefaultAgent },
			func(c *config.Config, v string) { c.DefaultAgent = v }),
		stringField("General", "editor",
			func(c config.Config) string { return c.UI.Editor }),
		enumField("Appearance", "theme", themes,
			func(c config.Config) string { return c.UI.Theme },
			func(c *config.Config, v string) { c.UI.Theme = v }),
		intField("Appearance", "sidebar width", config.MinSidebarWidth, config.MaxSidebarWidth, 2,
			func(c config.Config) int { return c.UI.SidebarWidth },
			func(c *config.Config, v int) { c.UI.SidebarWidth = v }),
		boolField("Pane Preview", "enabled",
			func(c config.Config) bool { return c.UI.PanePreview.Enabled },
			func(c *config.Config, v bool) { c.UI.PanePreview.Enabled = v }),
		intField("Pane Preview", "lines", config.MinPanePreviewLines, 0, 5,
			func(c config.Config) int { return c.UI.PanePreview.Lines },
			func(c *config.Config, v int) { c.UI.PanePreview.Lines = v }),
		durationField("Pane Preview", "interval", config.MinPanePreviewInterval, 250*time.Millisecond,
			func(c config.Config) time.Duration { return c.UI.PanePreview.Interval },
			func(c *config.Config, v time.Duration) { c.UI.PanePreview.Interval = v }),
		floatField("UX", "status timeout (s)", config.MinStatusTimeoutSeconds, config.MaxStatusTimeoutSeconds, 0.5, 1,
			func(c config.Config) float64 { return c.UI.StatusTimeoutSeconds },
			func(c *config.Config, v float64) { c.UI.StatusTimeoutSeconds = v }),
		intField("UX", "event log lines", config.MinEventLogLines, 0, 10,
			func(c config.Config) int { return c.UI.EventLogLines },
			func(c *config.Config, v int) { c.UI.EventLogLines = v }),
		boolField("Sound", "enabled",
			func(c config.Config) bool { return c.Sound.Enabled },
			func(c *config.Config, v bool) { c.Sound.Enabled = v }),
		floatField("Sound", "volume", config.MinSoundVolume, config.MaxSoundVolume, 0.05, 2,
			func(c config.Config) float64 { return c.Sound.Volume },
			func(c *config.Config, v float64) { c.Sound.Volume = v }),
	}

	return SettingsPopup{
		draft:       cfg,
		theme:       theme,
		fields:      fields,
		editorInput: ti,
	}
}

// --- field builders ---

func boolField(section, label string, get func(config.Config) bool, set func(*config.Config, bool)) settingField {
	return settingField{
		section: section, label: label, kind: fieldBool,
		display: func(c config.Config) string {
			if get(c) {
				return "on"
			}
			return "off"
		},
		// dir is ignored: either arrow (and space) flips the value.
		step: func(c config.Config, dir int) config.Config { set(&c, !get(c)); return c },
	}
}

func enumField(section, label string, options []string, get func(config.Config) string, set func(*config.Config, string)) settingField {
	return settingField{
		section: section, label: label, kind: fieldEnum,
		display: func(c config.Config) string { return get(c) },
		step: func(c config.Config, dir int) config.Config {
			if len(options) == 0 {
				return c
			}
			i := indexOf(options, get(c))
			if i < 0 {
				set(&c, options[0])
				return c
			}
			i = (i + dir + len(options)) % len(options)
			set(&c, options[i])
			return c
		},
	}
}

// intField clamps to [min, max]. A max of 0 means no upper bound (used for
// fields like line counts whose only real limit is the lower one).
func intField(section, label string, min, max, step int, get func(config.Config) int, set func(*config.Config, int)) settingField {
	return settingField{
		section: section, label: label, kind: fieldInt,
		display: func(c config.Config) string { return strconv.Itoa(get(c)) },
		step: func(c config.Config, dir int) config.Config {
			v := get(c) + dir*step
			if v < min {
				v = min
			}
			if max > 0 && v > max {
				v = max
			}
			set(&c, v)
			return c
		},
	}
}

func floatField(section, label string, min, max, step float64, prec int, get func(config.Config) float64, set func(*config.Config, float64)) settingField {
	return settingField{
		section: section, label: label, kind: fieldFloat,
		display: func(c config.Config) string { return strconv.FormatFloat(get(c), 'f', prec, 64) },
		step: func(c config.Config, dir int) config.Config {
			v := get(c) + float64(dir)*step
			// Snap to the step grid so repeated stepping doesn't accumulate
			// float drift (e.g. 0.7000000001).
			v = math.Round(v/step) * step
			if v < min {
				v = min
			}
			if v > max {
				v = max
			}
			set(&c, v)
			return c
		},
	}
}

func durationField(section, label string, min, step time.Duration, get func(config.Config) time.Duration, set func(*config.Config, time.Duration)) settingField {
	return settingField{
		section: section, label: label, kind: fieldDuration,
		display: func(c config.Config) string { return get(c).String() },
		step: func(c config.Config, dir int) config.Config {
			v := get(c) + time.Duration(dir)*step
			if v < min {
				v = min
			}
			set(&c, v)
			return c
		},
	}
}

func stringField(section, label string, get func(config.Config) string) settingField {
	return settingField{
		section: section, label: label, kind: fieldString,
		display: func(c config.Config) string { return get(c) },
		step:    nil,
	}
}

func indexOf(opts []string, v string) int {
	for i, o := range opts {
		if o == v {
			return i
		}
	}
	return -1
}

// --- model ---

func (p SettingsPopup) Init() tea.Cmd {
	if p.onStringField() {
		return p.editorInput.Focus()
	}
	return nil
}

func (p SettingsPopup) onStringField() bool {
	return p.cursor >= 0 && p.cursor < len(p.fields) && p.fields[p.cursor].kind == fieldString
}

func (p *SettingsPopup) moveCursor(dir int) {
	n := len(p.fields)
	if n == 0 {
		return
	}
	p.cursor = (p.cursor + dir + n) % n
}

// syncFocus focuses the editor text input when the cursor lands on the string
// field and blurs it otherwise, returning any cursor command the input emits.
func (p *SettingsPopup) syncFocus() tea.Cmd {
	if p.onStringField() {
		return p.editorInput.Focus()
	}
	p.editorInput.Blur()
	return nil
}

func (p SettingsPopup) changedCmd() tea.Cmd {
	cfg := p.draft
	return func() tea.Msg { return SettingsChanged{Config: cfg} }
}

func (p SettingsPopup) editField(dir int) (tea.Model, tea.Cmd) {
	f := p.fields[p.cursor]
	if f.step == nil {
		return p, nil
	}
	p.draft = f.step(p.draft, dir)
	// Keep the popup's own borders in sync when the theme field changes.
	p.theme = Resolve(p.draft.UI.Theme)
	return p, p.changedCmd()
}

func (p SettingsPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	onString := p.onStringField()

	switch keyMsg.String() {
	case "enter":
		return p, func() tea.Msg { return SettingsSaved{Config: p.draft} }
	case "tab", "down":
		p.moveCursor(1)
		cmd := p.syncFocus()
		return p, cmd
	case "shift+tab", "up":
		p.moveCursor(-1)
		cmd := p.syncFocus()
		return p, cmd
	case "j":
		if !onString {
			p.moveCursor(1)
			cmd := p.syncFocus()
			return p, cmd
		}
	case "k":
		if !onString {
			p.moveCursor(-1)
			cmd := p.syncFocus()
			return p, cmd
		}
	case "right":
		if !onString {
			return p.editField(1)
		}
	case "left":
		if !onString {
			return p.editField(-1)
		}
	case " ":
		if !onString {
			if k := p.fields[p.cursor].kind; k == fieldBool || k == fieldEnum {
				return p.editField(1)
			}
			return p, nil
		}
	}

	// On the string field, every other key (printable runes, caret moves,
	// backspace) is typing — forward it to the focused text input.
	if onString {
		var cmd tea.Cmd
		p.editorInput, cmd = p.editorInput.Update(keyMsg)
		p.draft.UI.Editor = p.editorInput.Value()
		return p, tea.Batch(cmd, p.changedCmd())
	}
	return p, nil
}

func (p SettingsPopup) View() string {
	const popW = 60
	bdr := popupBorderStyle(p.theme)
	iw := popW - 2
	cw := iw - 2
	const labelW = 22

	var b strings.Builder
	hbar := strings.Repeat("─", iw)

	writeRow := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}
	writeBlank := func() {
		b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	}

	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Settings")
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("live preview · saved on enter")
	gap := cw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	lastSection := ""
	for i, f := range p.fields {
		if f.section != lastSection {
			writeBlank()
			writeRow(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(f.section))
			lastSection = f.section
		}

		active := i == p.cursor
		cursorGlyph := "  "
		labelSt := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
		if active {
			cursorGlyph = lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("› ")
			labelSt = lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true)
		}

		var valStr string
		switch {
		case f.kind == fieldString && active:
			valStr = p.editorInput.View()
		case f.kind == fieldString:
			v := f.display(p.draft)
			if v == "" {
				valStr = lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("$EDITOR / $VISUAL")
			} else {
				valStr = lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true).Render(v)
			}
		default:
			valStr = lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("‹ " + f.display(p.draft) + " ›")
		}

		writeRow(cursorGlyph + labelSt.Render(padRight(f.label, labelW)) + valStr)
	}

	writeBlank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	foot := p.theme.KeyHint("↑/↓", "move") + "  " +
		p.theme.KeyHint("←/→", "change") + "  " +
		p.theme.KeyHint("enter", "save") + "  " +
		p.theme.KeyHint("esc", "cancel")
	writeRow(foot)
	b.WriteString(bdr.Render("└" + hbar + "┘"))

	return b.String()
}
