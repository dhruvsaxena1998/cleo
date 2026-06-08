package tui

import (
	"runtime"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"

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

type cursor struct {
	projectIdx int
	agentIdx   int // -1 = on the project row
}

func New(ctx *cli.Ctx) Model {
	m := Model{
		ctx:            ctx,
		theme:          Resolve(ctx.Config.UI.Theme),
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
