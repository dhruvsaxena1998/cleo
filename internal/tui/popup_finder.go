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

// FinderPopup is a center-screen fuzzy finder for attachable sessions.
type FinderPopup struct {
	ctx     *cli.Ctx
	theme   Theme
	query   string
	cursor  int // index into visible results
	items   []state.Session // all attachable sessions
	sources []string        // parallel to items, for fuzzy matching
}

// NewFinderPopup builds a finder over all non-finished sessions.
func NewFinderPopup(ctx *cli.Ctx, theme Theme, sessions []state.Session) FinderPopup {
	var items []state.Session
	var sources []string
	for _, s := range sessions {
		if s.State.IsFinished() {
			continue
		}
		items = append(items, s)
		sources = append(sources, s.Name+" "+s.ProjectID+" "+s.Agent)
	}
	// stable sort by project then name so identical scores group predictably
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProjectID != items[j].ProjectID {
			return items[i].ProjectID < items[j].ProjectID
		}
		return items[i].Name < items[j].Name
	})
	// re-order sources to match
	sources = make([]string, len(items))
	for i, it := range items {
		sources[i] = it.Name + " " + it.ProjectID + " " + it.Agent
	}
	return FinderPopup{
		ctx:     ctx,
		theme:   theme,
		items:   items,
		sources: sources,
	}
}

func (p FinderPopup) Init() tea.Cmd {
	return textinput.Blink
}

func (p FinderPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return p, func() tea.Msg { return FinderCancelled{} }
		case tea.KeyEnter:
			if sel, ok := p.selected(); ok {
				return p, func() tea.Msg { return FinderSubmitted{SessionID: sel.ID} }
			}
		case tea.KeyBackspace:
			if len(p.query) > 0 {
				p.query = p.query[:len(p.query)-1]
			}
			p.clampCursor()
			return p, nil
		case tea.KeyRunes:
			p.query += string(msg.Runes)
			p.cursor = 0
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
	return p, nil
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
	if p.query == "" {
		out := make([]int, len(p.items))
		for i := range p.items {
			out[i] = i
		}
		return out
	}
	results := fuzzy.Find(p.query, p.sources)
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

func (p FinderPopup) View() string {
	const popW = 72
	iw := popW - 2
	cw := iw - 2

	bdr := popupBorderStyle(p.theme)

	var b strings.Builder
	hbar := strings.Repeat("─", iw)
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")

	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Find Session")
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("attach to live agent")
	gap := cw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	blank := func() {
		b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	}
	row := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}

	blank()

	// query line
	q := p.query
	if q == "" {
		q = "type to filter sessions"
	}
	queryLine := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " +
		lipgloss.NewStyle().Foreground(p.theme.Text).Render(q)
	if p.query != "" {
		queryLine = lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " +
			lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true).Render(p.query+"▌")
	}
	row(queryLine)
	blank()

	selectedSt := lipgloss.NewStyle().Background(p.theme.Surf0).Foreground(p.theme.Text).Bold(true)
	faint := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(p.theme.Subtext0)

	matches := p.matches()
	if len(matches) == 0 {
		row(faint.Render("no matching sessions"))
	} else {
		for i, idx := range matches {
			if i >= 12 { // max visible results to keep popup reasonable
				more := fmt.Sprintf("  … and %d more", len(matches)-i)
				row(faint.Render(more))
				break
			}
			s := p.items[idx]
			cfgAgent := p.ctx.Config.Agents[s.Agent]
			badge := "[" + cfgAgent.Label + "]"
			agentLbl := lipgloss.NewStyle().Foreground(lipgloss.Color(cfgAgent.Color)).Bold(true).Render(badge)

			stColor := p.theme.StateColor(string(s.State))
			stLabel := lipgloss.NewStyle().Foreground(stColor).Render(shortStateLabel(s.State))
			ageStr := sessionAge(s)

			left := agentLbl + " " + dimmed.Render(truncateWidth(s.Name, 24))
			right := faint.Render(s.ProjectID) + "  " + stLabel + "  " + faint.Render(ageStr)
			inner := left + strings.Repeat(" ", cw-lipgloss.Width(left)-lipgloss.Width(right)) + right

			if i == p.cursor {
				row(selectedSt.Width(cw).Render(inner))
			} else {
				row(inner)
			}
		}
	}

	// fill remaining height to reach a minimum pleasant height
	linesSoFar := strings.Count(b.String(), "\n")
	minContentLines := 8
	for linesSoFar < minContentLines+3 { // +3 for header rows
		blank()
		linesSoFar++
	}
	blank()

	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	footLeft := faint.Render("esc cancel")
	footMid := faint.Render("j/k move")
	footRight := faint.Render("↵ attach")
	footGap1 := cw - lipgloss.Width(footLeft) - lipgloss.Width(footMid) - lipgloss.Width(footRight)
	if footGap1 < 2 {
		footGap1 = 2
	}
	footGap2 := 2
	footLine := footLeft + strings.Repeat(" ", footGap1/2) + footMid + strings.Repeat(" ", footGap2) + footRight
	// pad to full width
	footPad := cw - lipgloss.Width(footLine)
	if footPad > 0 {
		footLine += strings.Repeat(" ", footPad)
	}
	b.WriteString(bdr.Render("│") + " " + footLine + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("└"+hbar+"┘"))
	return b.String()
}
