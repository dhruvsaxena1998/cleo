# Session Lifecycle Creation Tracer-Bullet Implementation Plan

> **For agentic workers:** implement with a red-green-refactor loop. Keep the first slice small: move Session creation through the new Session lifecycle module, then stop. Do not opportunistically migrate kill/prune/rename/attach in the same diff.

**Goal:** Introduce a deep Session lifecycle module and route both CLI and TUI Session creation through it without changing user-facing behaviour.

**Architecture:** New `internal/sessionlifecycle` package owns Project resolution/registration mechanics, Session naming, state-first Session creation, tmux launch, rollback, focus hook installation, and detach-key binding. CLI and TUI keep prompts, popups, status text, attach choice, and rendering.

**Tech Stack:** Go, Cobra CLI, Bubble Tea TUI, file-backed Project/Session stores, tmux adapter.

---

## Background

Session creation is currently implemented twice: once in the CLI run command and once in the TUI spawn flow. Both callers know too much:

- how to resolve or register a Project
- how to compute existing Session names
- how to slug/dedupe a Session name
- how to build a Session ID
- how to write the initial `spawning` Session record
- how to launch tmux with `CLEO_SESSION_ID`
- how to roll back state if tmux launch fails
- how to install focus hooks
- how to bind the configured detach key

The deletion test says this logic is earning its keep: deleting either copy does not make complexity vanish; it reappears in the other caller. Concentrate it behind a Session lifecycle seam.

The project glossary now defines **Session lifecycle** as creation, attachment, revival, termination, pruning, and renaming for a Session. This plan implements the creation tracer-bullet only.

---

## Files

- Add: `internal/sessionlifecycle/*`
- Modify: `internal/cli/run.go`
- Modify: `internal/tui/handle_key.go`
- Modify as needed: tmux adapter/interface definitions for detach-key binding and focus hook installation
- Modify/add tests under the affected packages

---

## Design constraints

- Preserve current user-facing behaviour.
- Preserve state-first creation.
- Preserve rollback on tmux launch failure.
- Keep CLI prompts out of Session lifecycle.
- Keep TUI popups and status rendering out of Session lifecycle.
- Do not implement or reference Worktree behaviour in code.
- Do not migrate kill/prune/rename/attach yet.
- Prefer typed outcomes over caller-specific error text.
- Keep the only real external seam at tmux; do not introduce store interfaces unless the implementation needs them.

---

## Proposed module shape

Exact names may change during implementation, but preserve this shape:

- A lifecycle object constructed with concrete Project/Session stores, paths/config needed for launch, and a tmux adapter.
- A create input that carries: agent name, optional requested Session name, cwd/path, optional Project ID, and whether Project auto-registration is allowed.
- A create result that carries: created Session, resolved Project, whether a Project was registered, and any non-fatal side-effect warnings.
- A typed outcome/error for: unknown agent, Project not found/registration needed, Project lookup failure, state write failure, tmux launch failure, rollback failure/partial failure.
- A tmux adapter seam that can launch Sessions and expose optional focus-hook/detach-key behaviours in tests.
- Internal injection points for clock and generated Session name when needed by tests.

---

## Task 1: Add failing lifecycle creation tests

**Files:**
- Add lifecycle package tests

- [ ] Test unregistered path with auto-registration disabled returns a needs-registration outcome and writes no Session.
- [ ] Test unregistered path with auto-registration enabled registers the Project and creates the Session.
- [ ] Test existing Project ID creates under that Project without resolving cwd.
- [ ] Test requested Session name is slugified.
- [ ] Test requested Session name is deduped against existing Sessions for the same Project and agent.
- [ ] Test generated Session name is deterministic via test seam and deduped.
- [ ] Test state-first ordering: fake tmux checks that the `spawning` Session exists during launch.
- [ ] Test tmux launch receives Session ID, Project path, agent command, and `CLEO_SESSION_ID`.
- [ ] Test tmux failure removes the Session record.
- [ ] Test focus hook installation is attempted when the tmux adapter supports it.
- [ ] Test detach-key binding is attempted when configured.

Run the new package tests and confirm they fail because the package does not exist yet.

---

## Task 2: Implement the Session lifecycle creation module

**Files:**
- Add lifecycle package implementation

- [ ] Create the package and lifecycle type.
- [ ] Define the create input/result/outcome types.
- [ ] Resolve the agent from config.
- [ ] Resolve Project by explicit Project ID or cwd/path.
- [ ] If Project is missing and auto-registration is false, return needs-registration without writing state.
- [ ] If Project is missing and auto-registration is true, add the Project.
- [ ] Compute existing Session names from the state store for the resolved Project and agent.
- [ ] Slug/dedupe requested name or generate a name.
- [ ] Build the Session ID using existing ID helpers.
- [ ] Write a `spawning` Session with UTC time before tmux launch.
- [ ] Launch tmux using the Project path, agent command, Session ID, and `CLEO_SESSION_ID`.
- [ ] On tmux launch failure, delete the Session record and return the launch failure outcome.
- [ ] Install focus hooks as a best-effort launch side effect.
- [ ] Bind detach key as a best-effort launch side effect.
- [ ] Return created Session and Project data.

Run lifecycle package tests until green.

---

## Task 3: Switch CLI `run` to Session lifecycle creation

**Files:**
- Modify CLI run command
- Modify CLI run tests if needed

- [ ] Keep current argument parsing and flags.
- [ ] Keep the unknown-agent error behaviour or map the lifecycle outcome to equivalent output.
- [ ] For an unregistered cwd, call lifecycle creation once without auto-registration to detect the need for consent, prompt as today, then call with auto-registration if the user accepts.
- [ ] Print `registered project` when a Project was registered.
- [ ] Print `spawned <session-id>` when creation succeeds.
- [ ] Keep `--no-attach` behaviour outside the lifecycle module.
- [ ] Keep attach/switch-client behaviour outside this tracer-bullet.
- [ ] Remove duplicated Session naming/state/tmux/focus/detach code from CLI run.

Run CLI tests.

---

## Task 4: Switch TUI spawn to Session lifecycle creation

**Files:**
- Modify TUI spawn handling
- Modify TUI tests if needed

- [ ] Keep popup input and validation outside lifecycle.
- [ ] For existing Project selection, pass the Project ID into lifecycle creation.
- [ ] For new Project path submitted from the popup, allow auto-registration because the popup submission is the consent point.
- [ ] On lifecycle failure, set TUI status and close/reset popup as current behaviour expects.
- [ ] On success, close popup, return to normal mode, and reload state.
- [ ] Remove duplicated Session naming/state/tmux/focus/detach code from TUI spawn.

Run TUI tests.

---

## Task 5: Clean up duplication and imports

**Files:**
- CLI/TUI helper cleanup
- Tmux adapter/interface cleanup as needed

- [ ] Remove now-unused helper functions for existing Session slug calculation from CLI/TUI call sites.
- [ ] Remove direct focus hook install/detach key binding from creation callers.
- [ ] Ensure lifecycle package does not import CLI or TUI packages.
- [ ] Ensure no import cycle is introduced.
- [ ] Ensure naming uses `Session lifecycle` vocabulary in comments where helpful.

---

## Task 6: Full regression pass

- [ ] Run `go test ./...`.
- [ ] Manually smoke-test `cleo run <agent> --no-attach --yes` if a local configured agent is available.
- [ ] Manually smoke-test TUI spawn if practical.
- [ ] Verify `git diff` contains no Worktree implementation.
- [ ] Verify `CONTEXT.md` still contains the Session lifecycle definition.

---

## Acceptance criteria

- CLI `run` and TUI spawn both use the Session lifecycle creation module.
- User-facing creation behaviour is unchanged.
- Session creation remains state-first.
- tmux launch failure rolls back the Session record.
- Project registration consent remains caller-owned.
- Focus hook installation and detach-key binding are no longer duplicated in creation callers.
- Lifecycle creation has focused tests.
- Existing CLI/TUI tests pass.
- Full test suite passes.
- No Worktree code is introduced.

---

## Deferred follow-up migrations

After this tracer-bullet lands, future issues can move these behaviours into the same Session lifecycle module:

- attach / revive / mark dead
- kill Session
- prune finished Sessions and archive events
- rename Session
- remove Project Sessions and remove event logs
- typed outcomes for the full lifecycle, not just creation
