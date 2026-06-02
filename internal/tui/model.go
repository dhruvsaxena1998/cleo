package tui

import (
	"runtime"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
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

func (m *Model) clearStatus() {
	if m.status == "" {
		return
	}
	m.status = ""
	m.statusTimerID++
}
