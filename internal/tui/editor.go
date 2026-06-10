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
		return m, m.setStatus("select a Project first")
	}
	plan, err := editoropen.Plan(editoropen.Config{
		UIEditor:    m.ctx.Config.UI.Editor,
		EnvEditor:   os.Getenv("EDITOR"),
		ProjectPath: project.Path,
	})
	if err != nil {
		var statusMsg string
		switch {
		case errors.Is(err, editoropen.ErrNoEditor):
			statusMsg = "no editor configured; set ui.editor or EDITOR"
		case errors.Is(err, editoropen.ErrUnsupported):
			statusMsg = fmt.Sprintf("unsupported editor %q; set ui.editor to a supported editor", firstWordForStatus(m.ctx.Config.UI.Editor, os.Getenv("EDITOR")))
		default:
			statusMsg = fmt.Sprintf("open editor failed: %v", err)
		}
		return m, m.setStatus(statusMsg)
	}

	cmd := plan.Command()
	if plan.Mode == editoropen.ModeTerminal {
		// Always route the resume through editorFinishedMsg (even on success) so
		// Update re-arms mouse tracking, which ExecProcess leaves disabled.
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return editorFinishedMsg{err: err}
		})
	}

	if m.editorLauncher == nil {
		m.editorLauncher = processEditorLauncher{}
	}
	if err := m.editorLauncher.StartDetached(cmd); err != nil {
		return m, m.setStatus(fmt.Sprintf("open editor failed: %v", err))
	}
	return m, m.setStatus(fmt.Sprintf("opening Project %s in editor", project.ID))
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
