package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestFallbackReasonEnvPresent(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "cleo-x-claude-1", nil }
	if err := Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard); err != nil {
		t.Fatalf("handle: %v", err)
	}
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_present" {
		t.Errorf("fallback_reason: want env_present, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvMissing(t *testing.T) {
	d, _, p := setup(t)
	// Now returns errNoSession; FindByCwd is not configured (claude path).
	d.Now = func() (string, error) { return "", errNoSessionTest }
	_ = Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard)
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_missing" {
		t.Errorf("fallback_reason: want env_missing, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvUnknownSession(t *testing.T) {
	d, _, p := setup(t)
	// Now returns a sid that does not exist in the seeded store.
	d.Now = func() (string, error) { return "stale-sid", nil }
	_ = Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard)
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_unknown_session" {
		t.Errorf("fallback_reason: want env_unknown_session, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonNoMatchCodex(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	d.FindByCwd = func(cwd, agent string) (string, error) {
		return "", os.ErrNotExist
	}
	_ = Handle(d, "codex", "PreToolUse", strings.NewReader(`{"cwd":"/some/path"}`), io.Discard)
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "no_match" {
		t.Errorf("fallback_reason: want no_match, got %q", row.FallbackReason)
	}
	// no_match is the only reason that escalates to hook-errors.log; verify
	// the side effect so a regression that drops logHookErr is caught.
	errLog, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(errLog), "/some/path") {
		t.Errorf("expected cwd in error log, got: %s", string(errLog))
	}
	if !strings.Contains(string(errLog), "codex") {
		t.Errorf("expected protocol in error log, got: %s", string(errLog))
	}
}

// traceRowForTest mirrors cli.hookTraceRow; redeclared locally to avoid the
// import cycle that would otherwise pull cli into a hooks-package test.
type traceRowForTest struct {
	At              string `json:"at"`
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	Cwd             string `json:"cwd"`
	EnvSession      bool   `json:"env_session"`
	ResolvedSession string `json:"resolved_session"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason"`
}

// lastTraceRow reads the trace log and returns the last decoded row.
func lastTraceRow(t *testing.T, path string) traceRowForTest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatalf("no trace rows at %s", path)
	}
	var row traceRowForTest
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return row
}

var errNoSessionTest = errors.New("no session")
