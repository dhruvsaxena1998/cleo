package serve

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestViewJSONNeverLeaksIDOrLastMessage(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	sessions := []state.Session{{
		ID:          "cleo-myapp-claude-SECRETID",
		ProjectID:   "myapp",
		Agent:       "claude",
		Name:        "checkout",
		State:       state.Running,
		LastEventAt: now,
		LastMessage: "API_KEY=sk-LEAKED-secret-value",
	}}

	b, err := json.Marshal(Project(sessions, now))
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, forbidden := range []string{"SECRETID", "LEAKED", "last_message", "\"id\""} {
		if strings.Contains(out, forbidden) {
			t.Errorf("view JSON leaked %q: %s", forbidden, out)
		}
	}
}

func TestSessionsSortByUrgencyThenAge(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	sessions := []state.Session{
		{ProjectID: "p", Agent: "a", Name: "idle-old", State: state.Idle, LastEventAt: now.Add(-10 * time.Minute)},
		{ProjectID: "p", Agent: "a", Name: "running", State: state.Running, LastEventAt: now.Add(-1 * time.Minute)},
		{ProjectID: "p", Agent: "a", Name: "waiting", State: state.WaitingForInput, LastEventAt: now.Add(-5 * time.Minute)},
		{ProjectID: "p", Agent: "a", Name: "running-newer", State: state.Running, LastEventAt: now.Add(-30 * time.Second)},
	}

	v := Project(sessions, now)
	got := []string{}
	for _, s := range v.Projects[0].Sessions {
		got = append(got, s.Name)
	}
	// waiting_for_input first; then running ordered by age ascending (newer
	// first); idle last.
	want := []string{"waiting", "running-newer", "running", "idle-old"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestProjectsOrderByMostUrgentSession(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	// "calm" appears first in input but holds only a running session;
	// "urgent" holds a waiting_for_input and must float to the top.
	sessions := []state.Session{
		{ProjectID: "calm", Agent: "a", Name: "r", State: state.Running, LastEventAt: now},
		{ProjectID: "urgent", Agent: "a", Name: "idle", State: state.Idle, LastEventAt: now},
		{ProjectID: "urgent", Agent: "a", Name: "waits", State: state.WaitingForInput, LastEventAt: now},
	}

	v := Project(sessions, now)
	if v.Projects[0].Project != "urgent" {
		t.Errorf("first project = %q, want urgent", v.Projects[0].Project)
	}
	if v.Projects[1].Project != "calm" {
		t.Errorf("second project = %q, want calm", v.Projects[1].Project)
	}
}

func TestAttentionFlagsAndNeedCount(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	sessions := []state.Session{
		{ProjectID: "p1", Agent: "a", Name: "waits", State: state.WaitingForInput, LastEventAt: now},
		{ProjectID: "p1", Agent: "a", Name: "runs", State: state.Running, LastEventAt: now},
		{ProjectID: "p2", Agent: "a", Name: "broke", State: state.Errored, LastEventAt: now},
		{ProjectID: "p3", Agent: "a", Name: "chill", State: state.Idle, LastEventAt: now},
	}

	v := Project(sessions, now)

	// waiting_for_input + error are the only loud states.
	if v.NeedCount != 2 {
		t.Errorf("NeedCount = %d, want 2", v.NeedCount)
	}

	byName := map[string]ViewSession{}
	attnByProject := map[string]bool{}
	for _, g := range v.Projects {
		attnByProject[g.Project] = g.NeedsAttention
		for _, s := range g.Sessions {
			byName[s.Name] = s
		}
	}
	if !byName["waits"].Attn || !byName["broke"].Attn {
		t.Error("waiting_for_input and error sessions must be attn=true")
	}
	if byName["runs"].Attn || byName["chill"].Attn {
		t.Error("running and idle sessions must be attn=false")
	}
	if !attnByProject["p1"] || !attnByProject["p2"] {
		t.Error("projects with a loud session must have NeedsAttention=true")
	}
	if attnByProject["p3"] {
		t.Error("a calm project must have NeedsAttention=false")
	}
}

func TestProjectExposesOnlySafeFields(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	sessions := []state.Session{
		{
			ID:          "cleo-myapp-claude-1",
			ProjectID:   "myapp",
			Agent:       "claude",
			Name:        "checkout-flow",
			State:       state.Running,
			LastEventAt: now.Add(-90 * time.Second),
			LastMessage: "SECRET token=hunter2",
		},
	}

	v := Project(sessions, now)

	if len(v.Projects) != 1 {
		t.Fatalf("want 1 project group, got %d", len(v.Projects))
	}
	g := v.Projects[0]
	if g.Project != "myapp" {
		t.Errorf("project = %q, want myapp", g.Project)
	}
	if len(g.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(g.Sessions))
	}
	s := g.Sessions[0]
	if s.Agent != "claude" || s.Name != "checkout-flow" || s.State != "running" {
		t.Errorf("session fields wrong: %+v", s)
	}
	if s.AgeSeconds != 90 {
		t.Errorf("age = %ds, want 90", s.AgeSeconds)
	}
}
