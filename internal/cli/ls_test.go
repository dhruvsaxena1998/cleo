package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestLsShowsProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude", Name: "1", State: state.Running})
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-myapp-claude-1": true}}

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "myapp") || !strings.Contains(out.String(), "running") {
		t.Errorf("output: %q", out.String())
	}
}

func TestLsJSONFlag(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)

	// Project with a session.
	target1 := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target1)
	_, _ = c.Projects.Add(target1)
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp",
		Agent: "claude", Name: "1", State: state.Running,
	})

	// Project with no sessions.
	target2 := filepath.Join(t.TempDir(), "emptyapp")
	_ = mkdir(target2)
	_, _ = c.Projects.Add(target2)

	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-myapp-claude-1": true}}

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.ParseFlags([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Find myapp row and emptyapp row.
	var myappRow, emptyRow map[string]interface{}
	for _, r := range rows {
		switch r["project"] {
		case "myapp":
			myappRow = r
		case "emptyapp":
			emptyRow = r
		}
	}

	if myappRow == nil {
		t.Fatal("missing myapp row")
	}
	if myappRow["state"] != "running" {
		t.Errorf("myapp state = %v, want running", myappRow["state"])
	}
	if myappRow["project"] != "myapp" {
		t.Errorf("myapp project = %v, want myapp", myappRow["project"])
	}

	if emptyRow == nil {
		t.Fatal("missing emptyapp row")
	}
	if emptyRow["project"] != "emptyapp" {
		t.Errorf("emptyapp project = %v, want emptyapp", emptyRow["project"])
	}
	// All nullable fields should be null (nil in Go's JSON decode).
	for _, field := range []string{"agent", "name", "state", "id", "started_at", "last_event_at"} {
		if v, ok := emptyRow[field]; !ok || v != nil {
			t.Errorf("emptyapp row field %q = %v, want null", field, v)
		}
	}
}

func TestLsAGEColumn(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude", Name: "1", State: state.Running})
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-myapp-claude-1": true}}

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "AGE") {
		t.Errorf("expected AGE column header in output, got:\n%s", output)
	}
}

func TestLsReconcilesMissingSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	// Seed a running session that's NOT in fake tmux's exists set.
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-ghost", ProjectID: "myapp", Agent: "claude",
		Name: "ghost", State: state.Running,
	})
	c.Tmux = &fakeTmux{exists: map[string]bool{}} // empty: ghost is missing from tmux

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got, _ := c.State.Get("cleo-myapp-claude-ghost")
	if got.State != state.Dead {
		t.Errorf("expected dead after reconcile, got %s", got.State)
	}
}
