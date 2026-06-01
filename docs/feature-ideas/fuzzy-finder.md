basic idea: brainstorm more

- optimization and ui/ux polishing on fuzzy finder

currently search/filter functionality works on a basic approach via `/` keybind. 
it allows user to enter a search term and session get's filtered in the projects tab.

I feel this is okay but very basic and doesn't provide a good ux.
A good UX here can be something like file finder we have with neovim (lazyvim) <space-f> it opens up a new modal(Popup) in which user can find the file, and that becames the center of attraction. similar upon `/` we can open the popup and user can type / filter the session and directly attach to it.

## Cursor stuck on project row during filter

When filter is active and the cursor sits on a project row (`agentIdx == -1`), `clampCursor` does not auto-advance to the first matching session below it. The cursor stays on the project header even when the project only appears because its sessions matched the filter, or when there is exactly one session visible. The user must manually press `j` to move down.

**Root cause**: `clampCursor` (update.go:114) clamps indices to valid ranges but never promotes from `agentIdx == -1` to `agentIdx == 0` when sessions exist.

**Potential fix**: in `clampCursor`, when `m.filter != ""` and the cursor is on a project row that is expanded with sessions, auto-advance `m.cursor.agentIdx` to `0` so the user lands on the first session match.
