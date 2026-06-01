package editoropen

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Mode int

const (
	ModeDetached Mode = iota + 1
	ModeTerminal
)

var (
	ErrNoEditor    = errors.New("no editor configured")
	ErrUnsupported = errors.New("unsupported editor")
)

type Config struct {
	UIEditor    string
	EnvEditor   string
	ProjectPath string
}

type PlanResult struct {
	Mode          Mode
	EditorCommand string
	ProjectPath   string
	ShellCommand  string
}

func Plan(c Config) (PlanResult, error) {
	editor := strings.TrimSpace(c.UIEditor)
	if editor == "" {
		editor = strings.TrimSpace(c.EnvEditor)
	}
	if editor == "" {
		return PlanResult{}, ErrNoEditor
	}
	mode, ok := classify(editor)
	if !ok {
		return PlanResult{}, ErrUnsupported
	}
	return PlanResult{
		Mode:          mode,
		EditorCommand: editor,
		ProjectPath:   c.ProjectPath,
		ShellCommand:  editor + ` "$CLEO_PROJECT_PATH"`,
	}, nil
}

func (p PlanResult) Command() *exec.Cmd {
	cmd := exec.Command("sh", "-c", p.ShellCommand)
	cmd.Env = append(os.Environ(), "CLEO_PROJECT_PATH="+p.ProjectPath)
	return cmd
}

func classify(command string) (Mode, bool) {
	word := firstShellWord(command)
	if word == "" {
		return 0, false
	}
	name := strings.ToLower(filepath.Base(word))
	if terminalEditors[name] {
		return ModeTerminal, true
	}
	if guiEditors[name] {
		return ModeDetached, true
	}
	return 0, false
}

func firstShellWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	quote := byte(0)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				continue
			}
			b.WriteByte(c)
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

var terminalEditors = map[string]bool{
	"vi":    true,
	"vim":   true,
	"nvim":  true,
	"nano":  true,
	"emacs": true,
	"hx":    true,
	"helix": true,
	"micro": true,
}

var guiEditors = map[string]bool{
	"code":     true,
	"cursor":   true,
	"zed":      true,
	"subl":     true,
	"open":     true,
	"mate":     true,
	"bbedit":   true,
	"idea":     true,
	"goland":   true,
	"webstorm": true,
	"pycharm":  true,
	"phpstorm": true,
	"rubymine": true,
	"clion":    true,
	"rider":    true,
	"datagrip": true,
	"appcode":  true,
	"fleet":    true,
}
