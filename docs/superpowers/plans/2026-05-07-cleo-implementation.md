# cleo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship cleo v0.1 — a Go-based TUI/CLI session manager for AI coding agents (Claude Code, Codex), backed by tmux, with hook-driven state observation and sound feedback.

**Architecture:** Single static Go binary. Each agent runs in its own tmux session named `cleo-<project>-<agent>-<slug>`. Claude/Codex hooks invoke `cleo hook <event>`, which updates `~/.config/cleo/state.json` (atomic write + flock) and per-session events.jsonl logs. The TUI (Bubble Tea) is a reader/driver — closing it does not disrupt running agents.

**Tech Stack:** Go 1.22+, Charmbracelet (Bubble Tea, Bubbles, Lip Gloss), Cobra, BurntSushi/toml, fsnotify, gofrs/flock. Runtime requires tmux ≥ 3.0.

**Spec:** `docs/superpowers/specs/2026-05-07-cleo-design.md`

---

## File Structure

```
cleo/
├── cmd/cleo/main.go                # entrypoint; calls cli.Execute
├── go.mod / go.sum
├── Makefile                        # build, test, lint targets
├── .gitignore
├── README.md
├── internal/
│   ├── paths/                      # XDG paths, ConfigDir, StateFile, EventsDir
│   ├── ids/                        # Slugify, DedupeSlug, MakeSessionID, NextCounter
│   ├── config/                     # TOML config + embedded defaults
│   ├── projects/                   # projects.json read/write + walk-up resolver
│   ├── state/                      # session struct, state machine, store with flock
│   ├── events/                     # per-session jsonl log + archive
│   ├── tmux/                       # exec wrappers around tmux CLI
│   ├── sound/                      # player probe + fire-and-forget + embedded WAVs
│   ├── hooks/                      # cleo hook handler, claude/codex protocols, install
│   ├── reconcile/                  # tmux ls ↔ state.json sync
│   ├── tui/                        # bubbletea model, sidebar, view pane, popups
│   └── cli/                        # cobra commands
└── assets/sounds/                  # bundled WAVs (start, attention, done, error)
```

Each `internal/<pkg>/` directory contains its `<pkg>.go` (or split files), a `_test.go`, and nothing else. Tests live next to code. No public package outside `cmd/`.

## Common Patterns

**Test isolation.** Every test that touches the filesystem uses `t.TempDir()` and an injected paths root — no test ever writes to `~/.config/cleo`. Every tmux test uses a private socket: `tmux -L cleo-test-<random>`.

**Commit conventions.** Conventional commits: `feat(pkg): ...`, `fix(pkg): ...`, `test(pkg): ...`, `refactor(pkg): ...`, `chore: ...`, `docs: ...`. One commit per task by default.

**Run all tests after each task.** `go test ./...` must pass before commit. Tasks list specific test commands but `go test ./...` is the implicit gate.

**Type signatures are load-bearing.** Function names and types declared in early tasks are referenced in later tasks — keep them stable. If a task in Phase 5 calls `state.Apply(sid, event)`, that signature was set in Phase 3.

**Phase gates are mandatory.** Every phase ends with a `Phase N Gate` section: a manual checklist (run by hand against a real binary) and a bugfix sprint (TDD repros for anything the manual checks surfaced). Do not start the next phase's tasks until every box in the gate is checked and the sprint queue is empty. Automated tests catch regressions; the gate catches everything tests don't.

---

## Phase 0 — Bootstrap

Goal: Go module compiles, `cleo` binary prints "cleo v0.1.0-dev", `go test ./...` runs against zero packages without error.

### Task 0.1 — Initialize Go module + Cobra root

**Files:**
- Create: `go.mod`
- Create: `cmd/cleo/main.go`
- Create: `internal/cli/root.go`
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1: Initialize module**

```bash
cd /Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo
go mod init github.com/dhruvsaxena1998/cleo
go mod tidy
```

- [ ] **Step 2: Add cobra**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Create `internal/cli/root.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.0-dev"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cleo",
		Short:         "Terminal session manager for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TUI launch wired in Phase 9; for now a friendly stub.
			fmt.Println("cleo TUI — coming in phase 9")
			return nil
		},
	}
	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Create `cmd/cleo/main.go`**

```go
package main

import "github.com/dhruvsaxena1998/cleo/internal/cli"

func main() {
	cli.Execute()
}
```

- [ ] **Step 5: Create Makefile**

```make
.PHONY: build test lint run clean

build:
	go build -o bin/cleo ./cmd/cleo

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/cleo

clean:
	rm -rf bin/
```

- [ ] **Step 6: Create .gitignore**

```
bin/
*.test
*.out
.DS_Store
```

- [ ] **Step 7: Verify build and version output**

```bash
make build
./bin/cleo --version
```

Expected: `cleo version 0.1.0-dev`

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum cmd internal/cli Makefile .gitignore
git commit -m "chore: bootstrap go module and cobra root"
```

---

### Phase 0 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 1):**

- [ ] `make build` succeeds and produces `bin/cleo`
- [ ] `./bin/cleo --version` prints `cleo version 0.1.0-dev`
- [ ] `./bin/cleo --help` shows usage with `--help`, `--version`; no missing subcommands cause errors
- [ ] `./bin/cleo` (no args) prints the friendly stub and exits 0
- [ ] `make test` runs and reports zero packages (no test files yet — not an error)
- [ ] `make lint` (`go vet ./...`) reports nothing
- [ ] `git status` is clean — no untracked files generated by build

**Bugfix sprint:**

For every issue surfaced above, add a TDD task here:
- failing test or repro that demonstrates the bug
- fix
- test passes
- commit `fix(<pkg>): <bug summary>`

Do not advance to Phase 1 until this list is empty.

---

## Phase 1 — Foundations: paths, IDs, config

Goal: pure-Go libraries that the rest of the system depends on. All TDD.

### Task 1.1 — `internal/paths` package

Maps XDG paths so tests can inject a root.

**Files:**
- Create: `internal/paths/paths.go`
- Create: `internal/paths/paths_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/paths/paths_test.go
package paths

import (
	"path/filepath"
	"testing"
)

func TestNewWithRoot(t *testing.T) {
	p := NewWithRoot("/tmp/test")
	cases := map[string]string{
		"ConfigDir":   "/tmp/test",
		"ConfigFile":  "/tmp/test/config.toml",
		"ProjectsFile": "/tmp/test/projects.json",
		"StateFile":   "/tmp/test/state.json",
		"StateLock":   "/tmp/test/state.json.lock",
		"EventsDir":   "/tmp/test/events",
		"ArchiveDir":  "/tmp/test/events/archive",
		"SoundsDir":   "/tmp/test/sounds",
		"HookErrLog":  "/tmp/test/hook-errors.log",
	}
	got := map[string]string{
		"ConfigDir":    p.ConfigDir(),
		"ConfigFile":   p.ConfigFile(),
		"ProjectsFile": p.ProjectsFile(),
		"StateFile":    p.StateFile(),
		"StateLock":    p.StateLock(),
		"EventsDir":    p.EventsDir(),
		"ArchiveDir":   p.ArchiveDir(),
		"SoundsDir":    p.SoundsDir(),
		"HookErrLog":   p.HookErrLog(),
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("%s: got %q want %q", k, got[k], want)
		}
	}
	// EventsLog correlates to a session id
	if got := p.EventsLog("cleo-foo-bar"); got != filepath.Join("/tmp/test/events", "cleo-foo-bar.jsonl") {
		t.Errorf("EventsLog: %q", got)
	}
}
```

- [ ] **Step 2: Run, verify FAIL** (`paths` package not yet defined).

```bash
go test ./internal/paths/...
```

- [ ] **Step 3: Implement**

```go
// internal/paths/paths.go
package paths

import (
	"os"
	"path/filepath"
)

type Paths struct{ root string }

// New uses ~/.config/cleo (or $XDG_CONFIG_HOME/cleo if set).
func New() Paths {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return Paths{root: filepath.Join(x, "cleo")}
	}
	home, _ := os.UserHomeDir()
	return Paths{root: filepath.Join(home, ".config", "cleo")}
}

func NewWithRoot(root string) Paths { return Paths{root: root} }

func (p Paths) ConfigDir() string    { return p.root }
func (p Paths) ConfigFile() string   { return filepath.Join(p.root, "config.toml") }
func (p Paths) ProjectsFile() string { return filepath.Join(p.root, "projects.json") }
func (p Paths) StateFile() string    { return filepath.Join(p.root, "state.json") }
func (p Paths) StateLock() string    { return filepath.Join(p.root, "state.json.lock") }
func (p Paths) EventsDir() string    { return filepath.Join(p.root, "events") }
func (p Paths) ArchiveDir() string   { return filepath.Join(p.root, "events", "archive") }
func (p Paths) SoundsDir() string    { return filepath.Join(p.root, "sounds") }
func (p Paths) HookErrLog() string   { return filepath.Join(p.root, "hook-errors.log") }
func (p Paths) EventsLog(sid string) string {
	return filepath.Join(p.root, "events", sid+".jsonl")
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/paths/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/paths
git commit -m "feat(paths): add XDG paths helper with injectable root"
```

### Task 1.2 — `internal/ids` package

Slugification, dedup, session ID assembly, per-(project, agent) counter.

**Files:**
- Create: `internal/ids/slug.go`
- Create: `internal/ids/slug_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/ids/slug_test.go
package ids

import "testing"

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"fix auth bug":     "fix-auth-bug",
		"FIX-Auth_BUG":     "fix-auth-bug",
		"  multi   space ": "multi-space",
		"foo!@#bar":        "foo-bar",
		"":                 "",
		"--leading":        "leading",
		"trailing--":       "trailing",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupeSlug(t *testing.T) {
	existing := map[string]bool{"foo": true, "foo-2": true, "bar": true}
	for in, want := range map[string]string{
		"baz": "baz",
		"foo": "foo-3",
		"bar": "bar-2",
	} {
		if got := DedupeSlug(in, existing); got != want {
			t.Errorf("DedupeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMakeSessionID(t *testing.T) {
	got := MakeSessionID("myapp", "claude", "fix-auth-bug")
	if got != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

```bash
go test ./internal/ids/...
```

- [ ] **Step 3: Implement**

```go
// internal/ids/slug.go
package ids

import (
	"fmt"
	"strings"
	"unicode"
)

func Slugify(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// DedupeSlug returns slug if absent, else slug-2, slug-3, ...
func DedupeSlug(slug string, existing map[string]bool) string {
	if !existing[slug] {
		return slug
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", slug, i)
		if !existing[c] {
			return c
		}
	}
}

func MakeSessionID(project, agent, slug string) string {
	return fmt.Sprintf("cleo-%s-%s-%s", project, agent, slug)
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/ids/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/ids
git commit -m "feat(ids): slugify, dedupe, and session id assembly"
```

### Task 1.3 — `internal/config` types and TOML round-trip

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `go.mod` (adds BurntSushi/toml)

- [ ] **Step 1: Add dependency**

```bash
go get github.com/BurntSushi/toml@latest
```

- [ ] **Step 2: Write failing test**

```go
// internal/config/config_test.go
package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Defaults.DefaultAgent != "claude" {
		t.Errorf("default agent: %q", c.Defaults.DefaultAgent)
	}
	if !c.Sound.Enabled {
		t.Errorf("sound default disabled")
	}
	if c.Sound.Volume != 0.7 {
		t.Errorf("volume: %f", c.Sound.Volume)
	}
	if c.Agents["claude"].Label != "cl" {
		t.Errorf("claude label: %q", c.Agents["claude"].Label)
	}
	if c.Agents["claude"].Color != "#CC785C" {
		t.Errorf("claude color: %q", c.Agents["claude"].Color)
	}
	if c.UI.PanePreviewInterval != 1500*time.Millisecond {
		t.Errorf("interval: %v", c.UI.PanePreviewInterval)
	}
	if c.Retention.HintThreshold != 6 {
		t.Errorf("hint threshold: %d", c.Retention.HintThreshold)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Sound.Volume = 0.5
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Sound.Volume != 0.5 {
		t.Errorf("round trip lost volume: %f", c2.Sound.Volume)
	}
}
```

- [ ] **Step 3: Run, verify FAIL**

```bash
go test ./internal/config/...
```

- [ ] **Step 4: Implement struct + load/save**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Defaults  Defaults              `toml:"defaults"`
	Sound     Sound                 `toml:"sound"`
	Agents    map[string]Agent      `toml:"agents"`
	UI        UI                    `toml:"ui"`
	Retention Retention             `toml:"retention"`
}

type Defaults struct {
	DetachKey    string `toml:"detach_key"`
	DefaultAgent string `toml:"default_agent"`
}

type Sound struct {
	Enabled bool                  `toml:"enabled"`
	Volume  float64               `toml:"volume"`
	Events  map[string]string     `toml:"events"`
}

type Agent struct {
	Command string `toml:"command"`
	Label   string `toml:"label"`
	Color   string `toml:"color"`
	Hooks   string `toml:"hooks"` // "claude" | "codex" | "none"
}

type UI struct {
	ShowPanePreview     bool          `toml:"show_pane_preview"`
	PanePreviewLines    int           `toml:"pane_preview_lines"`
	PanePreviewInterval time.Duration `toml:"pane_preview_interval"`
	EventLogLines       int           `toml:"event_log_lines"`
	SidebarWidth        int           `toml:"sidebar_width"`
}

type Retention struct {
	HintThreshold           int           `toml:"hint_threshold"`
	PruneKeepDefault        int           `toml:"prune_keep_default"`
	IdleToCompletedTimeout  time.Duration `toml:"idle_to_completed_timeout"`
}

// Load reads from path; if not present, writes defaults and returns them.
func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c := Defaults_()
		if err := Save(path, c); err != nil {
			return Config{}, err
		}
		return c, nil
	}
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	mergeDefaults(&c)
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
```

- [ ] **Step 5: Implement defaults**

```go
// internal/config/defaults.go
package config

import "time"

func Defaults_() Config {
	return Config{
		Defaults: Defaults{DetachKey: "C-b d", DefaultAgent: "claude"},
		Sound: Sound{
			Enabled: true,
			Volume:  0.7,
			Events: map[string]string{
				"session_start":     "start.wav",
				"needs_input":       "attention.wav",
				"session_idle":      "done.wav",
				"session_completed": "done.wav",
				"session_error":     "error.wav",
			},
		},
		Agents: map[string]Agent{
			"claude":   {Command: "claude", Label: "cl", Color: "#CC785C", Hooks: "claude"},
			"codex":    {Command: "codex", Label: "cx", Color: "#10A37F", Hooks: "codex"},
			"opencode": {Command: "opencode", Label: "oc", Color: "#FF6B35", Hooks: "none"},
			"pi":       {Command: "pi", Label: "pi", Color: "#7C3AED", Hooks: "none"},
		},
		UI: UI{
			ShowPanePreview:     true,
			PanePreviewLines:    30,
			PanePreviewInterval: 1500 * time.Millisecond,
			EventLogLines:       200,
			SidebarWidth:        32,
		},
		Retention: Retention{
			HintThreshold:          6,
			PruneKeepDefault:       5,
			IdleToCompletedTimeout: 10 * time.Minute,
		},
	}
}

// mergeDefaults fills missing fields on a partially-specified config.
func mergeDefaults(c *Config) {
	d := Defaults_()
	if c.Defaults.DefaultAgent == "" {
		c.Defaults.DefaultAgent = d.Defaults.DefaultAgent
	}
	if c.Defaults.DetachKey == "" {
		c.Defaults.DetachKey = d.Defaults.DetachKey
	}
	if c.Sound.Volume == 0 {
		c.Sound.Volume = d.Sound.Volume
	}
	if c.Sound.Events == nil {
		c.Sound.Events = d.Sound.Events
	}
	if c.Agents == nil {
		c.Agents = d.Agents
	}
	if c.UI.SidebarWidth == 0 {
		c.UI = d.UI
	}
	if c.Retention.HintThreshold == 0 {
		c.Retention = d.Retention
	}
}
```

- [ ] **Step 6: Run, verify PASS**

```bash
go test ./internal/config/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/config go.mod go.sum
git commit -m "feat(config): TOML config with defaults, load and save"
```

### Phase 1 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 2):**

- [ ] `go test ./internal/paths/... ./internal/ids/... ./internal/config/...` all green
- [ ] Quick `go run` script: `config.Load("/tmp/cleo-cfg-test.toml")` → cat the file → keys are kebab/snake-cased per spec, well-indented, all five sound events present
- [ ] Embedded defaults include all four agents (claude, codex, opencode, pi) with correct labels (`cl`, `cx`, `oc`, `pi`) and brand colors
- [ ] `Slugify(" Hello — World!! ")` → `"hello-world"`; empty string → empty string; `"--leading"` → `"leading"`
- [ ] `DedupeSlug("foo", {"foo": true, "foo-2": true})` → `"foo-3"`
- [ ] `MakeSessionID("myapp","claude","fix-auth-bug")` → exactly `"cleo-myapp-claude-fix-auth-bug"`
- [ ] `paths.New()` resolves to `~/.config/cleo` on the dev box (or `$XDG_CONFIG_HOME/cleo` if set); confirm with `echo`

**Bugfix sprint:**

For every issue surfaced above, add a TDD task: failing test → fix → green → commit (`fix(<pkg>): <summary>`). Do not advance to Phase 2 until empty.

---

## Phase 2 — Projects

### Task 2.1 — Projects store

**Files:**
- Create: `internal/projects/store.go`
- Create: `internal/projects/projects_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/projects/projects_test.go
package projects

import (
	"path/filepath"
	"testing"
)

func TestAddAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)

	p, err := store.Add("/Users/x/Dev/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "myapp" {
		t.Errorf("id %q", p.ID)
	}

	got, err := store.Get("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/Users/x/Dev/myapp" {
		t.Errorf("path %q", got.Path)
	}
}

func TestAddDuplicateIDDeconflicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/myapp")
	p2, err := store.Add("/bar/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID != "myapp-2" {
		t.Errorf("expected myapp-2, got %q", p2.ID)
	}
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/myapp")
	if err := store.Remove("myapp"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("myapp"); err == nil {
		t.Errorf("expected ErrNotFound")
	}
}

func TestList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/a")
	_, _ = store.Add("/foo/b")
	got, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("len %d", len(got))
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

- [ ] **Step 3: Implement**

```go
// internal/projects/store.go
package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/ids"
)

var ErrNotFound = errors.New("project not found")

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	DefaultAgent string    `json:"default_agent,omitempty"`
	AddedAt      time.Time `json:"added_at"`
}

type fileFormat struct {
	Projects []Project `json:"projects"`
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Add(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	all, err := s.read()
	if err != nil {
		return Project{}, err
	}
	existing := map[string]bool{}
	for _, p := range all.Projects {
		existing[p.ID] = true
		if p.Path == abs {
			return Project{}, fmt.Errorf("path already registered: %s (id=%s)", abs, p.ID)
		}
	}
	id := ids.DedupeSlug(ids.Slugify(filepath.Base(abs)), existing)
	p := Project{
		ID:      id,
		Name:    filepath.Base(abs),
		Path:    abs,
		AddedAt: time.Now().UTC(),
	}
	all.Projects = append(all.Projects, p)
	return p, s.write(all)
}

func (s *Store) Remove(id string) error {
	all, err := s.read()
	if err != nil {
		return err
	}
	found := false
	out := all.Projects[:0]
	for _, p := range all.Projects {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return ErrNotFound
	}
	all.Projects = out
	return s.write(all)
}

func (s *Store) Get(id string) (Project, error) {
	all, err := s.read()
	if err != nil {
		return Project{}, err
	}
	for _, p := range all.Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}

func (s *Store) List() ([]Project, error) {
	all, err := s.read()
	return all.Projects, err
}

func (s *Store) read() (fileFormat, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return fileFormat{}, nil
	}
	if err != nil {
		return fileFormat{}, err
	}
	var f fileFormat
	return f, json.Unmarshal(b, &f)
}

func (s *Store) write(f fileFormat) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
```

- [ ] **Step 4: Run, verify PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/projects
git commit -m "feat(projects): json-backed store with add/remove/get/list"
```

### Task 2.2 — Walk-up resolver

Resolves a project from `pwd` by walking parent directories.

**Files:**
- Create: `internal/projects/resolver.go`
- Modify: `internal/projects/projects_test.go` (add resolver tests)

- [ ] **Step 1: Add failing test**

```go
// append to projects_test.go
func TestResolveFromCwd(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "projects.json"))
	root := filepath.Join(dir, "myapp")
	if err := os.MkdirAll(filepath.Join(root, "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(root); err != nil {
		t.Fatal(err)
	}
	p, err := store.ResolveFromCwd(filepath.Join(root, "src", "deep"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "myapp" {
		t.Errorf("got %q", p.ID)
	}
	if _, err := store.ResolveFromCwd(t.TempDir()); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/projects/resolver.go
package projects

import (
	"path/filepath"
	"strings"
)

func (s *Store) ResolveFromCwd(cwd string) (Project, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Project{}, err
	}
	all, err := s.List()
	if err != nil {
		return Project{}, err
	}
	// Match longest prefix wins (in case of nested registered paths).
	var best Project
	bestLen := -1
	for _, p := range all {
		if abs == p.Path || strings.HasPrefix(abs, p.Path+string(filepath.Separator)) {
			if len(p.Path) > bestLen {
				bestLen = len(p.Path)
				best = p
			}
		}
	}
	if bestLen < 0 {
		return Project{}, ErrNotFound
	}
	return best, nil
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/projects
git commit -m "feat(projects): walk-up resolver from cwd"
```

### Phase 2 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 3):**

- [ ] `go test ./internal/projects/...` green
- [ ] Manual: `go run` a small program that does `Add("/tmp/foo")`, `Add("/tmp/bar")`, `Remove("foo")`, `List()`. Inspect projects.json — JSON is well-indented, fields match spec §5.1
- [ ] `Add` of an already-registered absolute path returns an error (not a silent dupe)
- [ ] `Add` of two paths with the same basename → second gets `-2` suffix in ID
- [ ] `ResolveFromCwd("/tmp/foo/src/deep")` after registering `/tmp/foo` returns the project
- [ ] `ResolveFromCwd("/some/random/path")` returns `ErrNotFound`
- [ ] Nested registrations: register `/a` and `/a/b`; resolve from `/a/b/c` returns `/a/b` (longest prefix wins)

**Bugfix sprint:**

For every issue surfaced above: failing test → fix → green → commit. Do not advance to Phase 3 until empty.

---

## Phase 3 — State + Events

### Task 3.1 — Session struct, State enum, transitions

**Files:**
- Create: `internal/state/session.go`
- Create: `internal/state/transitions.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing test (transitions only — store comes next)**

```go
// internal/state/state_test.go
package state

import "testing"

func TestNextState(t *testing.T) {
	cases := []struct {
		from State
		ev   Event
		want State
	}{
		{Spawning, EvSessionStart, Running},
		{Spawning, EvPreToolUse, Running},
		{Running, EvPreToolUse, Running}, // no-op
		{Running, EvPostToolUse, Running},
		{Running, EvNotification, WaitingForInput},
		{WaitingForInput, EvUserResume, Running},
		{Running, EvStop, Idle},
		{Idle, EvSessionEnd, Completed},
		{Idle, EvIdleTimeout, Completed},
		{Running, EvSessionEnd, Completed},
		{Idle, EvError, Errored},
	}
	for _, c := range cases {
		got := NextState(c.from, c.ev)
		if got != c.want {
			t.Errorf("NextState(%s, %s) = %s, want %s", c.from, c.ev, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Implement enums and Session struct**

```go
// internal/state/session.go
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
	EvUserResume   Event = "user_resume"   // synthesized: input followed by activity
	EvIdleTimeout  Event = "idle_timeout"  // synthesized: reconciler
	EvError        Event = "error"
	EvDead         Event = "dead"          // synthesized: tmux session gone
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
}
```

- [ ] **Step 3: Implement transitions**

```go
// internal/state/transitions.go
package state

func NextState(from State, ev Event) State {
	switch ev {
	case EvDead:
		return Dead
	case EvError:
		return Errored
	case EvSessionEnd:
		return Completed
	case EvIdleTimeout:
		if from == Idle {
			return Completed
		}
		return from
	case EvSessionStart:
		return Running
	case EvNotification:
		return WaitingForInput
	case EvStop:
		return Idle
	case EvUserResume:
		return Running
	case EvPreToolUse, EvPostToolUse:
		if from == Spawning {
			return Running
		}
		if from == WaitingForInput {
			return Running // implicit resume
		}
		if from == Idle {
			return Running
		}
		return from
	}
	return from
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/state/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/state
git commit -m "feat(state): session struct, state enum, and transition rules"
```

### Task 3.2 — State store with flock + atomic rename

**Files:**
- Create: `internal/state/store.go`
- Modify: `internal/state/state_test.go`
- Modify: `go.mod` (add gofrs/flock)

- [ ] **Step 1: Add dep**

```bash
go get github.com/gofrs/flock@latest
```

- [ ] **Step 2: Write failing tests**

```go
// append to state_test.go
import (
	"path/filepath"
	"sync"
	"time"
)

func TestStorePutGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))

	s := Session{
		ID: "cleo-foo-claude-1", ProjectID: "foo", Agent: "claude",
		Name: "1", State: Spawning, StartedAt: time.Now().UTC(),
	}
	if err := store.Put(s); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Spawning {
		t.Errorf("state %s", got.State)
	}
}

func TestStoreApplyEvent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	s := Session{ID: "x", State: Spawning, Agent: "claude"}
	_ = store.Put(s)

	got, err := store.Apply("x", EvSessionStart, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Running {
		t.Errorf("state %s", got.State)
	}
	if got.ToolCount != 0 {
		t.Errorf("tool count %d", got.ToolCount)
	}

	got, _ = store.Apply("x", EvPostToolUse, "")
	if got.ToolCount != 1 {
		t.Errorf("expected tool_count 1, got %d", got.ToolCount)
	}
}

func TestStoreConcurrentApply(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	_ = store.Put(Session{ID: "x", State: Running, Agent: "claude"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Apply("x", EvPostToolUse, "")
		}()
	}
	wg.Wait()
	got, _ := store.Get("x")
	if got.ToolCount != 50 {
		t.Errorf("expected tool_count 50 (no lost updates), got %d", got.ToolCount)
	}
}
```

- [ ] **Step 3: Implement store**

```go
// internal/state/store.go
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

var ErrSessionNotFound = errors.New("session not found")

type fileFormat struct {
	Version  int                `json:"version"`
	Sessions map[string]Session `json:"sessions"`
}

type Store struct {
	path     string
	lockPath string
}

func NewStore(path, lockPath string) *Store {
	return &Store{path: path, lockPath: lockPath}
}

func (s *Store) Put(sess Session) error {
	return s.modify(func(f *fileFormat) error {
		if f.Sessions == nil {
			f.Sessions = map[string]Session{}
		}
		f.Sessions[sess.ID] = sess
		return nil
	})
}

func (s *Store) Get(id string) (Session, error) {
	f, err := s.read()
	if err != nil {
		return Session{}, err
	}
	sess, ok := f.Sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *Store) List() ([]Session, error) {
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(f.Sessions))
	for _, v := range f.Sessions {
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) Delete(id string) error {
	return s.modify(func(f *fileFormat) error {
		delete(f.Sessions, id)
		return nil
	})
}

// Apply transitions a session by event under the lock and returns the updated session.
// `lastMessage` is set on the session if non-empty (used for Notification text).
func (s *Store) Apply(id string, ev Event, lastMessage string) (Session, error) {
	var out Session
	err := s.modify(func(f *fileFormat) error {
		sess, ok := f.Sessions[id]
		if !ok {
			return ErrSessionNotFound
		}
		sess.State = NextState(sess.State, ev)
		sess.LastEventAt = time.Now().UTC()
		if lastMessage != "" {
			sess.LastMessage = lastMessage
		}
		if ev == EvPostToolUse {
			sess.ToolCount++
		}
		f.Sessions[id] = sess
		out = sess
		return nil
	})
	return out, err
}

func (s *Store) modify(fn func(*fileFormat) error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	lk := flock.New(s.lockPath)
	if err := lk.Lock(); err != nil {
		return err
	}
	defer lk.Unlock()

	f, err := s.readUnlocked()
	if err != nil {
		return err
	}
	if err := fn(&f); err != nil {
		return err
	}
	return s.writeUnlocked(f)
}

func (s *Store) read() (fileFormat, error) {
	lk := flock.New(s.lockPath)
	if err := lk.RLock(); err != nil {
		return fileFormat{}, err
	}
	defer lk.Unlock()
	return s.readUnlocked()
}

func (s *Store) readUnlocked() (fileFormat, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return fileFormat{Version: 1, Sessions: map[string]Session{}}, nil
	}
	if err != nil {
		return fileFormat{}, err
	}
	var f fileFormat
	if err := json.Unmarshal(b, &f); err != nil {
		return fileFormat{}, err
	}
	if f.Sessions == nil {
		f.Sessions = map[string]Session{}
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

func (s *Store) writeUnlocked(f fileFormat) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/state/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/state go.mod go.sum
git commit -m "feat(state): json store with flock and atomic apply"
```

### Task 3.3 — Per-session events log

**Files:**
- Create: `internal/events/log.go`
- Create: `internal/events/events_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/events/events_test.go
package events

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndTail(t *testing.T) {
	dir := t.TempDir()
	log := NewLog(filepath.Join(dir, "x.jsonl"))
	for i := 0; i < 3; i++ {
		if err := log.Append(Entry{
			At:    time.Now(),
			Type:  "PreToolUse",
			Tool:  "Bash",
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := log.Tail(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("got %d", len(got))
	}
}

func TestTailLimitsToN(t *testing.T) {
	dir := t.TempDir()
	log := NewLog(filepath.Join(dir, "x.jsonl"))
	for i := 0; i < 10; i++ {
		_ = log.Append(Entry{Type: "x"})
	}
	got, _ := log.Tail(3)
	if len(got) != 3 {
		t.Errorf("got %d", len(got))
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/events/log.go
package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	At        time.Time      `json:"at"`
	Type      string         `json:"type"`
	Tool      string         `json:"tool,omitempty"`
	Detail    string         `json:"detail,omitempty"`
	DurationS float64        `json:"duration_s,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type Log struct{ path string }

func NewLog(path string) *Log { return &Log{path: path} }

func (l *Log) Append(e Entry) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func (l *Log) Tail(n int) ([]Entry, error) {
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read all, return last n. v0.1 events are small (<1MB per session); this is fine.
	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err == nil {
			all = append(all, e)
		}
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, sc.Err()
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/events
git commit -m "feat(events): per-session jsonl append + tail"
```

### Task 3.4 — Archive (gzip + move)

**Files:**
- Create: `internal/events/archive.go`
- Modify: `internal/events/events_test.go`

- [ ] **Step 1: Add failing test**

```go
func TestArchive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.jsonl")
	log := NewLog(src)
	_ = log.Append(Entry{Type: "x"})
	archDir := filepath.Join(dir, "archive")
	if err := Archive(src, archDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src still exists")
	}
	matches, _ := filepath.Glob(filepath.Join(archDir, "x.jsonl.gz"))
	if len(matches) != 1 {
		t.Errorf("archive missing")
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/events/archive.go
package events

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

// Archive gzips srcPath and moves it into archiveDir; deletes the src on success.
func Archive(srcPath, archiveDir string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(archiveDir, filepath.Base(srcPath)+".gz")
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.Remove(srcPath)
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/events
git commit -m "feat(events): gzip archive on prune"
```

### Phase 3 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 4):**

- [ ] `go test ./internal/state/... ./internal/events/...` green
- [ ] Concurrent-apply test passes (validates flock — re-run with `-count=10` to be sure)
- [ ] Manual: spawn 4 background shells from a tiny `go run` driver that each call `Apply` 100 times against the same store; final tool count must equal 400 (no lost updates)
- [ ] Manual: `kill -9` a process mid-write; inspect state.json — must be either old or new content, never a half-written file (atomic rename works)
- [ ] Manual: open state.json in a viewer after some Apply calls — JSON is well-indented, all expected fields present, ISO timestamps
- [ ] State machine spot checks: `running + Notification → waiting_for_input`; `waiting_for_input + PreToolUse → running` (implicit resume); `idle + IdleTimeout → completed`; `* + Dead → dead`
- [ ] Events log: append 1000 entries, `Tail(50)` returns the last 50 in chronological order
- [ ] `events.Archive` produces a valid gzip; `gunzip -c` matches the original line count

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 4 until empty.

---

## Phase 4 — tmux wrapper

Goal: a thin Go wrapper around `tmux` CLI that's the *only* place we shell out to tmux.

### Task 4.1 — `tmux` package: NewSession, HasSession, Kill, Ls

**Files:**
- Create: `internal/tmux/tmux.go`
- Create: `internal/tmux/tmux_test.go`

These tests assume `tmux` is installed. Use a private socket `cleo-test-<random>` to avoid colliding with the user's tmux.

- [ ] **Step 1: Write failing tests**

```go
// internal/tmux/tmux_test.go
package tmux

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	if !Available() {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("cleo-test-%d", rand.Int63())
	c := NewClient(socket)
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

func TestNewSessionAndHas(t *testing.T) {
	c := newTestClient(t)
	if err := c.NewSession(NewSessionOpts{Name: "cleo-foo-claude-1", Cwd: "/tmp", Cmd: "sleep 60", Env: nil}); err != nil {
		t.Fatal(err)
	}
	ok, err := c.HasSession("cleo-foo-claude-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected has-session true")
	}
}

func TestLsWithPrefix(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-a-claude-1", Cwd: "/tmp", Cmd: "sleep 60"})
	_ = c.NewSession(NewSessionOpts{Name: "other", Cwd: "/tmp", Cmd: "sleep 60"})
	got, err := c.LsPrefix("cleo-")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "cleo-") {
		t.Errorf("got %v", got)
	}
}

func TestKill(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-x-claude-1", Cwd: "/tmp", Cmd: "sleep 60"})
	if err := c.Kill("cleo-x-claude-1"); err != nil {
		t.Fatal(err)
	}
	ok, _ := c.HasSession("cleo-x-claude-1")
	if ok {
		t.Errorf("expected gone")
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

- [ ] **Step 3: Implement**

```go
// internal/tmux/tmux.go
package tmux

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct{ socket string }

// NewClient with a custom socket name; pass "" for default tmux.
func NewClient(socket string) *Client { return &Client{socket: socket} }

func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

type NewSessionOpts struct {
	Name string
	Cwd  string
	Cmd  string
	Env  map[string]string
}

func (c *Client) cmd(args ...string) *exec.Cmd {
	full := []string{}
	if c.socket != "" {
		full = append(full, "-L", c.socket)
	}
	full = append(full, args...)
	return exec.Command("tmux", full...)
}

func (c *Client) NewSession(o NewSessionOpts) error {
	if o.Name == "" {
		return errors.New("tmux: empty session name")
	}
	args := []string{"new-session", "-d", "-s", o.Name}
	if o.Cwd != "" {
		args = append(args, "-c", o.Cwd)
	}
	for k, v := range o.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if o.Cmd != "" {
		args = append(args, o.Cmd)
	}
	out, err := c.cmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) HasSession(name string) (bool, error) {
	err := c.cmd("has-session", "-t", name).Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func (c *Client) LsPrefix(prefix string) ([]string, error) {
	out, err := c.cmd("ls", "-F", "#{session_name}").Output()
	if err != nil {
		// "no server running" means zero sessions; treat as empty list.
		if ee, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(ee.Stderr), "no server") {
				return nil, nil
			}
		}
		return nil, err
	}
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

func (c *Client) Kill(name string) error {
	return c.cmd("kill-session", "-t", name).Run()
}

func (c *Client) KillServer() error {
	return c.cmd("kill-server").Run()
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/tmux/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tmux
git commit -m "feat(tmux): client wrapper with NewSession, Has, Ls, Kill"
```

### Task 4.2 — CapturePane and RenameSession

**Files:**
- Modify: `internal/tmux/tmux.go`
- Modify: `internal/tmux/tmux_test.go`

- [ ] **Step 1: Add failing tests**

```go
func TestCapturePane(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-cap-1", Cwd: "/tmp", Cmd: "echo HELLO_WORLD; sleep 60"})
	// give shell a moment
	time.Sleep(150 * time.Millisecond)
	out, err := c.CapturePane("cleo-cap-1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "HELLO_WORLD") {
		t.Errorf("missing token in capture: %q", out)
	}
}

func TestRenameSession(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "old", Cwd: "/tmp", Cmd: "sleep 60"})
	if err := c.RenameSession("old", "new"); err != nil {
		t.Fatal(err)
	}
	ok, _ := c.HasSession("new")
	if !ok {
		t.Errorf("expected new")
	}
}
```

(add `import "time"` if needed)

- [ ] **Step 2: Implement**

```go
// add to tmux.go
func (c *Client) CapturePane(name string, lines int) (string, error) {
	args := []string{"capture-pane", "-p", "-t", name + ":."}
	if lines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", lines))
	}
	out, err := c.cmd(args...).Output()
	return string(out), err
}

func (c *Client) RenameSession(from, to string) error {
	return c.cmd("rename-session", "-t", from, to).Run()
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/tmux
git commit -m "feat(tmux): capture-pane and rename-session"
```

### Phase 4 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 5):**

- [ ] `go test ./internal/tmux/...` green against a real tmux server
- [ ] Tests use a private `-L cleo-test-<random>` socket and clean up — verify `tmux -L cleo-test-* ls` returns nothing after `go test` exits
- [ ] Manual: `tmux.NewClient("").NewSession({Name: "cleo-x-1", Cwd: "/tmp", Cmd: "sleep 60"})` from a `go run` driver; in another shell `tmux ls` — session present
- [ ] `CapturePane` on a session running `seq 1 100` returns content that includes the high numbers (proving `-S -<lines>` works)
- [ ] `RenameSession` works; `tmux ls` reflects the new name
- [ ] `LsPrefix("cleo-")` filters correctly when both cleo-prefixed and non-prefixed sessions exist
- [ ] `HasSession` on a missing name returns `(false, nil)` — not an error
- [ ] `LsPrefix` when no tmux server is running returns `(nil, nil)` — not an error
- [ ] Edge: `NewSession` with a name that already exists returns a real error
- [ ] Env propagation: `NewSession` with `Env: {"FOO":"bar"}`, then attach and run `echo $FOO` — sees `bar`

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 5 until empty.

---

## Phase 5 — Sound

Sound is small and standalone; ship it before CLI/hooks so they can wire it in.

### Task 5.1 — Embed default WAVs

**Files:**
- Create: `assets/sounds/start.wav` (placeholder bytes)
- Create: `assets/sounds/attention.wav`
- Create: `assets/sounds/done.wav`
- Create: `assets/sounds/error.wav`
- Create: `internal/sound/assets.go`
- Create: `internal/sound/sound_test.go`

- [ ] **Step 1: Create placeholder WAVs**

The implementer should source four short royalty-free WAVs (~200 ms each). Until then, generate stubs:

```bash
for f in start attention done error; do
  printf 'RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x40\x1f\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00' > assets/sounds/$f.wav
done
```

(The placeholder is a valid empty WAV header. Replace with real assets before shipping; tests don't play audio.)

- [ ] **Step 2: Write failing test**

```go
// internal/sound/sound_test.go
package sound

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAssetsCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"start.wav", "attention.wav", "done.wav", "error.wav"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestExtractAssetsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
	// second call must not error or overwrite if file exists
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Implement**

```go
// internal/sound/assets.go
package sound

import (
	"embed"
	"io"
	"os"
	"path/filepath"
)

//go:embed all:assets
var assetsFS embed.FS

// ExtractDefaults copies bundled WAVs to dir if not already present.
func ExtractDefaults(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue // idempotent
		}
		src, err := assetsFS.Open("assets/" + e.Name())
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			src.Close()
			out.Close()
			return err
		}
		src.Close()
		out.Close()
	}
	return nil
}
```

The `//go:embed` directive needs the assets to live inside the package. Move them: `internal/sound/assets/start.wav` etc. Update Step 1's paths accordingly:

```bash
mkdir -p internal/sound/assets
for f in start attention done error; do
  printf 'RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x40\x1f\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00' > internal/sound/assets/$f.wav
done
```

- [ ] **Step 4: Run, verify PASS**

```bash
go test ./internal/sound/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/sound
git commit -m "feat(sound): embed default wavs and extractor"
```

### Task 5.2 — Player probe + fire-and-forget Play

**Files:**
- Create: `internal/sound/player.go`
- Modify: `internal/sound/sound_test.go`

- [ ] **Step 1: Add failing test**

```go
func TestProbePlayerNoneFound(t *testing.T) {
	// override $PATH so no player is found
	t.Setenv("PATH", "")
	p := NewPlayer(0.7)
	if p.Available() {
		t.Errorf("expected unavailable")
	}
	// Play() must not return error even when nothing is available
	if err := p.Play("/nope.wav"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/sound/player.go
package sound

import (
	"os/exec"
	"runtime"
)

type Player struct {
	bin    string
	args   func(file string) []string
	volume float64
}

func NewPlayer(volume float64) *Player {
	p := &Player{volume: volume}
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("afplay"); err == nil {
			p.bin = path
			vol := volume
			p.args = func(file string) []string {
				return []string{"-v", floatStr(vol), file}
			}
			return p
		}
	}
	for _, name := range []string{"paplay", "aplay", "play"} {
		if path, err := exec.LookPath(name); err == nil {
			p.bin = path
			p.args = func(file string) []string { return []string{file} }
			return p
		}
	}
	return p
}

func (p *Player) Available() bool { return p.bin != "" }

// Play is fire-and-forget: returns as soon as the child process is started.
// Errors are intentionally swallowed; sound failures must not block the agent.
func (p *Player) Play(file string) error {
	if !p.Available() {
		return nil
	}
	cmd := exec.Command(p.bin, p.args(file)...)
	return cmd.Start() // intentionally not Wait()
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
```

(Add `"strconv"` to imports.)

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/sound
git commit -m "feat(sound): cross-platform player with fire-and-forget play"
```

### Phase 5 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 6):**

- [ ] `go test ./internal/sound/...` green
- [ ] Manual: `ExtractDefaults("/tmp/cleo-snd")` writes 4 wavs; running again does NOT overwrite (touch each file's mtime, re-extract, verify mtimes unchanged)
- [ ] Manual on macOS: `Player.Play("/tmp/cleo-snd/start.wav")` — a sound is audible (replace placeholder WAVs with real audio first; placeholder is silent)
- [ ] Player binary probe: `which afplay` on macOS / `which paplay` on Linux — confirm `Player.Available()` agrees
- [ ] Manual: temporarily `PATH=""` and call `Player.Play(...)` — returns nil error, doesn't crash
- [ ] Linux check (if available): `paplay` plays the wav; if `paplay` absent, `aplay` is tried; if both absent, `play` (sox) is tried
- [ ] Fire-and-forget: call `Play` and time the call — must return in <50ms even when the player binary is slow

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 6 until empty. Real WAV assets MUST be sourced before Phase 7 (hooks rely on them).

---

## Phase 6 — CLI commands

Each CLI command gets its own task. They share a common pattern:

1. Cobra command definition.
2. Wire into root in `cli.NewRootCmd()`.
3. Test for the happy path using `t.TempDir()` for isolated paths.
4. Test for the most important error case.

We introduce a small `cli/context.go` that constructs the wired-up dependencies (paths, config, stores) so each command can be tested with injected paths.

### Task 6.1 — CLI context wiring

**Files:**
- Create: `internal/cli/context.go`
- Create: `internal/cli/context_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/cli/context_test.go
package cli

import "testing"

func TestNewCtxWithRoot(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCtxWithRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Paths.ConfigDir() != dir {
		t.Errorf("config dir mismatch")
	}
	if c.Config.Defaults.DefaultAgent != "claude" {
		t.Errorf("config not loaded")
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/cli/context.go
package cli

import (
	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

type Ctx struct {
	Paths    paths.Paths
	Config   config.Config
	Projects *projects.Store
	State    *state.Store
	Tmux     *tmux.Client
	Player   *sound.Player
	Events   func(sid string) *events.Log
}

func NewCtx() (*Ctx, error) { return NewCtxWithRoot(paths.New().ConfigDir()) }

func NewCtxWithRoot(root string) (*Ctx, error) {
	p := paths.NewWithRoot(root)
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return nil, err
	}
	return &Ctx{
		Paths:    p,
		Config:   cfg,
		Projects: projects.NewStore(p.ProjectsFile()),
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     tmux.NewClient(""),
		Player:   sound.NewPlayer(cfg.Sound.Volume),
		Events:   func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
	}, nil
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): shared command context with injected paths"
```

### Task 6.2 — `cleo add`

**Files:**
- Create: `internal/cli/add.go`
- Create: `internal/cli/add_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing test**

```go
// internal/cli/add_test.go
package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestAddRegistersProject(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	cmd := newAddCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{target})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c, _ := NewCtxWithRoot(root)
	got, err := c.Projects.Get("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != target {
		t.Errorf("path %q", got.Path)
	}
}
```

(Helpers `mkdir`, `testRootedCtx` go in `internal/cli/cli_test_helpers.go`. Define them once.)

- [ ] **Step 2: Add helpers (one-time)**

```go
// internal/cli/cli_test_helpers.go
package cli

import (
	"os"
	"testing"
)

func testRootedCtx(t *testing.T, root string) func() *Ctx {
	t.Helper()
	return func() *Ctx {
		c, err := NewCtxWithRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
}

func mkdir(p string) error { return os.MkdirAll(p, 0o755) }
```

- [ ] **Step 3: Implement command**

```go
// internal/cli/add.go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAddCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "add [path]",
		Short: "Register a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			abs, err := os.Getwd()
			if err == nil && path == "." {
				path = abs
			}
			c := getCtx()
			p, err := c.Projects.Add(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "registered project %q at %s\n", p.ID, p.Path)
			return nil
		},
	}
}
```

- [ ] **Step 4: Wire into root**

In `internal/cli/root.go`, change the root constructor to attach subcommands:

```go
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cleo",
		Short:         "Terminal session manager for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cleo TUI — coming in phase 9")
			return nil
		},
	}
	getCtx := func() *Ctx {
		c, err := NewCtx()
		if err != nil {
			panic(err)
		}
		return c
	}
	root.AddCommand(
		newAddCmd(getCtx),
	)
	return root
}
```

- [ ] **Step 5: Run, verify PASS**

```bash
go test ./internal/cli/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): cleo add registers project"
```

### Task 6.3 — `cleo rm`

**Files:**
- Create: `internal/cli/rm.go`
- Create: `internal/cli/rm_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write test**

```go
func TestRmRemovesProject(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Get("myapp"); err == nil {
		t.Errorf("expected gone")
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/cli/rm.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRmCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project>",
		Short: "Unregister a project (running sessions keep running)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if err := c.Projects.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed project %q\n", args[0])
			return nil
		},
	}
}
```

- [ ] **Step 3: Wire into root, run tests, commit**

```bash
git add internal/cli
git commit -m "feat(cli): cleo rm unregisters project"
```

### Task 6.4 — `cleo ls`

**Files:**
- Create: `internal/cli/ls.go`
- Create: `internal/cli/ls_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write test (output assertion)**

```go
func TestLsShowsProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude", Name: "1", State: state.Running})

	cmd := newLsCmd(testRootedCtx(t, root))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "myapp") || !strings.Contains(out.String(), "running") {
		t.Errorf("output: %q", out.String())
	}
}
```

(Add `"github.com/dhruvsaxena1998/cleo/internal/state"` and `"strings"` imports.)

- [ ] **Step 2: Implement**

```go
// internal/cli/ls.go
package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newLsCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List projects and sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			projects, _ := c.Projects.List()
			sessions, _ := c.State.List()

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tAGENT\tNAME\tSTATE\tID")
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
			byProj := map[string][]int{}
			for i, s := range sessions {
				byProj[s.ProjectID] = append(byProj[s.ProjectID], i)
			}
			for _, p := range projects {
				if len(byProj[p.ID]) == 0 {
					fmt.Fprintf(tw, "%s\t-\t-\t-\t-\n", p.ID)
					continue
				}
				for _, i := range byProj[p.ID] {
					s := sessions[i]
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, s.Agent, s.Name, s.State, s.ID)
				}
			}
			return tw.Flush()
		},
	}
}
```

- [ ] **Step 3: Wire, test, commit**

```bash
git add internal/cli
git commit -m "feat(cli): cleo ls lists projects and sessions"
```

### Task 6.5 — `cleo run` (spawn agent in tmux)

**Files:**
- Create: `internal/cli/run.go`
- Create: `internal/cli/run_test.go`
- Modify: `internal/cli/root.go`

This is the most complex command. It:
1. Resolves the project from cwd (or auto-registers with confirmation).
2. Validates the agent exists in config.
3. Slugifies/dedupes the name.
4. Builds session ID.
5. Adds session to state.json (state=Spawning).
6. Spawns tmux with `CLEO_SESSION_ID` env.
7. Prints confirmation.

- [ ] **Step 1: Write failing test (use a stub tmux client)**

To avoid coupling the test to a real tmux, wrap the client in an interface:

```go
// internal/cli/run_test.go
package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRunSpawnsAndRecordsSession(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	_, _ = c.Projects.Add(target)

	// Use a fake tmux that records calls instead of running the binary.
	fake := &fakeTmux{}
	c.Tmux = fake
	getCtx := func() *Ctx { return c }

	cmd := newRunCmd(getCtx)
	cmd.SetArgs([]string{"claude", "--name", "fix-auth-bug", "--cwd", target, "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.created) != 1 {
		t.Fatalf("expected one session created, got %d", len(fake.created))
	}
	if fake.created[0].Name != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("session name: %q", fake.created[0].Name)
	}
	if fake.created[0].Env["CLEO_SESSION_ID"] != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("env not set")
	}
	got, err := c.State.Get("cleo-myapp-claude-fix-auth-bug")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != state.Spawning {
		t.Errorf("state: %s", got.State)
	}
}
```

- [ ] **Step 2: Define tmux interface in cli package**

```go
// internal/cli/tmux_iface.go
package cli

import "github.com/dhruvsaxena1998/cleo/internal/tmux"

type TmuxClient interface {
	NewSession(o tmux.NewSessionOpts) error
	HasSession(name string) (bool, error)
	LsPrefix(prefix string) ([]string, error)
	Kill(name string) error
	CapturePane(name string, lines int) (string, error)
	RenameSession(from, to string) error
}
```

Update `Ctx.Tmux` to be `TmuxClient`. `tmux.Client` already satisfies this. Add `fakeTmux` to the test helpers:

```go
// add to cli_test_helpers.go
type fakeTmux struct {
	created []tmux.NewSessionOpts
	exists  map[string]bool
}

func (f *fakeTmux) NewSession(o tmux.NewSessionOpts) error {
	f.created = append(f.created, o)
	if f.exists == nil {
		f.exists = map[string]bool{}
	}
	f.exists[o.Name] = true
	return nil
}
func (f *fakeTmux) HasSession(n string) (bool, error)      { return f.exists[n], nil }
func (f *fakeTmux) LsPrefix(p string) ([]string, error)    { /* enumerate */ return nil, nil }
func (f *fakeTmux) Kill(n string) error                    { delete(f.exists, n); return nil }
func (f *fakeTmux) CapturePane(string, int) (string, error){ return "", nil }
func (f *fakeTmux) RenameSession(from, to string) error    { return nil }
```

- [ ] **Step 3: Implement run**

```go
// internal/cli/run.go
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func newRunCmd(getCtx func() *Ctx) *cobra.Command {
	var name string
	var cwdFlag string
	var yes bool

	cmd := &cobra.Command{
		Use:   "run <agent>",
		Short: "Spawn an agent in the current project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			agentName := args[0]
			agent, ok := c.Config.Agents[agentName]
			if !ok {
				return fmt.Errorf("unknown agent %q (configured: %v)", agentName, agentKeys(c.Config.Agents))
			}

			cwd := cwdFlag
			if cwd == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				cwd = wd
			}
			cwd, _ = filepath.Abs(cwd)

			proj, err := c.Projects.ResolveFromCwd(cwd)
			if errors.Is(err, projects.ErrNotFound) {
				if !yes {
					fmt.Fprintf(cmd.OutOrStdout(), "register %q as a new project? [Y/n] ", cwd)
					ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
					ans = strings.TrimSpace(strings.ToLower(ans))
					if ans != "" && ans != "y" && ans != "yes" {
						return errors.New("aborted")
					}
				}
				proj, err = c.Projects.Add(cwd)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "registered project %q\n", proj.ID)
			} else if err != nil {
				return err
			}

			// Compute slug: user name (slugified) or counter
			existing := existingSlugs(c, proj.ID, agentName)
			var slug string
			if name != "" {
				slug = ids.DedupeSlug(ids.Slugify(name), existing)
			} else {
				slug = nextCounterSlug(existing)
			}
			sid := ids.MakeSessionID(proj.ID, agentName, slug)

			sess := state.Session{
				ID:        sid,
				ProjectID: proj.ID,
				Agent:     agentName,
				Name:      slug,
				State:     state.Spawning,
				StartedAt: time.Now().UTC(),
			}
			if err := c.State.Put(sess); err != nil {
				return err
			}
			err = c.Tmux.NewSession(tmux.NewSessionOpts{
				Name: sid,
				Cwd:  proj.Path,
				Cmd:  agent.Command,
				Env:  map[string]string{"CLEO_SESSION_ID": sid},
			})
			if err != nil {
				_ = c.State.Delete(sid)
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spawned %s — attach with `cleo attach %s`\n", sid, sid)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "session name (slugified)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "override working directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip auto-register confirmation")
	return cmd
}

func agentKeys(m map[string]config.Agent) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func existingSlugs(c *Ctx, project, agent string) map[string]bool {
	out := map[string]bool{}
	all, _ := c.State.List()
	prefix := fmt.Sprintf("cleo-%s-%s-", project, agent)
	for _, s := range all {
		if strings.HasPrefix(s.ID, prefix) {
			out[s.Name] = true
		}
	}
	return out
}

func nextCounterSlug(existing map[string]bool) string {
	for i := 1; ; i++ {
		c := fmt.Sprintf("%d", i)
		if !existing[c] {
			return c
		}
	}
}
```

(Imports `config`. Add `"github.com/dhruvsaxena1998/cleo/internal/config"`.)

- [ ] **Step 4: Wire, test, commit**

```bash
go test ./internal/cli/...
git add internal/cli
git commit -m "feat(cli): cleo run spawns agent in tmux with CLEO_SESSION_ID"
```

### Task 6.6 — `cleo attach`, `cleo kill`

**Files:**
- Create: `internal/cli/attach.go`
- Create: `internal/cli/kill.go`
- Tests: extend `kill_test.go` (skip a working test for attach since it execs into tmux)
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Implement attach (no test — execs)**

```go
// internal/cli/attach.go
package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newAttachCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := exec.Command("tmux", "attach", "-t", args[0])
			t.Stdin = os.Stdin
			t.Stdout = os.Stdout
			t.Stderr = os.Stderr
			return t.Run()
		},
	}
}
```

- [ ] **Step 2: Test + implement kill**

```go
// internal/cli/kill_test.go
func TestKillRemovesSessionFromState(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-foo-claude-1": true}}
	_ = c.State.Put(state.Session{ID: "cleo-foo-claude-1", State: state.Running})

	cmd := newKillCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"cleo-foo-claude-1", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.State.Get("cleo-foo-claude-1"); err != state.ErrSessionNotFound {
		t.Errorf("expected gone")
	}
}
```

```go
// internal/cli/kill.go
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newKillCmd(getCtx func() *Ctx) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "kill %q? [y/N] ", id)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			c := getCtx()
			_ = c.Tmux.Kill(id)
			return c.State.Delete(id)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
```

- [ ] **Step 3: Wire, test, commit**

```bash
git add internal/cli
git commit -m "feat(cli): cleo attach and cleo kill"
```

### Task 6.7 — `cleo prune`

**Files:**
- Create: `internal/cli/prune.go`
- Create: `internal/cli/prune_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Test**

```go
func TestPruneArchivesFinishedSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	for _, st := range []state.State{state.Completed, state.Errored, state.Dead, state.Running, state.Idle} {
		_ = c.State.Put(state.Session{
			ID:    "cleo-foo-claude-" + string(st),
			ProjectID: "foo", Agent: "claude", Name: string(st), State: st,
		})
	}
	cmd := newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--keep", "0", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	all, _ := c.State.List()
	for _, s := range all {
		if s.State.IsFinished() {
			t.Errorf("finished still present: %s", s.ID)
		}
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/cli/prune.go
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func newPruneCmd(getCtx func() *Ctx) *cobra.Command {
	var keep int
	var all bool
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "prune [project]",
		Short: "Remove finished sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if keep < 0 {
				keep = c.Config.Retention.PruneKeepDefault
			}
			projectFilter := ""
			if len(args) == 1 {
				projectFilter = args[0]
			}
			sessions, _ := c.State.List()
			candidates := []string{}
			byProj := map[string][]int{}
			for i, s := range sessions {
				if !s.State.IsFinished() {
					continue
				}
				if !all && projectFilter != "" && s.ProjectID != projectFilter {
					continue
				}
				byProj[s.ProjectID] = append(byProj[s.ProjectID], i)
			}
			for _, idxs := range byProj {
				sort.Slice(idxs, func(i, j int) bool {
					return sessions[idxs[i]].LastEventAt.After(sessions[idxs[j]].LastEventAt)
				})
				for i, idx := range idxs {
					if i < keep {
						continue
					}
					candidates = append(candidates, sessions[idx].ID)
				}
			}
			if dryRun {
				for _, id := range candidates {
					fmt.Fprintln(cmd.OutOrStdout(), id)
				}
				return nil
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "prune %d session(s)? [y/N] ", len(candidates))
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			for _, id := range candidates {
				_ = events.Archive(c.Paths.EventsLog(id), c.Paths.ArchiveDir())
				_ = c.State.Delete(id)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pruned %d session(s)\n", len(candidates))
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", -1, "keep N most recent finished per project (default config)")
	cmd.Flags().BoolVar(&all, "all", false, "across all projects")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without removing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
```

- [ ] **Step 3: Wire, test, commit**

```bash
git add internal/cli
git commit -m "feat(cli): cleo prune archives finished sessions"
```

### Phase 6 Gate — Manual Test + Bugfix Sprint

This is the first user-visible milestone. Test thoroughly — Phase 7 builds on a working CLI.

**Manual checks (all must pass before Phase 7):**

Use `XDG_CONFIG_HOME=$(mktemp -d)` so manual tests don't touch your real `~/.config/cleo`.

- [ ] `go test ./...` green (all packages)
- [ ] `./bin/cleo --help` lists all subcommands: `init`, `add`, `rm`, `run`, `ls`, `attach`, `kill`, `prune`
- [ ] `./bin/cleo add /tmp/foo` registers; `./bin/cleo ls` shows it
- [ ] `cleo add` (no args) registers cwd; the path stored is absolute
- [ ] `cleo add` of an already-registered absolute path → friendly error (no dupe)
- [ ] `cleo rm <id>` removes; subsequent `ls` confirms gone
- [ ] `cleo ls` table is aligned, has columns PROJECT/AGENT/NAME/STATE/ID
- [ ] `cleo run claude --name fix-auth-bug --cwd /tmp/foo --yes` (with `/tmp/foo` registered) creates tmux session `cleo-foo-claude-fix-auth-bug`; `tmux ls` shows it
- [ ] State.json after run: contains the session, `state: "spawning"`, `agent: "claude"`, env `CLEO_SESSION_ID` was passed (verify by attaching and `echo $CLEO_SESSION_ID`)
- [ ] `cleo run claude --cwd /unregistered/path` → confirmation prompt; "y" registers + spawns; "n" aborts cleanly
- [ ] `cleo run unknownAgent --cwd /tmp/foo --yes` → friendly error listing configured agents
- [ ] `cleo run claude --cwd /tmp/foo --yes` twice (no `--name`) → second session has slug `2` (counter advanced)
- [ ] `cleo attach <id>` drops into tmux; `Ctrl-b d` returns to shell
- [ ] `cleo kill <id> --yes` removes from state.json AND from tmux (`tmux ls` no longer shows it)
- [ ] Seed state.json with finished sessions; `cleo prune --keep 0 --yes` removes them; archive `.gz` files appear in `events/archive/`
- [ ] `cleo prune --dry-run` lists candidate IDs to stdout, makes no changes
- [ ] `cleo prune --keep 5` keeps the 5 most-recent finished sessions per project, removes older

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 7 until empty.

---

## Phase 7 — Hooks

### Task 7.1 — `cleo hook` dispatcher

**Files:**
- Create: `internal/cli/hook.go`
- Create: `internal/hooks/handler.go`
- Create: `internal/hooks/handler_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Failing test — claude PreToolUse fires PreToolUse event**

```go
// internal/hooks/handler_test.go
package hooks

import (
	"bytes"
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
```

- [ ] **Step 2: Implement handler skeleton + claude protocol**

```go
// internal/hooks/handler.go
package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type Player interface {
	Play(file string) error
	Available() bool
}

type Deps struct {
	Paths  paths.Paths
	State  *state.Store
	Config config.Config
	Events func(sid string) *events.Log
	Sound  Player
	// Now returns the cleo session id from env or returns an error if absent.
	// Test seam.
	Now func() (string, error)
}

func DefaultNow() (string, error) {
	sid := os.Getenv("CLEO_SESSION_ID")
	if sid == "" {
		return "", errNoSession
	}
	return sid, nil
}

var errNoSession = fmt.Errorf("CLEO_SESSION_ID not set")

func Handle(d Deps, protocol, event string, stdin io.Reader, stdout io.Writer) error {
	sid, err := d.Now()
	if err != nil {
		return nil // silent no-op
	}
	body, _ := io.ReadAll(stdin)
	switch protocol {
	case "claude":
		return handleClaude(d, sid, event, body)
	case "codex":
		return handleCodex(d, sid, event, body)
	case "none":
		return nil
	}
	return fmt.Errorf("unknown protocol %q", protocol)
}

type claudePayload struct {
	ToolName string `json:"tool_name"`
	Message  string `json:"message"`
	Reason   string `json:"reason"`
}

func handleClaude(d Deps, sid, event string, body []byte) error {
	var p claudePayload
	_ = json.Unmarshal(body, &p)

	var ev state.Event
	var soundEv string
	var msg string
	switch event {
	case "SessionStart":
		ev, soundEv = state.EvSessionStart, "session_start"
	case "PreToolUse":
		ev = state.EvPreToolUse
	case "PostToolUse":
		ev = state.EvPostToolUse
	case "Notification":
		ev, soundEv = state.EvNotification, "needs_input"
		msg = p.Message
	case "Stop":
		ev, soundEv = state.EvStop, "session_idle"
	case "SessionEnd":
		ev, soundEv = state.EvSessionEnd, "session_completed"
	case "SubagentStop":
		ev = state.EvPostToolUse // ish; logged but no top-level transition
	default:
		return nil
	}

	if _, err := d.State.Apply(sid, ev, msg); err != nil {
		return err
	}
	_ = d.Events(sid).Append(events.Entry{Type: event, Tool: p.ToolName, Detail: p.Message})
	if soundEv != "" && d.Config.Sound.Enabled {
		if file := d.Config.Sound.Events[soundEv]; file != "" {
			full := file
			if !filepath.IsAbs(full) {
				full = filepath.Join(d.Paths.SoundsDir(), full)
			}
			_ = d.Sound.Play(full)
		}
	}
	return nil
}

// (Add `"path/filepath"` to the imports at the top of handler.go.)
```

- [ ] **Step 3: Stub Codex handler**

```go
// in handler.go
func handleCodex(d Deps, sid, event string, body []byte) error {
	// TODO: confirm exact codex hook event names against current CLI release.
	// Conceptual mapping from spec §6.2:
	//   "start" → SessionStart, "pre-tool" → PreToolUse, "post-tool" → PostToolUse,
	//   "awaiting-input" → Notification, "done" → Stop.
	return handleClaude(d, sid, mapCodexEvent(event), body)
}

func mapCodexEvent(e string) string {
	switch e {
	case "start":
		return "SessionStart"
	case "pre-tool":
		return "PreToolUse"
	case "post-tool":
		return "PostToolUse"
	case "awaiting-input":
		return "Notification"
	case "done":
		return "Stop"
	case "session-end":
		return "SessionEnd"
	}
	return e
}
```

- [ ] **Step 4: CLI subcommand**

```go
// internal/cli/hook.go
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newHookCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:    "hook <protocol> <event>",
		Short:  "Internal: invoked by hook configs",
		Args:   cobra.ExactArgs(2),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			deps := hooks.Deps{
				Paths:  c.Paths,
				State:  c.State,
				Config: c.Config,
				Events: c.Events,
				Sound:  c.Player,
				Now:    hooks.DefaultNow,
			}
			return hooks.Handle(deps, args[0], args[1], os.Stdin, cmd.OutOrStdout())
		},
	}
}
```

- [ ] **Step 5: Wire, test, commit**

```bash
go test ./internal/hooks/... ./internal/cli/...
git add internal/hooks internal/cli
git commit -m "feat(hooks): cleo hook dispatcher with claude + codex protocols"
```

### Task 7.2 — `cleo init` (install hooks)

**Files:**
- Create: `internal/hooks/install.go`
- Create: `internal/hooks/install_test.go`
- Create: `internal/cli/init.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Failing test**

```go
// internal/hooks/install_test.go
package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallClaudeHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(settingsPath)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing")
	}
	for _, ev := range []string{"PreToolUse", "PostToolUse", "Notification", "Stop", "SessionStart", "SessionEnd"} {
		if hooks[ev] == nil {
			t.Errorf("missing %s", ev)
		}
	}
	// And the path is absolute
	if !strings.Contains(string(b), "/usr/local/bin/cleo hook claude") {
		t.Errorf("hook command not present: %s", string(b))
	}
}

func TestInstallClaudeRefusesPreExistingDifferentValue(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	prior := `{"hooks":{"PreToolUse":[{"hooks":[{"command":"some-other-tool"}]}]}}`
	_ = os.WriteFile(settingsPath, []byte(prior), 0o644)

	err := InstallClaude(settingsPath, "/cleo")
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got %v", err)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/hooks/install.go
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var claudeEvents = []string{"PreToolUse", "PostToolUse", "Notification", "Stop", "SessionStart", "SessionEnd", "SubagentStop"}

func InstallClaude(settingsPath, cleoBin string) error {
	b, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		b = []byte("{}")
	} else if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return fmt.Errorf("settings.json: %w", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, ev := range claudeEvents {
		want := []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": fmt.Sprintf("%s hook claude %s", cleoBin, ev),
						"timeout": 2,
					},
				},
			},
		}
		if existing, ok := hooks[ev]; ok {
			if !equalsHook(existing, want) {
				return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
			}
		}
		hooks[ev] = want
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	return os.WriteFile(settingsPath, out, 0o644)
}

func equalsHook(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
```

- [ ] **Step 3: CLI command + binary path resolution**

```go
// internal/cli/init.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)

func newInitCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install hooks into ~/.claude/settings.json (and codex)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if err := sound.ExtractDefaults(c.Paths.SoundsDir()); err != nil {
				return err
			}
			cleoBin, err := os.Executable()
			if err != nil {
				return err
			}
			cleoBin, _ = filepath.Abs(cleoBin)
			home, _ := os.UserHomeDir()
			claudeSettings := filepath.Join(home, ".claude", "settings.json")
			if err := hooks.InstallClaude(claudeSettings, cleoBin); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "installed claude hooks at", claudeSettings)
			// Codex install: TODO once event names confirmed; placeholder note.
			fmt.Fprintln(cmd.OutOrStdout(), "codex hook install: pending — see spec §17")
			return nil
		},
	}
}
```

- [ ] **Step 4: Wire, test, commit**

```bash
go test ./internal/hooks/... ./internal/cli/...
git add internal/hooks internal/cli
git commit -m "feat(hooks): cleo init writes hook entries into claude settings"
```

### Phase 7 Gate — Manual Test + Bugfix Sprint

This phase touches the user's real `~/.claude/settings.json`. **Back it up first**: `cp ~/.claude/settings.json ~/.claude/settings.json.bak`.

**Manual checks (all must pass before Phase 8):**

- [ ] `go test ./internal/hooks/... ./internal/cli/...` green
- [ ] `cleo init` writes hook entries to `~/.claude/settings.json`; inspect the file — entries point to the absolute cleo binary path, all 7 events present, `timeout: 2`
- [ ] `cleo init` is idempotent — second run produces no diff (compare with `diff` against the post-first-run version)
- [ ] Manually edit settings.json so PreToolUse points to a different command; rerun `cleo init` → conflict error, settings unchanged
- [ ] **Real-claude integration:** spawn `cleo run claude --cwd <real-project>`; attach; ask claude to use a tool (e.g. "list files in this directory"). Detach. Inspect state.json — state should be `running` then `idle` after Stop. Tool count > 0.
- [ ] events.jsonl for that session exists, contains entries for SessionStart, PreToolUse, PostToolUse, Stop with timestamps
- [ ] Notification flow: ask claude to run a command requiring approval; verify state transitions to `waiting_for_input`, `last_message` is set, sound plays
- [ ] Sound: with `[sound] enabled = true`, the `attention.wav` plays on Notification; `done.wav` plays on Stop
- [ ] Sound mute: edit config to `enabled = false`, repeat — no sound
- [ ] Hook failure resilience: temporarily `chmod -x` the cleo binary; trigger a hook; claude should not crash; restore permissions afterward
- [ ] `~/.claude/settings.json` restored from backup at end of phase to avoid polluting your real environment

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 8 until empty. Codex hook event names should be confirmed during this phase (spec §17 risk).

---

## Phase 8 — Reconciler

### Task 8.1 — Reconciler logic

**Files:**
- Create: `internal/reconcile/reconcile.go`
- Create: `internal/reconcile/reconcile_test.go`

- [ ] **Step 1: Failing test**

```go
// internal/reconcile/reconcile_test.go
package reconcile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type fakeTmux struct{ existing []string }

func (f *fakeTmux) LsPrefix(string) ([]string, error) { return f.existing, nil }

func TestReconcileMarksMissingSessionsDead(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "lock"))
	_ = st.Put(state.Session{ID: "cleo-foo-claude-1", State: state.Running, LastEventAt: time.Now()})
	_ = st.Put(state.Session{ID: "cleo-bar-claude-1", State: state.Running, LastEventAt: time.Now()})

	tx := &fakeTmux{existing: []string{"cleo-foo-claude-1"}}
	if err := Run(st, tx, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-bar-claude-1")
	if got.State != state.Dead {
		t.Errorf("expected dead, got %s", got.State)
	}
	got, _ = st.Get("cleo-foo-claude-1")
	if got.State != state.Running {
		t.Errorf("expected still running, got %s", got.State)
	}
}

func TestReconcileIdleTimeoutPromotesToCompleted(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "lock"))
	_ = st.Put(state.Session{
		ID: "cleo-foo-claude-1", State: state.Idle, LastEventAt: time.Now().Add(-30 * time.Minute),
	})
	tx := &fakeTmux{existing: []string{"cleo-foo-claude-1"}}
	if err := Run(st, tx, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-foo-claude-1")
	if got.State != state.Completed {
		t.Errorf("expected completed, got %s", got.State)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/reconcile/reconcile.go
package reconcile

import (
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type TmuxLs interface {
	LsPrefix(prefix string) ([]string, error)
}

func Run(st *state.Store, tx TmuxLs, idleTimeout time.Duration) error {
	live, err := tx.LsPrefix("cleo-")
	if err != nil {
		return err
	}
	liveSet := map[string]bool{}
	for _, n := range live {
		liveSet[n] = true
	}
	sessions, err := st.List()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if !liveSet[s.ID] && s.State != state.Dead {
			_, _ = st.Apply(s.ID, state.EvDead, "")
			continue
		}
		if s.State == state.Idle && time.Since(s.LastEventAt) > idleTimeout {
			_, _ = st.Apply(s.ID, state.EvIdleTimeout, "")
		}
	}
	return nil
}
```

- [ ] **Step 3: Run, verify PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/reconcile
git commit -m "feat(reconcile): mark missing sessions dead and apply idle timeout"
```

### Task 8.2 — Wire reconciler into TUI launch and `cleo ls`

**Files:**
- Modify: `internal/cli/ls.go` (call reconcile.Run before listing)
- Modify: `internal/cli/root.go` (call reconcile.Run on TUI launch — once Phase 9 lands; for now, in `cleo ls`)

- [ ] **Step 1: Modify ls to reconcile first**

In `newLsCmd`, before listing, call:

```go
_ = reconcile.Run(c.State, c.Tmux, c.Config.Retention.IdleToCompletedTimeout)
```

(`tmux.Client` already has `LsPrefix` — satisfies interface.)

- [ ] **Step 2: Add an integration test against the fake tmux**

```go
// internal/cli/ls_test.go: extend existing test
// (omitted for brevity; add a session not in fake.exists, run cleo ls, assert state shows dead)
```

- [ ] **Step 3: Run, commit**

```bash
git add internal/cli
git commit -m "feat(cli): reconcile state before ls"
```

### Phase 8 Gate — Manual Test + Bugfix Sprint

**Manual checks (all must pass before Phase 9):**

- [ ] `go test ./internal/reconcile/... ./internal/cli/...` green
- [ ] Manual: spawn agent via `cleo run`; in another shell `tmux kill-session -t <id>`; run `cleo ls` → state shows `dead`
- [ ] Manual: edit state.json to set a session's `state: "idle"` and `last_event_at` to 30 minutes ago; run `cleo ls` → state promoted to `completed` (idle timeout)
- [ ] Reconciler is idempotent: run `cleo ls` twice in a row, the second is a no-op (no spurious state changes)
- [ ] Cleo-prefixed tmux session not in state.json (manually `tmux new-session -d -s cleo-foo-bar-1`) — reconciler logs and ignores per spec §17 (verify hook-errors.log or stdout is calm)
- [ ] Reconciler does not nuke `running` sessions just because one tmux call momentarily failed; rerun with tmux server up — state is correct

**Bugfix sprint:** failing test → fix → green → commit. Do not advance to Phase 9 until empty.

---

## Phase 9 — TUI (Bubble Tea)

This phase is intentionally less prescriptive on per-line TDD — Bubble Tea apps are best validated with `teatest` golden snapshots and manual smoke tests. The structure below decomposes the model into focused files.

### Task 9.1 — Add Bubble Tea dependencies + skeleton

**Files:**
- Modify: `go.mod`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keymap.go`
- Create: `internal/tui/model.go`
- Create: `internal/tui/update.go`
- Create: `internal/tui/view.go`
- Create: `internal/tui/poll.go`
- Modify: `internal/cli/root.go` (replace stub with `tui.Run(ctx)`)

- [ ] **Step 1: Add deps**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/fsnotify/fsnotify@latest
```

- [ ] **Step 2: Skeleton model**

```go
// internal/tui/model.go
package tui

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type Model struct {
	ctx        *cli.Ctx
	projects   []projects.Project
	sessions   []state.Session
	cursor     cursor
	expanded   map[string]bool // project id → expanded
	filter     string
	filterMode bool
	mode       Mode
	popup      tea.Model
	help       help.Model
	width, height int
	err        error
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeFilter
	ModePopup
)

type cursor struct {
	projectIdx int
	agentIdx   int // -1 = on the project row
}

func New(ctx *cli.Ctx) Model {
	return Model{
		ctx:      ctx,
		expanded: map[string]bool{},
		help:     help.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadStateCmd(m.ctx), tickStateCmd())
}
```

- [ ] **Step 3: Update / View skeletons**

```go
// internal/tui/update.go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case stateLoadedMsg:
		m.projects = msg.projects
		m.sessions = msg.sessions
		return m, nil
	case tickStateMsg:
		return m, tea.Batch(loadStateCmd(m.ctx), tickStateCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}
```

```go
// internal/tui/view.go
package tui

func (m Model) View() string {
	// Composed in tasks below: header, sidebar, main pane, footer.
	return renderFrame(m)
}
```

- [ ] **Step 4: Run command in cli**

```go
// internal/cli/root.go (replace stub RunE)
RunE: func(cmd *cobra.Command, args []string) error {
	c, err := NewCtx()
	if err != nil {
		return err
	}
	return tui.Run(c)
},
```

```go
// internal/tui/run.go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dhruvsaxena1998/cleo/internal/cli"
)

func Run(c *cli.Ctx) error {
	_, err := tea.NewProgram(New(c), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
```

- [ ] **Step 5: Verify it builds and launches**

```bash
make build
./bin/cleo
```

(Press `q` or `ctrl-c` to quit. View will be empty for now — fleshed out in next tasks.)

- [ ] **Step 6: Commit**

```bash
git add internal/tui internal/cli go.mod go.sum
git commit -m "feat(tui): bubbletea skeleton with state polling"
```

### Task 9.2 — Polling commands

**Files:**
- Modify: `internal/tui/poll.go`

- [ ] **Step 1: Implement**

```go
// internal/tui/poll.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type stateLoadedMsg struct {
	projects []projects.Project
	sessions []state.Session
}

type tickStateMsg struct{}

type paneCapturedMsg struct {
	sid     string
	content string
}

func loadStateCmd(c *cli.Ctx) tea.Cmd {
	return func() tea.Msg {
		ps, _ := c.Projects.List()
		ss, _ := c.State.List()
		return stateLoadedMsg{projects: ps, sessions: ss}
	}
}

func tickStateCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(time.Time) tea.Msg { return tickStateMsg{} })
}

func capturePaneCmd(c *cli.Ctx, sid string, lines int) tea.Cmd {
	return func() tea.Msg {
		out, _ := c.Tmux.CapturePane(sid, lines)
		return paneCapturedMsg{sid: sid, content: out}
	}
}
```

- [ ] **Step 2: Run, commit**

```bash
git add internal/tui
git commit -m "feat(tui): state polling and pane capture commands"
```

### Task 9.3 — Sidebar rendering

**Files:**
- Create: `internal/tui/sidebar.go`
- Modify: `internal/tui/view.go`
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: Define style helpers**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleProject  = lipgloss.NewStyle().Bold(true)
	styleDimmed   = lipgloss.NewStyle().Faint(true)
	styleSelected = lipgloss.NewStyle().Reverse(true)
	stylePanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func agentLabel(label, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("[" + label + "]")
}

func stateGlyph(s string) string {
	switch s {
	case "running":
		return "●"
	case "waiting_for_input":
		return "◐"
	case "idle":
		return "○"
	case "completed":
		return "✓"
	case "error":
		return "✗"
	case "dead":
		return "☠"
	}
	return "·"
}
```

- [ ] **Step 2: Sidebar**

```go
// internal/tui/sidebar.go
package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (m Model) renderSidebar(width int) string {
	var b strings.Builder
	b.WriteString(styleProject.Render("Projects") + "\n\n")
	if m.filter != "" {
		b.WriteString(styleDimmed.Render("/"+m.filter) + "\n")
	}
	projects := append([]projects.Project(nil), m.projects...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	for pi, p := range projects {
		if !m.matchesFilter(p.ID) && !m.projectHasMatching(p.ID) {
			continue
		}
		caret := "▶"
		if m.expanded[p.ID] {
			caret = "▼"
		}
		line := fmt.Sprintf("%s %s", caret, p.ID)
		if pi == m.cursor.projectIdx && m.cursor.agentIdx == -1 {
			line = styleSelected.Render(line)
		}
		b.WriteString(line + "\n")
		if !m.expanded[p.ID] {
			continue
		}
		ss := m.sessionsFor(p.ID)
		for ai, s := range ss {
			cfgAgent := m.ctx.Config.Agents[s.Agent]
			label := agentLabel(cfgAgent.Label, cfgAgent.Color)
			row := fmt.Sprintf("  %s %-20s %s %s", label, truncate(s.Name, 20), stateGlyph(string(s.State)), shortState(s.State))
			if pi == m.cursor.projectIdx && ai == m.cursor.agentIdx {
				row = styleSelected.Render(row)
			}
			b.WriteString(row + "\n")
		}
	}
	return b.String()
}

func (m Model) sessionsFor(pid string) []state.Session {
	var out []state.Session
	for _, s := range m.sessions {
		if s.ProjectID == pid && m.matchesFilter(s.ID, s.Name, s.Agent) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (m Model) matchesFilter(parts ...string) bool {
	if m.filter == "" {
		return true
	}
	for _, p := range parts {
		if strings.Contains(strings.ToLower(p), strings.ToLower(m.filter)) {
			return true
		}
	}
	return false
}

func (m Model) projectHasMatching(pid string) bool {
	for _, s := range m.sessions {
		if s.ProjectID == pid && m.matchesFilter(s.ID, s.Name, s.Agent) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func shortState(s state.State) string {
	switch s {
	case state.WaitingForInput:
		return "waiting"
	case state.Running:
		return "running"
	}
	return string(s)
}
```

(Add `"github.com/dhruvsaxena1998/cleo/internal/projects"` import at top.)

- [ ] **Step 3: Compose frame in view**

```go
// internal/tui/view.go
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func renderFrame(m Model) string {
	side := m.renderSidebar(m.ctx.Config.UI.SidebarWidth)
	main := m.renderMain(m.width - m.ctx.Config.UI.SidebarWidth - 4)
	frame := lipgloss.JoinHorizontal(lipgloss.Top,
		stylePanel.Width(m.ctx.Config.UI.SidebarWidth).Render(side),
		stylePanel.Width(m.width-m.ctx.Config.UI.SidebarWidth-4).Render(main),
	)
	return frame + "\n" + m.renderFooter()
}

func (m Model) renderFooter() string {
	return styleDimmed.Render("n new  v view  ↵ attach  k kill  / filter  m mute  ? help  q quit")
}
```

- [ ] **Step 4: Stub renderMain (filled in next task)**

```go
// internal/tui/main_pane.go
package tui

func (m Model) renderMain(width int) string {
	return "select an agent to view"
}
```

- [ ] **Step 5: Run, screenshot, commit**

```bash
make build && ./bin/cleo
# manually verify rendering with mocked projects/sessions
```

```bash
git add internal/tui
git commit -m "feat(tui): sidebar with projects, agents, and filter awareness"
```

### Task 9.4 — Main view pane (events log + pane mirror)

**Files:**
- Modify: `internal/tui/main_pane.go` (rename or extend)
- Modify: `internal/tui/poll.go` (start capture-pane ticker when an agent is selected)

- [ ] **Step 1: Implement**

```go
// internal/tui/main_pane.go
package tui

import (
	"fmt"
	"strings"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func (m Model) renderMain(width int) string {
	sess, ok := m.selectedSession()
	if !ok {
		return styleDimmed.Render("press v on an agent to view")
	}
	header := fmt.Sprintf("%s\n%s  started %s ago  tools: %d  last: %s",
		sess.ID, stateGlyph(string(sess.State)),
		humanDuration(sess.StartedAt), sess.ToolCount, humanDuration(sess.LastEventAt))

	log := m.ctx.Events(sess.ID)
	entries, _ := log.Tail(m.ctx.Config.UI.EventLogLines)
	var lines []string
	for _, e := range entries {
		lines = append(lines, formatEntry(e))
	}
	eventsBlock := strings.Join(lines, "\n")

	pane := m.paneCache[sess.ID]
	return strings.Join([]string{
		header,
		"── recent ─────",
		eventsBlock,
		"── pane preview ─────",
		truncateLines(pane, m.ctx.Config.UI.PanePreviewLines),
	}, "\n")
}

func formatEntry(e events.Entry) string {
	return fmt.Sprintf("%s  %s  %s", e.At.Format("15:04:05"), e.Type, e.Tool)
}

func truncateLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func humanDuration(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
```

- [ ] **Step 2: Hook up paneCache, selection, and capture ticker**

Add `paneCache map[string]string` to `Model`. On `v` keypress in `update.go`, mark a session selected and queue a `capturePaneCmd`. Re-tick periodically.

- [ ] **Step 3: Run, smoke test, commit**

```bash
git add internal/tui
git commit -m "feat(tui): main view pane with events log and pane mirror"
```

### Task 9.5 — Keybindings, spawn popup, kill confirm, filter mode, mute

**Files:**
- Create: `internal/tui/popup_spawn.go`
- Create: `internal/tui/popup_confirm.go`
- Create: `internal/tui/filter.go`
- Modify: `internal/tui/update.go` (handleKey)

- [ ] **Step 1: Centralize keymap**

```go
// internal/tui/keymap.go
package tui

import "github.com/charmbracelet/bubbles/key"

type Keymap struct {
	Up, Down, Left, Right, Enter, New, View, Kill, Rename, Add, Filter, Mute, Help, Quit, Esc key.Binding
}

func DefaultKeymap() Keymap {
	return Keymap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "attach")),
		New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		View:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		Kill:   key.NewBinding(key.WithKeys("K", "ctrl+k"), key.WithHelp("K", "kill")),
		Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add project")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Mute:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Esc:    key.NewBinding(key.WithKeys("esc")),
	}
}
```

- [ ] **Step 2: Spawn popup**

```go
// internal/tui/popup_spawn.go
package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SpawnPopup struct {
	agents    []string
	cursor    int
	nameInput textinput.Model
	focusName bool
	projectID string
}

func NewSpawnPopup(projectID string, agents []string) SpawnPopup {
	sorted := append([]string(nil), agents...)
	sort.Strings(sorted)
	ti := textinput.New()
	ti.Placeholder = "(optional)"
	ti.CharLimit = 64
	return SpawnPopup{agents: sorted, nameInput: ti, projectID: projectID}
}

type SpawnSubmitted struct {
	ProjectID string
	Agent     string
	Name      string
}
type SpawnCancelled struct{}

func (p SpawnPopup) Init() tea.Cmd { return textinput.Blink }

func (p SpawnPopup) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Spawn agent in '%s'\n\n", p.projectID)
	b.WriteString("Agent:\n")
	for i, a := range p.agents {
		marker := "  "
		if i == p.cursor && !p.focusName {
			marker = "▸ "
		}
		b.WriteString(marker + a + "\n")
	}
	b.WriteString("\nName: ")
	b.WriteString(p.nameInput.View())
	b.WriteString("\n\ntab switch field   ↵ spawn   esc cancel")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(b.String())
}

func (p SpawnPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg { return SpawnCancelled{} }
		case "tab":
			p.focusName = !p.focusName
			if p.focusName {
				p.nameInput.Focus()
			} else {
				p.nameInput.Blur()
			}
			return p, nil
		case "enter":
			if len(p.agents) == 0 {
				return p, func() tea.Msg { return SpawnCancelled{} }
			}
			return p, func() tea.Msg {
				return SpawnSubmitted{
					ProjectID: p.projectID,
					Agent:     p.agents[p.cursor],
					Name:      strings.TrimSpace(p.nameInput.Value()),
				}
			}
		case "up", "k":
			if !p.focusName && p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "down", "j":
			if !p.focusName && p.cursor < len(p.agents)-1 {
				p.cursor++
			}
			return p, nil
		}
	}
	if p.focusName {
		var cmd tea.Cmd
		p.nameInput, cmd = p.nameInput.Update(msg)
		return p, cmd
	}
	return p, nil
}
```

- [ ] **Step 3: Kill confirmation popup**

Generic confirm box that returns `ConfirmedYes{}` or `ConfirmedNo{}` messages.

- [ ] **Step 4: Filter mode**

In `handleKey`, when `m.mode == ModeFilter`:
- `esc` clears filter and returns to normal mode
- `enter` freezes filter and returns to normal mode (filter stays applied)
- backspace edits
- printable runes append to `m.filter`

- [ ] **Step 5: Wire all keys in handleKey**

```go
// internal/tui/update.go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == ModeFilter {
		return m.handleFilterKey(msg)
	}
	if m.mode == ModePopup {
		var cmd tea.Cmd
		m.popup, cmd = m.popup.Update(msg)
		return m, cmd
	}
	km := DefaultKeymap()
	switch {
	case key.Matches(msg, km.Quit):
		return m, tea.Quit
	case key.Matches(msg, km.Filter):
		m.mode = ModeFilter
		return m, nil
	case key.Matches(msg, km.New):
		return m.openSpawnPopup()
	case key.Matches(msg, km.View):
		return m.viewSelectedAgent()
	case key.Matches(msg, km.Enter):
		return m.attachSelectedAgent()
	case key.Matches(msg, km.Kill):
		return m.confirmKill()
	case key.Matches(msg, km.Mute):
		return m.toggleMute()
	case key.Matches(msg, km.Up):
		return m.cursorUp()
	case key.Matches(msg, km.Down):
		return m.cursorDown()
	}
	return m, nil
}
```

Each helper (`openSpawnPopup`, `attachSelectedAgent`, etc.) is ~5-15 lines and ties to existing CLI code paths where possible (e.g., `attachSelectedAgent` returns `tea.ExecProcess` to run `tmux attach`).

- [ ] **Step 6: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): keybindings, spawn popup, kill confirm, filter mode"
```

### Task 9.6 — Retention banner

**Files:**
- Modify: `internal/tui/view.go`

- [ ] **Step 1: Compute and render**

In `renderFrame`, before composing horizontal layout, compute per-project finished counts and emit a one-line banner if any exceed `Retention.HintThreshold`:

```go
banner := ""
counts := map[string]int{}
for _, s := range m.sessions {
	if s.State.IsFinished() {
		counts[s.ProjectID]++
	}
}
for pid, n := range counts {
	if n > m.ctx.Config.Retention.HintThreshold {
		banner = fmt.Sprintf("💡 '%s' has %d finished sessions — run `cleo prune %s` to clean up", pid, n, pid)
		break
	}
}
out := frame
if banner != "" {
	out = styleDimmed.Render(banner) + "\n" + frame
}
return out + "\n" + m.renderFooter()
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): retention advisory banner"
```

### Task 9.7 — TUI snapshot tests

**Files:**
- Create: `internal/tui/tui_test.go`

Use `github.com/charmbracelet/x/exp/teatest` to drive the TUI with synthetic input and assert on the rendered output.

- [ ] **Step 1: Add dep**

```bash
go get github.com/charmbracelet/x/exp/teatest@latest
```

- [ ] **Step 2: One smoke snapshot**

```go
// internal/tui/tui_test.go
package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestSidebarRendersProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := cli.NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdirAll(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// expand the project
	tm.Send(tea.KeyMsg{Type: tea.KeySpace})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "[cl]") && contains(b, "running")
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}

func contains(b []byte, s string) bool {
	return strings.Contains(string(b), s)
}
```

(Add `import "strings"` and a `mkdirAll` helper.)

- [ ] **Step 3: Run, commit**

```bash
go test ./internal/tui/...
git add internal/tui go.mod go.sum
git commit -m "test(tui): snapshot test for sidebar rendering"
```

### Phase 9 Gate — Manual Test + Bugfix Sprint

This is the largest manual-test phase. Test in real terminals (Terminal.app, iTerm2, Alacritty, kitty if available) since rendering varies.

**Manual checks (all must pass before Phase 10):**

Setup: have at least 2 registered projects and 4+ sessions in mixed states (some running, some idle, some completed).

- [ ] `./bin/cleo` launches into the alt screen; `q` cleanly returns to shell with terminal restored
- [ ] Initial render: sidebar shows both projects collapsed; main pane shows `select an agent to view`; footer lists keybindings
- [ ] Resize the terminal — layout reflows without crash; very narrow widths (60 cols) degrade gracefully
- [ ] Navigation: `↑/↓` (and `j/k`) moves cursor through projects and (when expanded) agents
- [ ] `space` collapses/expands a project; cursor stays on the project line
- [ ] Brand colors render — `[cl]` is clay, `[cx]` is green, `[oc]` is orange, `[pi]` is purple
- [ ] State glyphs render (`●` for running, `◐` for waiting, etc.) and update within ~1s of an underlying state change (verify by triggering a hook event in another shell)
- [ ] `n` on a project → spawn popup appears with bordered box, agent list, name field
- [ ] Spawn popup: arrows pick agent; `tab` toggles focus; `enter` submits → popup closes, new session appears in sidebar within 1s, toast shown
- [ ] Spawn popup: `esc` cancels without spawning
- [ ] Spawn popup: empty name → session named `<agent>-<n>` (counter)
- [ ] Spawn popup: typed name "Fix Auth!" → slug `fix-auth`
- [ ] `v` on an agent → main pane shows session header, events log (last N), pane mirror (last 30 lines, updating every 1.5s)
- [ ] `enter` on an agent → drops you into the tmux session interactively; `Ctrl-b d` returns to cleo TUI without redraw glitches
- [ ] `k` on agent → confirm popup; `y` kills (session disappears within 1s), `n`/`esc` cancels
- [ ] `r` on agent → inline rename; new name reflected in tmux (`tmux ls` shows new session name)
- [ ] `r` on project → inline rename of project display name
- [ ] `/` enters filter mode; typing reduces sidebar to matching items; `esc` clears; `enter` freezes filter (header shows the active filter)
- [ ] Filter matches across project name, agent slug, session name (case-insensitive substring)
- [ ] `m` toggles mute; toast confirms; config.toml updated on disk; subsequent state changes don't play sounds
- [ ] `?` shows help overlay listing all keys
- [ ] Retention banner: seed state.json with 7+ finished sessions for one project; relaunch cleo → banner appears at top
- [ ] Multiple cleo TUI instances: open two terminals running `./bin/cleo` against the same `XDG_CONFIG_HOME`; both render correctly; spawning in one updates the other within 1s
- [ ] No goroutine leaks: spawn/kill 20 sessions through the TUI; quit; check `pgrep` for leftover cleo processes
- [ ] Empty state: clear all projects/sessions; cleo renders an empty but non-broken sidebar with `(no projects)` hint

**Bugfix sprint:** failing test or repro → fix → green → commit. The TUI test surface is wide; expect this sprint to be the longest. Do not advance to Phase 10 until empty.

---

## Phase 10 — Integration, distribution, README

### Task 10.1 — End-to-end smoke script

**Files:**
- Create: `scripts/smoke.sh`

- [ ] **Step 1: Write smoke**

```bash
#!/usr/bin/env bash
# scripts/smoke.sh
# Manual end-to-end smoke. Requires: tmux installed, claude CLI installed.
set -euo pipefail

CLEO_HOME=$(mktemp -d)
export XDG_CONFIG_HOME="$CLEO_HOME"

bin=./bin/cleo

trap 'tmux kill-server 2>/dev/null || true' EXIT

$bin add /tmp
$bin ls
$bin run claude --name smoke --cwd /tmp --yes
$bin ls | grep smoke
$bin kill cleo-tmp-claude-smoke --yes
$bin ls
echo "smoke OK"
```

```bash
chmod +x scripts/smoke.sh
./scripts/smoke.sh
```

- [ ] **Step 2: Commit**

```bash
git add scripts
git commit -m "test: end-to-end smoke script"
```

### Task 10.2 — README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Sections: install, quick start, commands, configuration, hooks (claude/codex), troubleshooting, dev. Keep terse.

```markdown
# cleo

Terminal session manager for AI coding agents. Manages multiple Claude Code, Codex (and more) sessions in tmux with a TUI dashboard.

## Install

```bash
go install github.com/dhruvsaxena1998/cleo/cmd/cleo@latest
```

## Quick start

```bash
cleo init                # one-time: install hooks
cd ~/Dev/myapp && cleo add
cleo run claude --name fix-auth-bug
cleo                     # open TUI
```

[... rest of README ...]
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README"
```

### Task 10.3 — Goreleaser config (homebrew + GitHub releases)

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Add config**

```yaml
project_name: cleo
builds:
  - main: ./cmd/cleo
    binary: cleo
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w -X github.com/dhruvsaxena1998/cleo/internal/cli.Version={{.Version}}
archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
brews:
  - repository:
      owner: dhruvsaxena1998
      name: homebrew-tap
    homepage: https://github.com/dhruvsaxena1998/cleo
    description: Terminal session manager for AI coding agents
```

- [ ] **Step 2: Commit**

```bash
git add .goreleaser.yaml
git commit -m "chore: goreleaser config for darwin/linux + homebrew tap"
```

### Phase 10 Gate — Final Acceptance

This is the v0.1 release gate. Treat manual checks as the release-readiness checklist.

**Manual checks (all must pass before tagging v0.1):**

- [ ] `./scripts/smoke.sh` runs end-to-end without error on macOS (and Linux if available)
- [ ] `goreleaser build --snapshot --clean` produces darwin-amd64, darwin-arm64, linux-amd64, linux-arm64 binaries
- [ ] One of those binaries (the one matching dev box) runs `--version` correctly
- [ ] README install instructions: copy-paste from a clean shell, follow them, end up with a working `cleo`
- [ ] README quick-start sequence works as written
- [ ] Cross-platform: build for linux/amd64, scp to a Linux machine (or run in a Docker container), confirm `cleo --version` works
- [ ] Total run-through of v0.1 user journey: install → `cleo init` → `cleo add` → `cleo` (TUI) → spawn agent → use agent → kill → prune. End-to-end with no fatal issues.
- [ ] All `[ ]` checkboxes in earlier phase gates are checked
- [ ] `git log --oneline` reads as a coherent narrative; no `WIP` or `fixup` commits left

**Final bugfix sprint:** any release-blocking issue gets a TDD fix here before tagging. Document anything intentionally deferred to v0.2 in the README's "known limitations" section.

---

## Self-Review

### Spec coverage matrix

| Spec section                  | Plan task(s)              |
|-------------------------------|---------------------------|
| §4 architecture & lifecycle   | All phases                |
| §5.1 projects.json            | 2.1                       |
| §5.2 session.id format        | 1.2, 6.5                  |
| §5.3 state machine            | 3.1, 3.2                  |
| §5.4 file layout              | 1.1                       |
| §6.1 claude hook mapping      | 7.1                       |
| §6.2 codex hook mapping       | 7.1 (mapCodexEvent — flagged for verification) |
| §6.3 CLEO_SESSION_ID env      | 6.5, 7.1                  |
| §6.4 flock + atomic writes    | 3.2                       |
| §6.5 hook failure logging     | covered via Handle error path; minor — log file write deferred; see Open Items below |
| §7 config.toml schema         | 1.3                       |
| §8 CLI surface                | 6.1–6.7, 7.2              |
| §9 TUI layout & keys          | 9.1–9.6                   |
| §10 sound design              | 5.1, 5.2, 7.1             |
| §11 search + retention        | 9.5 (filter), 6.7 (prune), 9.6 (banner) |
| §12 failure modes             | partly covered; see Open Items |
| §13 cross-platform            | 5.2 (player), 4 (tmux test gates), 10.3 |
| §14 distribution              | 10.3                      |
| §15 testing approach          | tests in every phase      |

### Open items the implementer must complete during execution

1. **Source four real WAV assets** before publishing — the Phase 5 placeholder is silent.
2. **Confirm Codex hook event names** against the current Codex CLI release. The mapping in `mapCodexEvent` is conceptual; correct it before the codex install path lands. Spec §17 flags this as a known-risk.
3. **`hook-errors.log` writer.** Add a minimal best-effort logger to `internal/hooks/handler.go` so panics/errors append a line to `paths.HookErrLog()`. Couple-of-lines task; do as part of Phase 7 hardening if not already.
4. **`cleo init` Codex install** — currently logs "pending"; implement once event names are confirmed.
5. **TUI rename UX** for projects (the `r` keybinding) — function stubbed in keymap; implementer wires inline-edit.
6. **TUI orphan adoption** is a v0.2 concern; v0.1 reconciler logs and ignores per spec §17.

### Type / signature consistency check

- `state.Session` fields used in: `state` (3.1), `cli.run` (6.5), `cli.ls` (6.4), `cli.prune` (6.7), `hooks.handler` (7.1), `reconcile` (8.1), `tui.sidebar` (9.3), `tui.main_pane` (9.4). All consistent.
- `state.Apply(id, event, lastMessage)` — used in: 3.2 (definition), 7.1 (claude hook), 8.1 (reconciler). All match.
- `tmux.Client` interface methods — implementations satisfy `cli.TmuxClient` and `reconcile.TmuxLs`. All match.
- `events.Entry.At` set both at append time and via Append default. Consistent.
- `paths.Paths.EventsLog(sid)` used in Phase 6 (prune), Phase 7 (hooks), Phase 9 (TUI main pane). All match.

No drift detected. The implementer should keep these signatures stable when extending.

---

## Execution

Plan complete and saved to `cleo/docs/superpowers/plans/2026-05-07-cleo-implementation.md`.

Recommended execution: **subagent-driven** (one task per fresh subagent, two-stage review between tasks). Inline execution with checkpoints is also viable for quick phases.
