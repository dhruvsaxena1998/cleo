package tui

import (
	"runtime"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type Model struct {
	ctx            *cli.Ctx
	theme          Theme
	projects       []projects.Project
	sessions       []state.Session
	cursor         cursor
	expanded       map[string]bool   // project id → expanded
	paneCache      map[string]string // session id → last captured pane content
	selected       string            // session id selected for "v" view; "" = none
	status         string
	statusTimerID  int
	mode           Mode
	popup          tea.Model
	help           help.Model
	editorLauncher editorLauncher
	width, height  int
	err            error

	// settingsBackup is the config snapshot taken when the settings popup
	// opens. Edits preview live against ctx.Config; cancelling (esc) restores
	// this snapshot. See openSettingsPopup and the Esc handler in handleKey.
	settingsBackup config.Config

	// paneCaptureInFlight is true between dispatching a capturePaneCmd and
	// receiving the corresponding paneCapturedMsg. The selection-driven
	// preview ticker uses it to avoid dispatching overlapping captures.
	paneCaptureInFlight bool

	// firstStateLoaded flips to true after the first stateLoadedMsg is
	// processed. The handler uses it to fire one immediate pane capture on
	// startup instead of waiting for the first previewTickCmd interval.
	firstStateLoaded bool

	heapAlloc     uint64 // updated once per state tick via runtime.ReadMemStats
	agentMemAlloc uint64 // combined RSS of all agent process trees (bytes)

	// animFrame advances on each animTick and drives the "working" pulse — the
	// running/spawning markers breathe (see styles.go pulseColor). animTicking
	// guards the ~120ms loop so it runs only while a working session exists and
	// is never double-armed: stateLoadedMsg starts it when work appears, and the
	// tick stops re-arming when work is gone.
	animFrame   int
	animTicking bool
}

// hasWorkingSession reports whether any session is actively working, gating the
// pulse animation loop so an idle dashboard does not re-render on a timer.
func (m Model) hasWorkingSession() bool {
	for _, s := range m.sessions {
		if s.State == state.Running || s.State == state.Spawning {
			return true
		}
	}
	return false
}

func readHeapAlloc() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

type Mode int

const (
	ModeNormal Mode = iota
	ModePopup
)

func New(ctx *cli.Ctx) Model {
	// Initialise the global bubblezone manager so View() can mark/scan clickable
	// regions. Done here (not in Run) so tests that construct a Model directly
	// also get a live manager; re-creating per-Model is cheap.
	zone.NewGlobal()
	theme := Resolve(ctx.Config.UI.Theme)
	theme.Icons = resolveIcons(ctx.Config.UI.Icons)
	m := Model{
		ctx:            ctx,
		theme:          theme,
		expanded:       map[string]bool{},
		paneCache:      map[string]string{},
		help:           help.New(),
		editorLauncher: processEditorLauncher{},
		heapAlloc:      readHeapAlloc(),
	}
	if len(ctx.Config.Warnings) > 0 {
		m.popup = NewWarningsPopup(m.theme, ctx.Config.Diagnostics)
		m.mode = ModePopup
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadStateCmd(m.ctx),
		tickStateCmd(),
		previewTickCmd(m.ctx.Config.UI.PanePreview.Interval),
		agentMemTickCmd(),
	)
}

// setStatus is the single path for showing an explicit status message. It sets
// the message, advances the timer id so any older expiry tick can no longer
// clear this newer message, and returns the expiry command built from the
// configured status timeout. Every explicit status assignment must go through
// here so auto-expiry behaves identically across all Dashboard actions.
func (m *Model) setStatus(msg string) tea.Cmd {
	m.status = msg
	m.statusTimerID++
	return statusExpiryCmd(m.statusTimerID, m.ctx.Config.UI.StatusTimeout())
}

func (m *Model) clearStatus() {
	if m.status == "" {
		return
	}
	m.status = ""
	m.statusTimerID++
}

// applyTheme switches the live theme and returns a command to re-sync the
// terminal background (OSC 11) when the base colour actually changes. The
// background is set once at startup (Run); without this, a runtime theme change
// recolours the lipgloss-rendered cells but leaves the terminal background on
// the old theme until the program is restarted. Returns nil when the base is
// unchanged so non-theme edits don't write escape sequences on every keystroke.
func (m *Model) applyTheme(name string) tea.Cmd {
	prev := m.theme.Base
	m.theme = Resolve(name)
	// Re-resolve the glyph set from the live config rather than carrying the old
	// one, so the settings editor's icons field previews/saves/reverts through
	// the same path as theme. Every caller updates ctx.Config before calling.
	m.theme.Icons = resolveIcons(m.ctx.Config.UI.Icons)
	if m.theme.Base == prev {
		return nil
	}
	return setBackgroundCmd(m.theme.Base)
}
