# Drop charmbracelet/huh Dependency Design

**Date:** 2026-05-12  
**Status:** Approved

## Summary

Replace `charmbracelet/huh` (interactive form library) with a plain `y/N` per-agent terminal prompt. `huh` is used only in two tiny functions — `promptHookSelection` and `promptCleanupSelection` — and costs ~500–800 KB in the stripped binary. The replacement is pure `bufio`/`fmt`, zero new deps.

Add a `build-release` Makefile target for locally-stripped builds (goreleaser already strips releases).

## Scope

- Replace `promptHookSelection` in `internal/cli/init.go`
- Replace `promptCleanupSelection` in `internal/cli/cleanup.go`
- Introduce shared `promptYN` helper (in one of the two files, or a small shared file)
- Run `go mod tidy` to drop `huh` and its exclusive transitives from `go.mod`/`go.sum`
- Add `build-release` target to `Makefile`

Out of scope: changes to `--yes` flag behaviour, output formatting, hook installation logic, goreleaser config (already has `-s -w`).

## Prompt UX

```
Which hook systems to install?
  [Y/n] Claude Code  (~/.claude/settings.json)
  [Y/n] Codex        (~/.codex/hooks.json)
  [y/N] Pi           (~/.pi/agent/extensions/cleo.ts)
```

- Capital letter = default (pressing enter accepts it)
- Accepts: blank input (uses default), `y`/`Y` (yes), `n`/`N` (no)
- Any other input re-prompts on the same line (or treats as default — simpler)

## Implementation

### Shared helper

```go
// promptYN prints "[Y/n] label" or "[y/N] label" and reads one line from br.
// defaultYes controls which letter is capitalised and what blank input means.
// br must be created once by the caller and reused across multiple promptYN
// calls — creating a new bufio.Reader per call would consume buffered bytes
// intended for subsequent prompts.
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
```

Place this helper in `internal/cli/init.go` (same package as cleanup). Callers create `bufio.NewReader(os.Stdin)` once and pass it to each `promptYN` call.

### `promptHookSelection` replacement (init.go)

```go
func promptHookSelection(w io.Writer, selected *[]string) error {
    fmt.Fprintln(w, "Which hook systems to install?")
    opts := []struct {
        key    string
        label  string
        defYes bool
    }{
        {hookClaude, "Claude Code  (~/.claude/settings.json)", true},
        {hookCodex,  "Codex        (~/.codex/hooks.json)",     true},
        {hookPi,     "Pi           (~/.pi/agent/extensions/cleo.ts)", false},
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

Update the call site in `RunE` to pass `cmd.OutOrStdout()` only (stdin is wired inside the function).

### `promptCleanupSelection` replacement (cleanup.go)

Same pattern, two options (claude + codex, both default yes), no pi.

### Makefile addition

```makefile
build-release:
	go build -ldflags="-s -w" -o bin/cleo ./cmd/cleo
```

Add to `.PHONY` list.

## go mod tidy

After removing the `huh` import from both files, run `go mod tidy`. This drops:
- `github.com/charmbracelet/huh` direct dep
- Any transitive deps exclusively required by huh (e.g. `mitchellh/hashstructure`, `dustin/go-humanize`, `clipperhouse/*` if not used elsewhere)

Some `charmbracelet/x/*` packages will be retained since bubbletea/lipgloss use them.

## Testing

- `go build ./...` — verifies compile
- `go test ./...` — existing tests pass
- Manual smoke: `cleo init` without `--yes` should print the per-agent prompts and respect input
- `make build-release && ls -lh bin/cleo` — confirm smaller binary
