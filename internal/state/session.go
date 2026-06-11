package state

import "time"

type State string

const (
	Spawning        State = "spawning"
	Running         State = "running"
	WaitingForInput State = "waiting_for_input"
	Idle            State = "idle"
	Completed       State = "completed"
	Errored         State = "error"
	Dead            State = "dead"
)

func (s State) IsFinished() bool {
	return s == Completed || s == Errored || s == Dead
}

type Event string

const (
	EvSessionStart Event = "session_start"
	EvPreToolUse   Event = "pre_tool_use"
	EvPostToolUse  Event = "post_tool_use"
	EvNotification Event = "notification"
	EvStop         Event = "stop"
	EvSessionEnd   Event = "session_end"
	EvUserResume   Event = "user_resume"  // synthesized: input followed by activity
	EvIdleTimeout  Event = "idle_timeout" // synthesized: reconciler
	EvError        Event = "error"
	EvDead         Event = "dead" // synthesized: tmux session gone
)

type Session struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Agent       string    `json:"agent"`
	Name        string    `json:"name"`
	State       State     `json:"state"`
	StartedAt   time.Time `json:"started_at"`
	LastEventAt time.Time `json:"last_event_at"`
	LastMessage string    `json:"last_message,omitempty"`
	ToolCount   int       `json:"tool_count"`
	// Worktree fields are persisted at creation for worktree-backed Sessions so
	// cleanup and display never re-derive them. Empty for main-tree Sessions.
	WorktreePath   string `json:"worktree_path,omitempty"`
	WorktreeBranch string `json:"worktree_branch,omitempty"`
}

// HasWorktree reports whether the Session is backed by a Cleo-managed Worktree.
func (s Session) HasWorktree() bool { return s.WorktreePath != "" }
