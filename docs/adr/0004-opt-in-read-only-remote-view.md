---
status: accepted
---

# cleo gains an opt-in, read-only HTTP remote view (`cleo serve`)

cleo will gain a `cleo serve` command that starts an opt-in, foreground HTTP
server exposing a read-only **remote view**: a web page, reached over the LAN by
scanning a QR code, that lists every Session with its Project, agent, name,
state, and age. It is read-only — it never attaches to or sends input to a
Session — and it is off by default; nothing runs until the user starts
`cleo serve`. The server binds `0.0.0.0:<port>` and gates access with a
per-run token carried in the QR'd URL.

## Why

The motivating need: when the user steps away from the machine but keeps their
phone, they need to know which agent Session needs attention (e.g.
`waiting_for_input`) without being at the keyboard. Walking the cheap paths
first established the real gap:

- SSH + a phone terminal into the existing TUI works as transport, but the
  Dashboard is unusable at phone width; only the in-Session agent prompt is
  usable. So "attach and type" already works; "see which Session needs me" does
  not.
- mosh (free, open source) solves resilient transport and attach. Third-party
  apps (Moshi) add a tmux session list and push notifications but (a) cost money
  and meter usage (~20 tmux connections/month) and (b) list raw *tmux* sessions,
  so they cannot show cleo's lifecycle **state** — the one thing that answers
  "which Session needs me."

The unmet gap is therefore *Session-state visibility from a phone*, which only
cleo can provide because the state lives in cleo, not tmux. A read-only web page
reusing the existing `state.List()` + reconciler is the smallest thing that
fills it, works in any browser (no paid app, no per-device install), and stays
free and open source. The data is already sufficient: `WaitingForInput` is a
first-class state (`internal/state/session.go`), so the page can sort the
Sessions that need you to the top without any state-machine change.

## Considered and rejected

- **An interactive web UI (send keys / web terminal).** Sending input via
  `SendKeys` is remote code execution — the agent runs shell — so an exposed
  endpoint without strong auth hands the machine to anyone on the LAN. A web
  terminal would also re-implement, worse, what free mosh/SSH already do well.
  Deferred behind its own security-focused design pass; v1 is read-only.
- **Relying on a third-party app (Moshi).** Paid, metered, closed source, and
  blind to cleo state. Fails both the free-and-open-source constraint and the
  core need.
- **HTTPS / Tailscale / SSH-tunnel-only for v1.** Safer channels, but
  self-signed certs break the scan-and-open QR flow, and Tailscale adds a
  third-party dependency. Chosen instead: plain HTTP on the LAN gated by a
  per-run token, accepting cleartext-on-a-trusted-LAN. Read-only + token caps
  the blast radius at information disclosure, never RCE. These remain available
  as later hardening.
- **An always-on daemon (like Moshi's `moshi-hook` brew service).** Would
  contradict cleo's local-first promise that it "does not require a service
  process." `cleo serve` is opt-in and foreground, so the promise holds: nothing
  runs unless the user starts it.

## Consequences

- cleo acquires its first network surface. The README/CONTEXT "no service
  process" promise is preserved only so long as the server stays opt-in and off
  by default — this constraint must not erode.
- The remote view is a read-only consumer of durable Session state; it must not
  gain attach/send without a separate ADR addressing the RCE surface and auth.
- Exposed data is deliberately limited to Project / agent / name / state / age —
  no Session IDs, no `LastMessage`, no pane previews (which can leak secrets).
  Widening what the remote view exposes is a security decision, not a cosmetic
  one.
- The `EvNotification` → `waiting_for_input` transition is the natural hook point
  for a future "push notification when a Session needs you," which remains out of
  scope here.
