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
$bin events cleo-tmp-claude-smoke -n 5
$bin doctor
$bin doctor --quiet >/dev/null
$bin kill cleo-tmp-claude-smoke --yes
$bin ls
echo "smoke OK"
