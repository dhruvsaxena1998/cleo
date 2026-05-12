# Drop charmbracelet/huh Dependency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `charmbracelet/huh` (used only for two interactive multi-selects) with a plain `y/N` per-agent terminal prompt, drop the dep, and add a stripped release build target to the Makefile.

**Architecture:** Add a `promptYN` helper to `init.go` that reads one line from a caller-supplied `*bufio.Reader`. Both `promptHookSelection` (init) and `promptCleanupSelection` (cleanup) are rewritten to loop over agent options using this helper. The rest of both files is untouched.

**Tech Stack:** Go stdlib only (`bufio`, `fmt`, `io`, `os`, `strings`). No new deps.

---

## File Map

| File | Change |
|------|--------|
| `internal/cli/init.go` | Remove `huh` import; add `promptYN`; replace `promptHookSelection`; update call site |
| `internal/cli/cleanup.go` | Remove `huh` import; replace `promptCleanupSelection`; update call site |
| `internal/cli/init_test.go` | Add `TestPromptYN_*` unit tests |
| `go.mod` / `go.sum` | `go mod tidy` drops `huh` and its exclusive transitives |
| `Makefile` | Add `build-release` target |

---

### Task 1: Add `promptYN` helper + replace `promptHookSelection` in `init.go`

**Files:**
- Modify: `internal/cli/init.go`
- Test: `internal/cli/init_test.go`

- [ ] **Step 1: Write failing tests for `promptYN`**

Add to `internal/cli/init_test.go`:

```go
func TestPromptYN_YesInput(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("y\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true for 'y' input")
	}
}

func TestPromptYN_NoInput(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("n\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false for 'n' input")
	}
}

func TestPromptYN_BlankDefaultYes(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true for blank input when defaultYes=true")
	}
}

func TestPromptYN_BlankDefaultNo(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false for blank input when defaultYes=false")
	}
}

func TestPromptYN_UppercaseYN(t *testing.T) {
	for _, input := range []string{"Y\n", "N\n"} {
		br := bufio.NewReader(strings.NewReader(input))
		var w bytes.Buffer
		got, err := promptYN(&w, br, "opt", true)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.ToLower(strings.TrimSpace(input)) == "y"
		if got != want {
			t.Errorf("input %q: expected %v", input, want)
		}
	}
}

func TestPromptYN_PrintsBracket(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	promptYN(&w, br, "My option", true)
	if !strings.Contains(w.String(), "[Y/n]") {
		t.Errorf("expected [Y/n] in output, got: %s", w.String())
	}

	var w2 bytes.Buffer
	br2 := bufio.NewReader(strings.NewReader("\n"))
	promptYN(&w2, br2, "My option", false)
	if !strings.Contains(w2.String(), "[y/N]") {
		t.Errorf("expected [y/N] in output, got: %s", w2.String())
	}
}
```

Also add `"bufio"` to the test file imports if not present.

- [ ] **Step 2: Run tests — expect compile error since `promptYN` doesn't exist yet**

```bash
go test ./internal/cli/ -run TestPromptYN -v
```

Expected: `undefined: promptYN`

- [ ] **Step 3: Implement `promptYN` in `init.go`**

Replace the entire `promptHookSelection` function and add the helper. The final state of the bottom of `init.go` (replacing the existing `promptHookSelection` function) should be:

```go
func promptYN(w io.Writer, br *bufio.Reader, label string, defaultYes bool) (bool, error) {
	bracket := "[Y/n]"
	if !defaultYes {
		bracket = "[y/N]"
	}
	fmt.Fprintf(w, "  %s %s\n", bracket, label)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y":
		return true, nil
	case "n":
		return false, nil
	default:
		return defaultYes, nil
	}
}

func promptHookSelection(w io.Writer, selected *[]string) error {
	fmt.Fprintln(w, "Which hook systems to install?")
	type hookOpt struct {
		key    string
		label  string
		defYes bool
	}
	opts := []hookOpt{
		{hookClaude, "Claude Code  (~/.claude/settings.json)", true},
		{hookCodex, "Codex        (~/.codex/hooks.json)", true},
		{hookPi, "Pi           (~/.pi/agent/extensions/cleo.ts)", false},
	}
	br := bufio.NewReader(os.Stdin)
	var out []string
	for _, o := range opts {
		yes, err := promptYN(w, br, o.label, o.defYes)
		if err != nil {
			return err
		}
		if yes {
			out = append(out, o.key)
		}
	}
	*selected = out
	return nil
}
```

Update the call site in `RunE` (the `if !yes { ... }` block) from:
```go
if err := promptHookSelection(&selected); err != nil {
```
to:
```go
if err := promptHookSelection(cmd.OutOrStdout(), &selected); err != nil {
```

Update the imports in `init.go` — remove `"github.com/charmbracelet/huh"`, add `"bufio"` if not present. Full import block:

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)
```

- [ ] **Step 4: Run the tests — expect all to pass**

```bash
go test ./internal/cli/ -run TestPromptYN -v
```

Expected: all 6 `TestPromptYN_*` tests PASS.

- [ ] **Step 5: Verify existing init tests still pass**

```bash
go test ./internal/cli/ -run TestPrintInitSummary -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/init.go internal/cli/init_test.go
git commit -m "feat(cli): replace huh multi-select with y/N prompts in init"
```

---

### Task 2: Replace `promptCleanupSelection` in `cleanup.go`

**Files:**
- Modify: `internal/cli/cleanup.go`

- [ ] **Step 1: Replace `promptCleanupSelection` in `cleanup.go`**

Replace the existing `promptCleanupSelection` function with:

```go
func promptCleanupSelection(w io.Writer, selected *[]string) error {
	fmt.Fprintln(w, "Which hook systems to clean up?")
	type hookOpt struct {
		key    string
		label  string
		defYes bool
	}
	opts := []hookOpt{
		{hookClaude, "Claude Code  (~/.claude/settings.json)", true},
		{hookCodex, "Codex        (~/.codex/hooks.json)", true},
	}
	br := bufio.NewReader(os.Stdin)
	var out []string
	for _, o := range opts {
		yes, err := promptYN(w, br, o.label, o.defYes)
		if err != nil {
			return err
		}
		if yes {
			out = append(out, o.key)
		}
	}
	*selected = out
	return nil
}
```

Update the call site in `RunE` from:
```go
if err := promptCleanupSelection(&selected); err != nil {
```
to:
```go
if err := promptCleanupSelection(cmd.OutOrStdout(), &selected); err != nil {
```

Update the imports in `cleanup.go` — remove `"github.com/charmbracelet/huh"`, add `"bufio"`. Full import block:

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)
```

- [ ] **Step 2: Run all CLI tests**

```bash
go test ./internal/cli/ -v
```

Expected: all PASS, no compilation errors.

- [ ] **Step 3: Build to confirm no import errors**

```bash
go build ./...
```

Expected: clean, no output.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/cleanup.go
git commit -m "feat(cli): replace huh multi-select with y/N prompts in cleanup"
```

---

### Task 3: Drop dep, add Makefile target, verify binary size

**Files:**
- Modify: `go.mod`, `go.sum` (via `go mod tidy`)
- Modify: `Makefile`

- [ ] **Step 1: Run `go mod tidy`**

```bash
go mod tidy
```

Expected: lines removed from `go.mod` — at minimum `github.com/charmbracelet/huh`. Possibly also `mitchellh/hashstructure`, `dustin/go-humanize`, and some `clipperhouse/*` packages if they were exclusive to huh.

- [ ] **Step 2: Verify `huh` is gone from `go.mod`**

```bash
grep "charmbracelet/huh" go.mod go.sum
```

Expected: no output.

- [ ] **Step 3: Full build + test after tidy**

```bash
go build ./... && go test ./...
```

Expected: clean build, all tests PASS.

- [ ] **Step 4: Add `build-release` target to Makefile**

The current Makefile:
```makefile
.PHONY: build test lint run clean

build:
	go build -o bin/cleo ./cmd/cleo
```

Update to:
```makefile
.PHONY: build build-release test lint run clean

build:
	go build -o bin/cleo ./cmd/cleo

build-release:
	go build -ldflags="-s -w" -o bin/cleo ./cmd/cleo
```

- [ ] **Step 5: Verify release build size**

```bash
make build-release && ls -lh bin/cleo
```

Expected: binary size noticeably smaller than 8.2 MB (target: ~5–6 MB).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum Makefile
git commit -m "chore: drop charmbracelet/huh dep, add build-release Makefile target"
```
