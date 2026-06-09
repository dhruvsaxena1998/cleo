package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// FinderSubmitted is sent when the user presses Enter on a session in the finder.
type FinderSubmitted struct {
	SessionID string
}

// FinderCancelled is sent when the user presses Esc in the finder.
type FinderCancelled struct{}

// finderRow is one visual line in the results list: either a non-selectable
// project header or a selectable session row.
type finderRow struct {
	isHeader bool
	project  string // set when isHeader
	matchIdx int    // index into the current matches slice when !isHeader
}

// FinderPopup is a center-screen fuzzy finder for attachable sessions.
type FinderPopup struct {
	ctx     *cli.Ctx
	theme   Theme
	input   textinput.Model // query field; owns the cursor and standard editing keys
	cursor  int             // index into the selectable session rows (matches slice)
	items   []state.Session // all attachable sessions, sorted by project then name
	sources []string        // parallel to items, for fuzzy matching
}

// NewFinderPopup builds a finder over all non-finished sessions.
func NewFinderPopup(ctx *cli.Ctx, theme Theme, sessions []state.Session) FinderPopup {
	var items []state.Session
	for _, s := range sessions {
		if s.State.IsFinished() {
			continue
		}
		items = append(items, s)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProjectID != items[j].ProjectID {
			return items[i].ProjectID < items[j].ProjectID
		}
		return items[i].Name < items[j].Name
	})
	sources := make([]string, len(items))
	for i, it := range items {
		sources[i] = it.Name + " " + it.ProjectID + " " + it.Agent
	}

	ti := textinput.New()
	ti.Placeholder = "type to filter sessions"
	ti.Prompt = "" // the "›" chevron is rendered by View, not the input
	ti.Focus()
	// Popup content is 74 cols (cw in View); cap below that so the styled view
	// never needs truncating and the cursor stays visible while typing.
	ti.Width = 70
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Overlay0)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text).Bold(true)

	return FinderPopup{
		ctx:     ctx,
		theme:   theme,
		input:   ti,
		items:   items,
		sources: sources,
	}
}

func (p FinderPopup) Init() tea.Cmd {
	return textinput.Blink
}

func (p FinderPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return p, func() tea.Msg { return FinderCancelled{} }
		case tea.KeyEnter:
			if sel, ok := p.selected(); ok {
				return p, func() tea.Msg { return FinderSubmitted{SessionID: sel.ID} }
			}
			return p, nil
		case tea.KeyUp:
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case tea.KeyDown:
			if p.cursor+1 < p.matchCount() {
				p.cursor++
			}
			return p, nil
		}
	}

	// Everything else — printable runes (including space and j/k), backspace,
	// and the standard readline editing/navigation keys (ctrl+u, ctrl+w,
	// alt+←/→, home/end, …) — is handled by the text input. Reset the result
	// cursor to the top whenever the query text changes.
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor = 0
	}
	p.clampCursor()
	return p, cmd
}

func (p FinderPopup) selected() (state.Session, bool) {
	matches := p.matches()
	if p.cursor < 0 || p.cursor >= len(matches) {
		return state.Session{}, false
	}
	return p.items[matches[p.cursor]], true
}

func (p FinderPopup) matchCount() int {
	return len(p.matches())
}

func (p FinderPopup) matches() []int {
	query := p.input.Value()
	if query == "" {
		out := make([]int, len(p.items))
		for i := range p.items {
			out[i] = i
		}
		return out
	}
	results := fuzzy.Find(query, p.sources)
	out := make([]int, len(results))
	for i, r := range results {
		out[i] = r.Index
	}
	return out
}

func (p *FinderPopup) clampCursor() {
	mc := p.matchCount()
	if p.cursor >= mc {
		p.cursor = mc - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// visibleRows builds the tree of project headers and session rows for rendering.
// A project header is inserted whenever the project ID changes across consecutive
// matches. The cursor maps to matchIdx values only (headers are never selectable).
func (p FinderPopup) visibleRows(matches []int, limit int) []finderRow {
	var rows []finderRow
	sessionCount := 0
	lastProject := ""
	for matchIdx, itemIdx := range matches {
		if sessionCount >= limit {
			break
		}
		s := p.items[itemIdx]
		if s.ProjectID != lastProject {
			rows = append(rows, finderRow{isHeader: true, project: s.ProjectID})
			lastProject = s.ProjectID
		}
		rows = append(rows, finderRow{isHeader: false, matchIdx: matchIdx})
		sessionCount++
	}
	return rows
}

func (p FinderPopup) View() string {
	const popW = 78
	cw := popW - 4

	// styles
	selectedBg := lipgloss.NewStyle().Background(p.theme.Surf1).Foreground(p.theme.Text).Bold(true)
	headerSt := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	faint := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(p.theme.Subtext0)

	// query line — the text input owns the cursor and placeholder
	chevron := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›")
	rows := []string{"", chevron + " " + p.input.View(), ""}

	matches := p.matches()
	if len(matches) == 0 {
		rows = append(rows, faint.Render("no matching sessions"))
	} else {
		const maxSessions = 12
		for _, r := range p.visibleRows(matches, maxSessions) {
			if r.isHeader {
				// Grey non-selectable project group header.
				rows = append(rows, headerSt.Render("  "+r.project))
				continue
			}

			s := p.items[matches[r.matchIdx]]
			cfgAgent := p.ctx.Config.Agents[s.Agent]
			badge := "[" + cfgAgent.Label + "]"

			st := shortStateLabel(s.State)
			ageStr := sessionAge(s)
			name := truncateWidth(s.Name, 32)

			if r.matchIdx == p.cursor {
				// Plain unstyled string inside selectedBg so the background
				// fills the full row width — ANSI spans from sub-styles break
				// lipgloss background propagation.
				plainL := "    " + badge + " " + name
				plainR := fmt.Sprintf("%-4s", st) + "  " + ageStr
				gap := cw - len(plainL) - len(plainR)
				if gap < 1 {
					gap = 1
				}
				plain := plainL + strings.Repeat(" ", gap) + plainR
				rows = append(rows, selectedBg.Width(cw).Render(plain))
			} else {
				agentLbl := lipgloss.NewStyle().Foreground(lipgloss.Color(cfgAgent.Color)).Bold(true).Render(badge)
				stColor := p.theme.StateColor(string(s.State))
				stLabel := lipgloss.NewStyle().Foreground(stColor).Render(st)
				left := "    " + agentLbl + " " + dimmed.Render(name)
				right := stLabel + "  " + faint.Render(ageStr)
				gap := cw - lipgloss.Width(left) - lipgloss.Width(right)
				if gap < 1 {
					gap = 1
				}
				rows = append(rows, left+strings.Repeat(" ", gap)+right)
			}
		}

		if len(matches) > maxSessions {
			rows = append(rows, faint.Render(fmt.Sprintf("  … and %d more — refine your query", len(matches)-maxSessions)))
		}
	}

	// Pad the result list to a stable minimum so the popup doesn't jump as
	// matches change. The frame later adds the top border, title, and its
	// divider (3 lines) above these rows; the original bookkeeping padded until
	// those 3 plus the body reached 14, then appended one final blank.
	for len(rows) < 11 {
		rows = append(rows, "")
	}
	rows = append(rows, "")

	// footer — a centered three-part hint, pre-built to the full content width.
	footLeft := faint.Render("esc cancel")
	footMid := faint.Render("↑/↓ move")
	footRight := faint.Render("↵ attach")
	footTotal := lipgloss.Width(footLeft) + lipgloss.Width(footMid) + lipgloss.Width(footRight)
	footSpace := cw - footTotal
	if footSpace < 4 {
		footSpace = 4
	}
	pad1 := footSpace / 2
	pad2 := footSpace - pad1
	footLine := footLeft + strings.Repeat(" ", pad1) + footMid + strings.Repeat(" ", pad2) + footRight
	if footPad := cw - lipgloss.Width(footLine); footPad > 0 {
		footLine += strings.Repeat(" ", footPad)
	}

	return drawFrame(frameSpec{
		Width:    popW,
		Title:    lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Find Session"),
		Hint:     lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("attach to live agent"),
		Border:   popupBorderStyle(p.theme),
		Sections: [][]string{rows, {footLine}},
	})
}
