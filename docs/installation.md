# Installation

How to install, upgrade, and uninstall Cleo. See [Quick Start](quickstart.md) once it's installed.

## Requirements

- Go `1.25.5` (only required if building from source; prebuilt releases are available via the curl installer, Homebrew, or the Releases page).
- `tmux` `3.0+` at runtime.
- The agent CLIs you want to use, such as `claude`, `codex`, `opencode`, or `pi`.
- Sound playback uses `afplay` on macOS and the first of `paplay`, `aplay`, or `play` available on Linux. Windows is not supported.

## Install

**Curl installer (recommended):**

```bash
curl -fsSL https://github.com/dhruvsaxena1998/cleo/raw/main/scripts/install.sh | sh
```

Installs the latest GitHub Release to `~/.local/bin`. Set `CLEO_INSTALL_DIR` or `CLEO_VERSION` to override the install directory or pin a version.

**Homebrew:**

```bash
brew tap dhruvsaxena1998/tap
brew install cleo
```

**Go install:**

```bash
go install github.com/dhruvsaxena1998/cleo/cmd/cleo@latest
```

For local development from this repository:

```bash
make build
./bin/cleo --version
```

## Verify the install

```bash
cleo --version
```

This should print the installed version (for example, `v0.2.0`). If `cleo --version` prints anything else, see [Troubleshooting](troubleshooting.md).

## Upgrade

```bash
brew update && brew upgrade cleo
```

For curl installs, re-run the installer:

```bash
curl -fsSL https://github.com/dhruvsaxena1998/cleo/raw/main/scripts/install.sh | sh
```

## Uninstall

For curl installs:

```bash
rm ~/.local/bin/cleo
```

For Homebrew installs:

```bash
brew uninstall cleo
brew untap dhruvsaxena1998/tap
```
