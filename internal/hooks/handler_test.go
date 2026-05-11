package hooks

import (
	"encoding/json"
	"errors"
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
		Now:    func() (string, error) { return "cleo-x-claude-1", nil },
	}
	return deps, st, p
}

type noopPlayer struct{}

func (noopPlayer) Play(string) error { return nil }
func (noopPlayer) Available() bool   { return false }

type recordingPlayer struct{ played []string }

func (p *recordingPlayer) Play(file string) error {
	p.played = append(p.played, file)
	return nil
}
func (*recordingPlayer) Available() bool { return true }

func TestClaudePreToolUseTransitions(t *testing.T) {
	deps, st, _ := setup(t)
	if err := Handle(deps, "claude", "PreToolUse", []byte(`{"tool_name":"Bash"}`)); err != nil {
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
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Approve Bash command?"}`)); err != nil {
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
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
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
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
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
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
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
	if err := Handle(deps, "claude", "UserPromptSubmit", []byte(`{"cwd":"/tmp/myproject"}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running after prompt submit, got %s", got.State)
	}
}

func TestClaudeStandaloneSessionIgnoredWhenNoEnvVar(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		t.Errorf("FindByCwd must not be called for claude protocol")
		return "cleo-x-claude-1", nil
	}
	_ = Handle(deps, "claude", "Stop", []byte(`{"cwd":"/tmp/myproject"}`))
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected state unchanged (Running), got %s", got.State)
	}
}

func TestCodexPermissionRequestSetsWaitingForInput(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/foo"}}`)
	if err := Handle(deps, "codex", "PermissionRequest", payload); err != nil {
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
	if err := Handle(deps, "codex", "UserPromptSubmit", []byte(`{"cwd":"/tmp/myproject"}`)); err != nil {
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
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd == "/tmp/myproject" && agent == "codex" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}
	payload := []byte(`{"cwd":"/tmp/myproject","tool_name":"Bash"}`)
	if err := Handle(deps, "codex", "PreToolUse", payload); err != nil {
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
	_ = Handle(deps, "codex", "Stop", []byte(`{}`))
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Idle {
		t.Errorf("expected idle via process cwd fallback, got %s", got.State)
	}
}

func TestHandleUnknownProtocolLogsError(t *testing.T) {
	deps, _, p := setup(t)
	_ = Handle(deps, "unknown-proto", "SomeEvent", []byte(""))
	b, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(b), "unknown-proto") {
		t.Errorf("expected protocol name in error log, got: %s", string(b))
	}
}

func TestResolveSession_CwdFallbackNotCalledForClaude(t *testing.T) {
	deps, _, _ := setup(t)
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	called := false
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		called = true
		return "", nil
	}
	_ = Handle(deps, "claude", "PreToolUse", []byte(`{"cwd":"/proj"}`))
	if called {
		t.Error("FindByCwd must not be called for ClaudeProtocol (UsesCwdFallback=false)")
	}
}

func TestFallbackReasonEnvPresent(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "cleo-x-claude-1", nil }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_present" {
		t.Errorf("fallback_reason: want env_present, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvMissing(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_missing" {
		t.Errorf("fallback_reason: want env_missing, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvUnknownSession(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "stale-sid", nil }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
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
	_ = Handle(d, "codex", "PreToolUse", []byte(`{"cwd":"/some/path"}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "no_match" {
		t.Errorf("fallback_reason: want no_match, got %q", row.FallbackReason)
	}
	errLog, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(errLog), "/some/path") {
		t.Errorf("expected cwd in error log, got: %s", string(errLog))
	}
}

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

func TestClaudeStaleSidFallsBackToCwd(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	// Simulate: env var is set but points to a session not in the store (stale).
	deps.Now = func() (string, error) { return "stale-session-id", nil }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd == "/tmp/myproject" && agent == "claude" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}

	payload := []byte(`{"cwd":"/tmp/myproject","message":"Need approval"}`)
	if err := Handle(deps, "claude", "Notification", payload); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("expected WaitingForInput via stale-sid CWD fallback, got %s", got.State)
	}
}

var errNoSessionTest = errors.New("no session")

type errStateStore struct{ inner *state.Store }

func (e *errStateStore) Apply(_ string, _ state.Event, _ string) (state.Session, error) {
	return state.Session{}, fmt.Errorf("disk full")
}
func (e *errStateStore) Get(id string) (state.Session, error) { return e.inner.Get(id) }

func TestSoundPlaysEvenWhenStateApplyFails(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.State = &errStateStore{inner: st}

	err := Handle(deps, "claude", "Notification", []byte(`{"message":"Need approval"}`))
	if err == nil {
		t.Error("expected error from failed state apply")
	}
	if len(player.played) != 1 {
		t.Errorf("expected sound to play despite state error, played %v", player.played)
	}
}

func TestIdleNudgeNotificationDoesNotPlaySound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")
	// Simulate Claude's ~60s idle nudge arriving after Stop.
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Claude is waiting for your input"}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("idle-nudge Notification (from Idle state) must not play sound, played %v", player.played)
	}
	// State transition to WaitingForInput must still happen for TUI visibility.
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("state should be WaitingForInput after Notification, got %s", got.State)
	}
}

func TestGenuineNotificationFromRunningPlaysSound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	// Session is Running — Claude needs tool approval (genuine blocking request).
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Approve Bash command?"}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 1 {
		t.Errorf("genuine Notification (from Running state) must play sound, played %v", player.played)
	}
}

func TestCodexPermissionRequestFromIdleStillPlaysSound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")
	// Codex PermissionRequest is a genuine blocking request, not an idle nudge.
	// Sound must play even when the session is Idle.
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/foo"}}`)
	if err := Handle(deps, "codex", "PermissionRequest", payload); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 1 {
		t.Errorf("Codex PermissionRequest from Idle state must play sound, played %v", player.played)
	}
}
