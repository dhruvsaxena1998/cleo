package editoropen

import (
	"errors"
	"strings"
	"testing"
)

func TestPlanUsesConfiguredEditorOverEnvAndAppendsProjectPath(t *testing.T) {
	plan, err := Plan(Config{
		UIEditor:    "code --reuse-window",
		EnvEditor:   "nvim",
		ProjectPath: "/tmp/my project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != ModeDetached {
		t.Fatalf("mode = %v, want detached", plan.Mode)
	}
	if plan.EditorCommand != "code --reuse-window" {
		t.Fatalf("editor command = %q", plan.EditorCommand)
	}
	if plan.ProjectPath != "/tmp/my project" {
		t.Fatalf("project path = %q", plan.ProjectPath)
	}
	if plan.ShellCommand != `code --reuse-window "$CLEO_PROJECT_PATH"` {
		t.Fatalf("shell command = %q", plan.ShellCommand)
	}
}

func TestPlanUsesEnvEditorWhenConfigIsEmpty(t *testing.T) {
	plan, err := Plan(Config{EnvEditor: "/usr/bin/nvim -p", ProjectPath: "/tmp/project"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != ModeTerminal {
		t.Fatalf("mode = %v, want terminal", plan.Mode)
	}
	if plan.EditorCommand != "/usr/bin/nvim -p" {
		t.Fatalf("editor command = %q", plan.EditorCommand)
	}
}

func TestPlanRejectsMissingAndUnsupportedEditors(t *testing.T) {
	if _, err := Plan(Config{ProjectPath: "/tmp/project"}); !errors.Is(err, ErrNoEditor) {
		t.Fatalf("missing editor error = %v, want ErrNoEditor", err)
	}
	if _, err := Plan(Config{UIEditor: "mystery-editor", ProjectPath: "/tmp/project"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported editor error = %v, want ErrUnsupported", err)
	}
}

func TestPlanClassifiesFirstShellWordBasenameOnly(t *testing.T) {
	cases := []struct {
		command string
		mode    Mode
	}{
		{"/opt/bin/code --reuse-window", ModeDetached},
		{"open -a \"Visual Studio Code\"", ModeDetached},
		{"emacs -nw", ModeTerminal},
		{"hx --config /tmp/helix.toml", ModeTerminal},
	}
	for _, tc := range cases {
		plan, err := Plan(Config{UIEditor: tc.command, ProjectPath: "/tmp/project"})
		if err != nil {
			t.Fatalf("%q: %v", tc.command, err)
		}
		if plan.Mode != tc.mode {
			t.Fatalf("%q mode = %v, want %v", tc.command, plan.Mode, tc.mode)
		}
		if !strings.HasPrefix(plan.ShellCommand, tc.command) {
			t.Fatalf("%q was not preserved in shell command %q", tc.command, plan.ShellCommand)
		}
	}
}

func TestCommandPassesProjectPathThroughEnvironment(t *testing.T) {
	plan, err := Plan(Config{UIEditor: "code", ProjectPath: "/tmp/my project"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := plan.Command()
	if got := strings.Join(cmd.Args, " "); got != `sh -c code "$CLEO_PROJECT_PATH"` {
		t.Fatalf("args = %q", got)
	}
	if !containsEnv(cmd.Env, "CLEO_PROJECT_PATH=/tmp/my project") {
		t.Fatalf("CLEO_PROJECT_PATH missing from env: %v", cmd.Env)
	}
}

func containsEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}
