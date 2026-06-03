---
status: accepted
---

# Shelve mobile remote TUI access (PRD #91) — baseline for a future attempt

We set out (PRD GitHub issue #91) to make the Cleo dashboard usable from a phone over
SSH + tmux + a Termius client — no mosh, no daemon, no extra listening port, keeping
Cleo's local-first "never a service process" stance. Two features were built: a
**compact dashboard** that collapses to a single column on phone-width terminals, and
**`cleo remote setup`**, which registers a phone's SSH key in `~/.ssh/authorized_keys`
behind a `restrict,pty,command="…cleo"` forced command so connecting drops straight
into the dashboard.

After real-device testing, **we are shelving the whole effort.** The end-to-end mobile
TUI experience is not usable, and the one piece that was clearly correct (tmux discovery
over SSH) does not justify carrying the rest. This ADR records what was built, why it
did not work, and enough detail — including the un-merged tmux fix in full — to restart
from a known baseline rather than from scratch.

## What was built (and its disposition)

| Piece | Where | Disposition |
| --- | --- | --- |
| Compact dashboard (`decideLayout`, width-aware render) | merged to `main` as `3721891` (PR #95, from `feat/compact-dashboard-92`); issue #92 | **Reverted** by this PR (revert commit on `chore/shelve-mobile-remote-tui`) |
| `cleo remote setup` (authorized_keys module + `--print` + idempotent write) | PR **#96**, branch `feat/remote-setup-print-93` (tip `422c35d`, includes the Enter-key paste fix); issues #93/#94 | **PR closed**, branch left on origin for recovery |
| attach-no-dead correctness fix | PR **#97**, branch `fix/attach-no-dead-on-tmux-error` (`3ea8def`) | **PR closed**, branch left on origin for recovery |
| tmux discovery fallback (SSH PATH fix) | local-only `b6225e3` on `fix/tmux-discovery-fallback`, **never pushed** | **Discarded** — preserved verbatim below |

`feat/remote-setup-print-93` and `fix/attach-no-dead-on-tmux-error` remain pushed on
origin, so their code is recoverable even with the PRs closed. The tmux fix existed only
locally, so its full diff is embedded in this document.

## Why we are shelving it

**The compact layout never engaged on the actual phone.** `decideLayout` switched to the
single-column mode below a fixed `width < 60` columns, but Termius reports **≥60 columns**
(it commonly defaults to ~80). So on the target device the dashboard rendered the full
multi-panel layout — sidebar + events + tmux preview + metadata grid — crammed into a
phone screen, which is broken at most interaction points. The compact mode we built was
effectively dead code on the one client it was for. A fixed threshold was the wrong
mechanism; the real signal (does this client have a usably narrow viewport) was never
measured against a real device before building.

**The forced-command transport surfaced a cascade of environment problems**, only some of
which were fixed:

1. *cleo not on PATH over the forced command* — fixed in PR #96 by writing an **absolute**
   binary path into the managed line (a non-login `$SHELL -c cleo` shell never sources the
   rc file that adds Homebrew to PATH).
2. *tmux not on PATH* — the same gap one level down: cleo then called `tmux` by bare name.
   Fixed locally (the discarded fix below), and verified working over SSH.
3. *the agent binary (e.g. `claude`) not on PATH inside a session created over SSH* — never
   addressed. A session spawned through the stripped-PATH forced command inherits that
   environment, so new-session creation can fail to find the agent. This is the next layer
   of the same onion and signals that bolting onto the non-login forced-command environment
   is a leaky foundation.

Each fix revealed another instance of the same root problem (a deliberately minimal,
non-login environment), and the payoff at the end — the compact UI — did not actually work
on the device. The combination is what tips this from "keep iterating" to "shelve and
rethink the transport/UX as a unit."

## The discarded tmux discovery fix (verbatim, for reuse)

This change was correct and verified (it found Homebrew's tmux over the SSH forced command),
but with the mobile effort shelved there is no in-tree consumer that needs it, so it is not
being merged. It is general-purpose — it helps any non-login-shell launch of Cleo (cron,
`ssh host cleo`, GUI launchers) — so a future attempt should lift it directly.

In `internal/tmux/tmux.go`, the `Client` gained a resolved `bin` field and the bare
`exec.Command("tmux", …)` became `exec.Command(c.bin, …)`:

```go
type Client struct {
	socket string
	bin    string // resolved tmux binary; see resolveTmux
}

// resolved once at construction; every command then uses c.bin
func NewClient(socket string) *Client {
	bin, _ := resolveTmux(exec.LookPath, isExecutable, fallbackTmuxPaths)
	return &Client{socket: socket, bin: bin}
}

func Available() bool {
	_, found := resolveTmux(exec.LookPath, isExecutable, fallbackTmuxPaths)
	return found
}

// common absolute install locations probed when "tmux" is not on PATH:
// Homebrew (Apple silicon, then Intel/older), then the system path.
var fallbackTmuxPaths = []string{
	"/opt/homebrew/bin/tmux",
	"/usr/local/bin/tmux",
	"/usr/bin/tmux",
}

// pure decision (table-tested): prefer PATH (bare name so PATH resolution
// applies); else the first candidate that is a runnable executable; else bare
// "tmux" with found=false so callers can report unavailability.
func resolveTmux(lookPath func(string) (string, error), isExec func(string) bool, candidates []string) (path string, found bool) {
	if _, err := lookPath("tmux"); err == nil {
		return "tmux", true
	}
	for _, p := range candidates {
		if isExec(p) {
			return p, true
		}
	}
	return "tmux", false
}

// matches what exec.LookPath guarantees on PATH, so a candidate that exists but
// is a directory or non-executable is skipped, not selected and failed at exec.
// os.Stat follows symlinks (the Homebrew tmux is a symlink).
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	m := info.Mode()
	return !m.IsDir() && m&0o111 != 0
}
```

The attach-no-dead fix (PR #97) addressed a *separate* real bug that is recoverable from
its branch: `PrepareAttach` in `internal/sessionlifecycle/attach.go` marked a session
`dead` on `err != nil || !live`, conflating a transient tmux *query error* with "session
gone". `dead` is terminal (the reconciler never revives it), so a tmux hiccup
irreversibly corrupted shared `~/.config/cleo/state.json`. The fix: only `live==false &&
err==nil` marks dead; a query error is surfaced and the record left intact. **If this
class of state corruption resurfaces on the desktop, recover this fix first** — it is not
mobile-specific.

## Recommendations for a future attempt

- **Validate the viewport signal on a real device before building UI.** Measure what the
  intended client (Termius/iSH/Blink) actually reports for `width`/`height`/`$TERM` and
  whether it changes on rotation, then design the breakpoint around that — not a guessed
  `< 60`. Consider a config-driven threshold or an explicit "compact" mode rather than
  inferring from columns.
- **Don't build onto the non-login forced-command environment.** It strips PATH and
  sources only `~/.zshenv`, which leaked into cleo-discovery, tmux-discovery, and
  agent-discovery in turn. A login-shell forced command (`$SHELL -lc 'exec …'`) sources
  the rc and would have closed all three at once — at the cost of shell-quoting. Evaluate
  that trade deliberately up front instead of patching bare-name lookups one binary at a
  time. (See also ADR 0001 on the tmux seam.)
- **Reuse the pieces that were sound**: the pure `resolveTmux`/`isExecutable` discovery
  above, the attach-no-dead correctness fix (PR #97 branch), and the security-reviewed
  `authorized_keys` merge module (PR #96 branch — line/newline-injection and
  forced-command escaping were reviewed clean).

## Consequences

- The compact dashboard is removed from `main` (`internal/tui/layout.go` and its test
  deleted; `view.go`/`main_pane.go`/`sidebar.go` restored to their pre-#95 form).
- PRs #96 and #97 are closed unmerged; their branches stay on origin as the recovery
  point for `cleo remote setup` and the attach-no-dead fix.
- PRD #91 is not delivered. The decided transport (SSH + tmux + Termius; no
  mosh/daemon/port) still stands as the direction *if* mobile is revisited.
- No user-facing surface changes beyond reverting the (non-functional-on-mobile) compact
  layout; desktop behavior is unaffected.
