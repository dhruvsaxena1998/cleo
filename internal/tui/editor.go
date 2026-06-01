package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/editoropen"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
)

type editorLauncher interface {
	StartDetached(*exec.Cmd) error
}

type processEditorLauncher struct{}

func (processEditorLauncher) StartDetached(cmd *exec.Cmd) error {
	return cmd.Start()
}

type editorFinishedMsg struct{ err error }

func (m Model) openSelectedProjectInEditor() (Model, tea.Cmd) {
	project, ok := m.selectedProjectForEditor()
	if !ok {
		m.status = "select a Project first"
		return m, nil
	}
	plan, err := editoropen.Plan(editoropen.Config{
		UIEditor:    m.ctx.Config.UI.Editor,
		EnvEditor:   os.Getenv("EDITOR"),
		ProjectPath: project.Path,
	})
	if err != nil {
		switch {
		case errors.Is(err, editoropen.ErrNoEditor):
			m.status = "no editor configured; set ui.editor or EDITOR"
		case errors.Is(err, editoropen.ErrUnsupported):
			m.status = fmt.Sprintf("unsupported editor %q; set ui.editor to a supported editor", firstWordForStatus(m.ctx.Config.UI.Editor, os.Getenv("EDITOR")))
		default:
			m.status = fmt.Sprintf("open editor failed: %v", err)
		}
		return m, nil
	}

	cmd := plan.Command()
	if plan.Mode == editoropen.ModeTerminal {
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return editorFinishedMsg{err: err}
			}
			return nil
		})
	}

	if m.editorLauncher == nil {
		m.editorLauncher = processEditorLauncher{}
	}
	if err := m.editorLauncher.StartDetached(cmd); err != nil {
		m.status = fmt.Sprintf("open editor failed: %v", err)
		return m, nil
	}
	m.status = fmt.Sprintf("opening Project %s in editor", project.ID)
	m.statusTimerID++
	return m, statusExpiryCmd(m.statusTimerID)
}

func (m Model) selectedProjectForEditor() (projects.Project, bool) {
	if sess, ok := m.sessionAtCursor(); ok {
		return m.projectByID(sess.ProjectID)
	}
	pid, ok := m.projectAtCursor()
	if !ok {
		return projects.Project{}, false
	}
	return m.projectByID(pid)
}

func (m Model) projectByID(pid string) (projects.Project, bool) {
	for _, p := range m.projects {
		if p.ID == pid {
			return p, true
		}
	}
	return projects.Project{}, false
}

func firstWordForStatus(uiEditor, envEditor string) string {
	if uiEditor != "" {
		return uiEditor
	}
	return envEditor
}
