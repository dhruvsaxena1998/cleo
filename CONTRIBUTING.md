# Contributing to cleo

cleo is in alpha (v0.2 at time of writing). Behavior, configuration, and the CLI surface may change between minor releases. Bug reports and small PRs are welcome; for larger changes, open an issue first to discuss the design.

## Filing issues

- **Bug:** use the [bug report form](https://github.com/dhruvsaxena1998/cleo/issues/new?template=bug_report.yml).
- **Idea or feature:** open a regular issue describing the use case.

## Building and testing

```bash
make build         # go build -o bin/cleo ./cmd/cleo
make test          # go test ./...
make lint          # go vet ./...
make run           # build and launch ./bin/cleo
./scripts/smoke.sh # end-to-end manual smoke; requires claude CLI and tmux
```

## Test policy

- Every new feature ships with at least one test.
- Every bug fix ships with a regression test that fails on the broken code and passes on the fix.
- TUI rendering and tmux interactions that resist unit testing are covered by manual rituals (below).

## Commit format

Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `ci:`, `test:`. Subject under 70 characters. Body explains the *why* if non-obvious.

## Pull requests

- Keep PRs small and focused (≤ ~400 lines diff is a soft target).
- Describe the *why* in the PR body: what problem, what trade-offs.
- Add a one-line entry under `## [Unreleased]` in `CHANGELOG.md` in the same commit as the code change.
- The CI workflow runs `go vet` and `go test ./...` on every PR. Do not merge while CI is red.

## Manual verification rituals

Some behaviors are not fully covered by unit tests. Run these by hand before approving PRs that touch the relevant area.

### Pane preview correctness — `internal/tui/`, `internal/tmux/CapturePane`

Open `cleo` against a project with at least one running claude session and one running codex session. Navigate up/down between them rapidly (10+ alternations). The preview must not get stuck on either session's last frame; the visual content for the currently-selected row must update within `pane_preview_interval`. Resize the terminal to roughly half-width — panel borders must stay aligned across all rows.

### Reconciler timing — `internal/reconcile/`, `internal/state/`

Set `idle_to_completed_timeout = 30s` and `spawning_timeout = 10s` in test config. Start a session, immediately fire `Notification`, observe the TUI state column for ~70s. Expected progression: `Spawning → Running → WaitingForInput → Idle → Completed`.

## License

cleo is MIT-licensed. By contributing, you agree your contributions will be MIT-licensed too.
