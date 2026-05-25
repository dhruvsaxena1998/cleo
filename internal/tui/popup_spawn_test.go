package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
)

// Helper: create a SpawnPopup with sensible defaults for testing.
func newTestSpawnPopup(t *testing.T) SpawnPopup {
	t.Helper()
	return NewSpawnPopup("", nil, "/tmp", []string{"claude", "codex"}, Resolve("catppuccin"))
}

// Helper: create a SpawnPopup with a pre-filled project path.
func newTestSpawnPopupWithProject(t *testing.T, projectPath string) SpawnPopup {
	t.Helper()
	projs := []projects.Project{{ID: "myapp", Path: projectPath}}
	return NewSpawnPopup("myapp", projs, "/tmp", []string{"claude", "codex"}, Resolve("catppuccin"))
}

// ── pathSuggestions ────────────────────────────────────────────────────────

func TestPathSuggestions(t *testing.T) {
	// Create directory structure:
	//   $tmpDir/
	//     alpha/
	//     alpha-beta/
	//     bravo/
	//     notdir (regular file)
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "alpha"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "alpha-beta"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "bravo"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "notdir"), []byte("x"), 0o644)

	tests := []struct {
		name  string
		input string
		want  []string // expected suggestions (unordered)
	}{
		{
			name:  "empty path",
			input: "",
			want:  nil,
		},
		{
			name:  "unique partial match",
			input: filepath.Join(tmpDir, "br"),
			want:  []string{filepath.Join(tmpDir, "bravo") + "/"},
		},
		{
			name:  "exact directory match gets trailing slash suggestion",
			input: filepath.Join(tmpDir, "bravo"),
			want:  []string{filepath.Join(tmpDir, "bravo") + "/"},
		},
		{
			name:  "multiple matches with common prefix",
			input: filepath.Join(tmpDir, "al"),
			want:  []string{filepath.Join(tmpDir, "alpha") + "/", filepath.Join(tmpDir, "alpha-beta") + "/"},
		},
		{
			name:  "no matches",
			input: filepath.Join(tmpDir, "zzz"),
			want:  nil,
		},
		{
			name:  "nonexistent directory parent",
			input: "/no/such/dir/prefix",
			want:  nil,
		},
		{
			name:  "skips regular files",
			input: filepath.Join(tmpDir, "not"),
			want:  nil, // "notdir" is a file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathSuggestions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("pathSuggestions(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			// Check all expected values are present (order may vary)
			gotMap := map[string]bool{}
			for _, s := range got {
				gotMap[s] = true
			}
			for _, w := range tt.want {
				if !gotMap[w] {
					t.Errorf("pathSuggestions(%q): missing expected suggestion %q", tt.input, w)
				}
			}
		})
	}
}

func TestPathSuggestions_TrailingSlash(t *testing.T) {
	parent := t.TempDir()
	os.MkdirAll(filepath.Join(parent, "child"), 0o755)

	// Trailing slash with a single child directory
	got := pathSuggestions(parent + string(filepath.Separator))
	if len(got) != 1 || got[0] != parent+string(filepath.Separator)+"child/" {
		t.Errorf("got %v, want [%q]", got, parent+string(filepath.Separator)+"child/")
	}
}

// ── Tab cycling ────────────────────────────────────────────────────────────

func TestSpawnPopupTabCyclesForwardThroughFocusZones(t *testing.T) {
	popup := newTestSpawnPopup(t)
	// focusIndex: 0=path, 1=label, 2=agents
	if popup.focusIndex != 0 {
		t.Fatalf("initial focusIndex: want 0 (path), got %d", popup.focusIndex)
	}

	// Tab: path → label
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 1 {
		t.Fatalf("after 1st tab: want focusIndex 1 (label), got %d", popup.focusIndex)
	}

	// Tab: label → agents
	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 2 {
		t.Fatalf("after 2nd tab: want focusIndex 2 (agents), got %d", popup.focusIndex)
	}

	// Tab: agents → path (wrap)
	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 0 {
		t.Fatalf("after 3rd tab: want focusIndex 0 (path), got %d", popup.focusIndex)
	}
}

func TestSpawnPopupShiftTabCyclesBackward(t *testing.T) {
	popup := newTestSpawnPopup(t)
	if popup.focusIndex != 0 {
		t.Fatalf("initial focusIndex: want 0, got %d", popup.focusIndex)
	}

	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 2 {
		t.Fatalf("after shift+tab from path: want focusIndex 2 (agents), got %d", popup.focusIndex)
	}

	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 1 {
		t.Fatalf("after shift+tab from agents: want focusIndex 1 (label), got %d", popup.focusIndex)
	}

	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	popup = model.(SpawnPopup)
	if popup.focusIndex != 0 {
		t.Fatalf("after shift+tab from label: want focusIndex 0 (path), got %d", popup.focusIndex)
	}
}

// ── Path validation on submit ──────────────────────────────────────────────

func TestSpawnPopupValidationEmptyPath(t *testing.T) {
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue("")
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError != "path is required" {
		t.Fatalf("empty path error: want %q, got %q", "path is required", sp.pathError)
	}
}

func TestSpawnPopupValidationNonExistentPath(t *testing.T) {
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue("/no/such/directory/ever")
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError != "directory not found" {
		t.Fatalf("non-existent path error: want %q, got %q", "directory not found", sp.pathError)
	}
}

func TestSpawnPopupValidationNotADirectory(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(tmpFile)
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError != "not a directory" {
		t.Fatalf("non-directory path error: want %q, got %q", "not a directory", sp.pathError)
	}
}

func TestSpawnPopupValidationValidNewDirectory(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(dir)
	model, cmd := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError != "" {
		t.Fatalf("valid new directory should not have error, got %q", sp.pathError)
	}
	msg := cmd()
	submitted, ok := msg.(SpawnSubmitted)
	if !ok {
		t.Fatalf("expected SpawnSubmitted, got %T", msg)
	}
	if submitted.Path != dir {
		t.Fatalf("submitted Path: want %q, got %q", dir, submitted.Path)
	}
	if submitted.ProjectID != "" {
		t.Fatalf("submitted ProjectID should be empty for new directory, got %q", submitted.ProjectID)
	}
}

func TestSpawnPopupValidationExistingProjectPath(t *testing.T) {
	dir := t.TempDir()
	projs := []projects.Project{{ID: "myapp", Path: dir}}
	popup := NewSpawnPopup("", projs, "/tmp", []string{"claude", "codex"}, Resolve("catppuccin"))
	popup.pathInput.SetValue(dir)
	model, cmd := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError != "" {
		t.Fatalf("valid existing project should not have error, got %q", sp.pathError)
	}
	msg := cmd()
	submitted, ok := msg.(SpawnSubmitted)
	if !ok {
		t.Fatalf("expected SpawnSubmitted, got %T", msg)
	}
	if submitted.ProjectID != "myapp" {
		t.Fatalf("submitted ProjectID: want %q, got %q", "myapp", submitted.ProjectID)
	}
	if submitted.Path != dir {
		t.Fatalf("submitted Path: want %q, got %q", dir, submitted.Path)
	}
}

func TestSpawnPopupPathErrorClearsOnInput(t *testing.T) {
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue("")
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	if sp.pathError == "" {
		t.Fatal("expected pathError after empty-submit, got none")
	}
	popup = sp
	popup.focusIndex = 0
	popup.pathInput.Focus()
	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	sp = model.(SpawnPopup)
	if sp.pathError != "" {
		t.Fatalf("pathError should clear on input, got %q", sp.pathError)
	}
}

// ── Default values ───────────────────────────────────────────────────────────

func TestSpawnPopupWithProjectDefaultsFocusOnLabel(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopupWithProject(t, dir)
	if popup.focusIndex != 1 {
		t.Fatalf("with project, focus should start on label (1), got %d", popup.focusIndex)
	}
	if popup.pathInput.Value() != dir {
		t.Fatalf("path should be prefilled with project path %q, got %q", dir, popup.pathInput.Value())
	}
}

func TestSpawnPopupWithoutProjectDefaultsFocusOnPath(t *testing.T) {
	popup := NewSpawnPopup("", nil, "/custom/cwd", []string{"claude", "codex"}, Resolve("catppuccin"))
	if popup.focusIndex != 0 {
		t.Fatalf("without project, focus should start on path (0), got %d", popup.focusIndex)
	}
	if popup.pathInput.Value() != "/custom/cwd" {
		t.Fatalf("path should default to CWD, got %q", popup.pathInput.Value())
	}
}

// ── Agent selection ────────────────────────────────────────────────────────

func TestSpawnPopupDefaultAgentFromProject(t *testing.T) {
	dir := t.TempDir()
	projs := []projects.Project{{ID: "myapp", Path: dir, DefaultAgent: "codex"}}
	popup := NewSpawnPopup("myapp", projs, "/tmp", []string{"claude", "codex"}, Resolve("catppuccin"))
	if popup.cursor != 1 {
		t.Fatalf("cursor should be on codex (index 1), got %d", popup.cursor)
	}
}

func TestSpawnPopupDefaultAgentFallsBackToFirst(t *testing.T) {
	popup := NewSpawnPopup("", nil, "/tmp", []string{"claude", "codex"}, Resolve("catppuccin"))
	if popup.cursor != 0 {
		t.Fatalf("cursor should default to first agent (0), got %d", popup.cursor)
	}
}

// ── Enter emits SpawnSubmitted ─────────────────────────────────────────────

func TestSpawnPopupEnterEmitsSubmitWithValidData(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopupWithProject(t, dir)
	_, cmd := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	submitted, ok := msg.(SpawnSubmitted)
	if !ok {
		t.Fatalf("expected SpawnSubmitted, got %T", msg)
	}
	if submitted.Agent != "claude" {
		t.Fatalf("submitted Agent: want %q, got %q", "claude", submitted.Agent)
	}
	if submitted.Name != "" {
		t.Fatalf("submitted Name should be empty when input is blank, got %q", submitted.Name)
	}
}

// ── Integration tests: openSpawnPopup ──────────────────────────────────────

func TestOpenSpawnPopupWithNoProject(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = nil

	m2, _ := m.openSpawnPopup()
	if m2.mode != ModePopup {
		t.Fatalf("expected ModePopup, got %v", m2.mode)
	}
	sp, ok := m2.popup.(SpawnPopup)
	if !ok {
		t.Fatalf("expected SpawnPopup, got %T", m2.popup)
	}
	if sp.focusIndex != 0 {
		t.Fatalf("expected focusIndex 0 (path), got %d", sp.focusIndex)
	}
}

func TestOpenSpawnPopupWithProjectAtCursor(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0

	m2, _ := m.openSpawnPopup()
	if m2.mode != ModePopup {
		t.Fatalf("expected ModePopup, got %v", m2.mode)
	}
	sp, ok := m2.popup.(SpawnPopup)
	if !ok {
		t.Fatalf("expected SpawnPopup, got %T", m2.popup)
	}
	if sp.focusIndex != 1 {
		t.Fatalf("expected focusIndex 1 (label), got %d", sp.focusIndex)
	}
	if sp.pathInput.Value() != target {
		t.Fatalf("expected path %q, got %q", target, sp.pathInput.Value())
	}
}

// ── Integration tests: performSpawn ──────────────────────────────────────────

func TestPerformSpawnWithNewProjectRegistersIt(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "newproj")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(c)
	m.projects, _ = c.Projects.List()

	m2, cmd := m.performSpawn(SpawnSubmitted{
		ProjectID: "",
		Path:      target,
		Agent:     "claude",
		Name:      "test-sess",
	})

	projs, err := c.Projects.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(projs) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projs))
	}
	if projs[0].Path != target {
		t.Fatalf("expected project path %q, got %q", target, projs[0].Path)
	}
	if m2.mode != ModeNormal {
		t.Fatalf("expected ModeNormal, got %v", m2.mode)
	}
	if m2.popup != nil {
		t.Fatalf("expected popup to be nil")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (loadStateCmd)")
	}
}

func TestPerformSpawnWithExistingProjectDoesNotReregister(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	registered, err := c.Projects.Add(target)
	if err != nil {
		t.Fatal(err)
	}

	m := New(c)
	m.projects, _ = c.Projects.List()

	m2, cmd := m.performSpawn(SpawnSubmitted{
		ProjectID: registered.ID,
		Path:      target,
		Agent:     "claude",
		Name:      "test-sess",
	})

	projs, _ := c.Projects.List()
	if len(projs) != 1 {
		t.Fatalf("expected 1 project (no duplicate), got %d", len(projs))
	}
	if m2.mode != ModeNormal {
		t.Fatalf("expected ModeNormal, got %v", m2.mode)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

// ── View rendering ──────────────────────────────────────────────────────────

func TestSpawnPopupViewShowsPathField(t *testing.T) {
	popup := newTestSpawnPopup(t)
	view := popup.View()
	if !strings.Contains(view, "1. path") {
		t.Fatal("view should contain '1. path'")
	}
	if !strings.Contains(view, "2. label") {
		t.Fatal("view should contain '2. label'")
	}
	if !strings.Contains(view, "3. ai-agent") {
		t.Fatal("view should contain '3. ai-agent'")
	}
}

func TestSpawnPopupViewShowsPathError(t *testing.T) {
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue("")
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := model.(SpawnPopup)
	view := sp.View()
	if !strings.Contains(view, "path is required") {
		t.Fatal("view should contain 'path is required' error")
	}
}

func TestSpawnPopupViewShowsNewProjectPreview(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(dir)
	view := popup.View()
	if !strings.Contains(view, "will register project, then create session") {
		t.Fatal("view should show registration preview for new project")
	}
}

func TestSpawnPopupViewShowsExistingProjectPreview(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopupWithProject(t, dir)
	view := popup.View()
	if !strings.Contains(view, "cleo-myapp") {
		t.Fatal("view should contain session ID preview with project ID")
	}
}

func TestSpawnPopupViewShowsFooterHints(t *testing.T) {
	popup := newTestSpawnPopup(t)
	view := popup.View()
	if !strings.Contains(view, "tab") || !strings.Contains(view, "next field") {
		t.Fatal("view should contain tab/next field hint")
	}
	if !strings.Contains(view, "complete path") {
		t.Fatal("view should contain 'complete path' hint for right arrow")
	}
}

// ── Path suggestions via textinput ──────────────────────────────────────────

func TestSpawnPopupSetsSuggestionsOnPathInput(t *testing.T) {
	parent := t.TempDir()
	os.MkdirAll(filepath.Join(parent, "myapp"), 0o755)

	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(filepath.Join(parent, "my"))
	popup.focusIndex = 0
	popup.pathInput.Focus()
	popup.updatePathSuggestions()

	suggestions := popup.pathInput.AvailableSuggestions()
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion after setting path")
	}
	// The suggestion should contain "myapp/"
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "myapp/") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("suggestions should contain 'myapp/', got %v", suggestions)
	}
}

func TestSpawnPopupRightArrowAcceptsSuggestion(t *testing.T) {
	parent := t.TempDir()
	os.MkdirAll(filepath.Join(parent, "myapp"), 0o755)

	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(filepath.Join(parent, "my"))
	popup.focusIndex = 0
	popup.pathInput.Focus()
	popup.pathInput.CursorEnd()
	popup.updatePathSuggestions()

	// The textinput's internal suggestion matching should have found "myapp/"
	if len(popup.pathInput.AvailableSuggestions()) == 0 {
		t.Skip("no suggestions available to accept")
	}

	// Press right arrow → textinput should accept the suggestion
	model, _ := popup.Update(tea.KeyMsg{Type: tea.KeyRight})
	sp := model.(SpawnPopup)

	if !strings.HasSuffix(sp.pathInput.Value(), "myapp/") {
		t.Errorf("after right arrow: path = %q, want suffix 'myapp/'", sp.pathInput.Value())
	}
}