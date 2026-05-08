package hooks

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func setup(t *testing.T) (Deps, *state.Store, paths.Paths) {
	root := t.TempDir()
	p := paths.NewWithRoot(root)
	st := state.NewStore(p.StateFile(), p.StateLock())
	_ = st.Put(state.Session{ID: "cleo-x-claude-1", Agent: "claude", State: state.Spawning})
	cfg, _ := config.Load(p.ConfigFile())
	deps := Deps{
		Paths:  p,
		State:  st,
		Config: cfg,
		Events: func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
		Sound:  noopPlayer{},
		Now:    func() (string, error) { return "cleo-x-claude-1", nil }, // sid
	}
	return deps, st, p
}

type noopPlayer struct{}

func (noopPlayer) Play(string) error { return nil }
func (noopPlayer) Available() bool   { return false }

type recordingPlayer struct {
	played []string
}

func (p *recordingPlayer) Play(file string) error {
	p.played = append(p.played, file)
	return nil
}

func (*recordingPlayer) Available() bool { return true }

func TestClaudePreToolUseTransitions(t *testing.T) {
	deps, st, _ := setup(t)
	in := strings.NewReader(`{"tool_name":"Bash"}`)
	out := &bytes.Buffer{}
	if err := Handle(deps, "claude", "PreToolUse", in, out); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("state %s", got.State)
	}
}

func TestClaudeNotificationSetsLastMessage(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	in := strings.NewReader(`{"message":"Approve Bash command?"}`)
	if err := Handle(deps, "claude", "Notification", in, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("state %s", got.State)
	}
	if got.LastMessage == "" {
		t.Errorf("last message empty")
	}
}

func TestDisabledSoundEventDoesNotPlay(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Config.Sound.EventEnabled["session_completed"] = false
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", strings.NewReader(`{}`), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("expected no sound, played %v", player.played)
	}
}

func TestFocusedSessionDoesNotPlaySound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Focused = func(sid string) bool { return sid == "cleo-x-claude-1" }
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", strings.NewReader(`{}`), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("expected no sound for focused session, played %v", player.played)
	}
}

func TestEnabledSoundEventPlays(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", strings.NewReader(`{}`), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 1 {
		t.Fatalf("expected one sound, played %v", player.played)
	}
	if !strings.HasSuffix(player.played[0], "done.wav") {
		t.Errorf("expected done.wav, got %q", player.played[0])
	}
}

func TestClaudeUserPromptSubmitResumesRunning(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")

	if err := Handle(deps, "claude", "UserPromptSubmit", strings.NewReader(`{"cwd":"/tmp/myproject"}`), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running after prompt submit, got %s", got.State)
	}
}

// TestClaudeStandaloneSessionIgnoredWhenNoEnvVar verifies that a hook event
// from a standalone Claude session (CLEO_SESSION_ID absent) is NOT attributed
// to an active cleo session via FindByCwd. Claude propagates env to hook
// subprocesses, so absent CLEO_SESSION_ID means genuinely standalone.
func TestClaudeStandaloneSessionIgnoredWhenNoEnvVar(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		// Should never be called for the claude protocol.
		t.Errorf("FindByCwd must not be called for claude protocol")
		return "cleo-x-claude-1", nil
	}

	payload := `{"cwd":"/tmp/myproject","hook_event_name":"Stop"}`
	if err := Handle(deps, "claude", "Stop", strings.NewReader(payload), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	// State must remain Running — the Stop event must be ignored because the
	// hook could not be attributed to any cleo session.
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected state unchanged (Running), got %s", got.State)
	}
}

func TestCodexPermissionRequestSetsWaitingForInput(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/foo"}}`
	if err := Handle(deps, "codex", "PermissionRequest", strings.NewReader(payload), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("expected waiting_for_input, got %s", got.State)
	}
	if got.LastMessage == "" {
		t.Errorf("last message should be set from tool input command")
	}
}

func TestCodexUserPromptSubmitResumesRunning(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")

	if err := Handle(deps, "codex", "UserPromptSubmit", strings.NewReader(`{"cwd":"/tmp/myproject"}`), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running after prompt submit, got %s", got.State)
	}
}

func TestCodexCwdFallbackWhenNoEnvVar(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	// Simulate CLEO_SESSION_ID not in env: Now returns an error.
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	// Fallback: FindByCwd resolves to our test session.
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd == "/tmp/myproject" && agent == "codex" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}

	payload := `{"cwd":"/tmp/myproject","hook_event_name":"PreToolUse","tool_name":"Bash"}`
	if err := Handle(deps, "codex", "PreToolUse", strings.NewReader(payload), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running via cwd fallback, got %s", got.State)
	}
}

func TestCodexCwdFallbackUsesProcessWorkingDirectory(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd != "" && agent == "codex" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}

	payload := `{"hook_event_name":"Stop"}`
	if err := Handle(deps, "codex", "Stop", strings.NewReader(payload), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Idle {
		t.Errorf("expected idle via process cwd fallback, got %s", got.State)
	}
}

func TestHandleUnknownProtocolLogsError(t *testing.T) {
	deps, _, p := setup(t)
	_ = Handle(deps, "unknown-proto", "SomeEvent", strings.NewReader(""), &bytes.Buffer{})
	b, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(b), "unknown-proto") {
		t.Errorf("expected protocol name in error log, got: %s", string(b))
	}
}
