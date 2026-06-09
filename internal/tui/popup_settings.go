package tui

import (
	"math"
	"sort"
	"strconv"
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
//
// The field list can outgrow short terminals, so the body scrolls: maxHeight is
// the terminal height, and View renders a window of rows that keeps the cursor
// visible while pinning the title and footer.
type SettingsPopup struct {
	draft       config.Config
	theme       Theme
	fields      []settingField
	cursor      int
	editorInput textinput.Model
	maxHeight   int
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

// soundEventOrder lists the known sound events in lifecycle order so the editor
// presents them predictably; any extra events in the config are appended sorted.
var soundEventOrder = []string{
	"session_start", "needs_input", "session_idle", "session_completed", "session_error",
}

func NewSettingsPopup(cfg config.Config, theme Theme, agentNames []string, maxHeight int) SettingsPopup {
	// Own a private copy of the events map: it is shared by reference with the
	// parent's config (and thus the revert snapshot), so per-event toggles must
	// not mutate it in place or esc-cancel would not restore the originals.
	cfg.Sound.Events = cloneSoundEvents(cfg.Sound.Events)

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
		enumField("Appearance", "icons", IconSetNames(),
			func(c config.Config) string { return c.UI.Icons },
			func(c *config.Config, v string) { c.UI.Icons = v }),
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
		durationField("Timeouts", "idle→completed", time.Minute, time.Minute,
			func(c config.Config) time.Duration { return c.Timeouts.IdleToCompletedTimeout },
			func(c *config.Config, v time.Duration) { c.Timeouts.IdleToCompletedTimeout = v }),
		durationField("Timeouts", "spawning", time.Second, 5*time.Second,
			func(c config.Config) time.Duration { return c.Timeouts.SpawningTimeout },
			func(c *config.Config, v time.Duration) { c.Timeouts.SpawningTimeout = v }),
		intField("Pruning", "hint threshold", 0, 0, 1,
			func(c config.Config) int { return c.Pruning.HintThreshold },
			func(c *config.Config, v int) { c.Pruning.HintThreshold = v }),
		intField("Pruning", "keep default", 0, 0, 1,
			func(c config.Config) int { return c.Pruning.KeepDefault },
			func(c *config.Config, v int) { c.Pruning.KeepDefault = v }),
		boolField("Sound", "enabled",
			func(c config.Config) bool { return c.Sound.Enabled },
			func(c *config.Config, v bool) { c.Sound.Enabled = v }),
		floatField("Sound", "volume", config.MinSoundVolume, config.MaxSoundVolume, 0.05, 2,
			func(c config.Config) float64 { return c.Sound.Volume },
			func(c *config.Config, v float64) { c.Sound.Volume = v }),
	}

	// One toggle per configured sound event. The global Sound.enabled switch
	// gates all of these at play time; these control each event individually.
	for _, name := range orderedSoundEvents(cfg.Sound.Events) {
		nm := name
		fields = append(fields, boolField("Sound Events", nm,
			func(c config.Config) bool { return c.Sound.Events[nm].Enabled },
			func(c *config.Config, v bool) {
				e := c.Sound.Events[nm]
				e.Enabled = v
				c.Sound.Events[nm] = e
			}))
	}

	return SettingsPopup{
		draft:       cfg,
		theme:       theme,
		fields:      fields,
		editorInput: ti,
		maxHeight:   maxHeight,
	}
}

func cloneSoundEvents(in map[string]config.SoundEvent) map[string]config.SoundEvent {
	out := make(map[string]config.SoundEvent, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// orderedSoundEvents returns the event keys in lifecycle order (soundEventOrder
// first), with any extra keys appended alphabetically for a stable display.
func orderedSoundEvents(events map[string]config.SoundEvent) []string {
	out := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, k := range soundEventOrder {
		if _, ok := events[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var extra []string
	for k := range events {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
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
	// Keep the popup's own theme in sync when the theme/icons fields change, so
	// its chrome tracks the live preview.
	p.theme = Resolve(p.draft.UI.Theme)
	p.theme.Icons = resolveIcons(p.draft.UI.Icons)
	return p, p.changedCmd()
}

func (p SettingsPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		p.maxHeight = ws.Height
		return p, nil
	}
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

const settingsPopupWidth = 60

// settingsChromeRows counts the non-body lines: top border, title, divider
// (above) and divider, footer, bottom border (below).
const settingsChromeRows = 6

// bodyBudget is how many body rows fit given the terminal height, leaving a row
// of margin top and bottom. A non-positive height means height is unknown, so
// no limit is applied.
func (p SettingsPopup) bodyBudget() int {
	if p.maxHeight <= 0 {
		return len(p.fields)*2 + 16 // effectively unlimited
	}
	b := p.maxHeight - settingsChromeRows - 2
	if b < 6 {
		b = 6
	}
	return b
}

func (p SettingsPopup) fieldRow(i int, f settingField, labelW int) string {
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
		if v := f.display(p.draft); v != "" {
			valStr = lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true).Render(v)
		} else {
			valStr = lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("$EDITOR / $VISUAL")
		}
	default:
		valStr = lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("‹ " + f.display(p.draft) + " ›")
	}

	return cursorGlyph + labelSt.Render(padRight(f.label, labelW)) + valStr
}

func (p SettingsPopup) View() string {
	const labelW = 22

	// Build the body as inner-content rows (section spacers, headers, fields)
	// and remember which row the cursor sits on so the scroll window can keep
	// it on screen.
	var rows []string
	cursorRow := 0
	lastSection := ""
	for i, f := range p.fields {
		if f.section != lastSection {
			rows = append(rows, "")
			rows = append(rows, lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(f.section))
			lastSection = f.section
		}
		if i == p.cursor {
			cursorRow = len(rows)
		}
		rows = append(rows, p.fieldRow(i, f, labelW))
	}

	// Scroll window: the popup keeps the cursor visible and only hands the frame
	// the rows that fit. The frame never owns a scroll model — it renders exactly
	// the slice it is given.
	budget := p.bodyBudget()
	start := 0
	if len(rows) > budget {
		if cursorRow >= budget {
			start = cursorRow - budget + 1
		}
		if start+budget > len(rows) {
			start = len(rows) - budget
		}
		if start < 0 {
			start = 0
		}
	}
	end := start + budget
	if end > len(rows) {
		end = len(rows)
	}
	visible := rows[start:end]
	scrollable := start > 0 || end < len(rows)

	foot := p.theme.KeyHint("↑/↓", "move") + "  " +
		p.theme.KeyHint("←/→", "change") + "  " +
		p.theme.KeyHint("enter", "save") + "  " +
		p.theme.KeyHint("esc", "cancel")
	if scrollable {
		foot += "  " + lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("• ↕ more")
	}

	return drawFrame(frameSpec{
		Width:    settingsPopupWidth,
		Title:    lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Settings"),
		Hint:     lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("live preview · saved on enter"),
		Border:   popupBorderStyle(p.theme),
		Sections: [][]string{visible, {foot}},
	})
}
