#!/bin/sh
set -eu

repo="dhruvsaxena1998/cleo"
github="https://github.com/${repo}"
api="https://api.github.com/repos/${repo}/releases"

say() {
  printf '%s\n' "$*"
}

err() {
  printf 'cleo install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

need curl
need tar
need uname
need mktemp

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) err "unsupported OS: $os (supported: macOS, Linux)" ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $arch (supported: amd64, arm64)" ;;
esac

if [ "${CLEO_VERSION:-}" ]; then
  case "$CLEO_VERSION" in
    v*) tag="$CLEO_VERSION" ;;
    *) tag="v$CLEO_VERSION" ;;
  esac
else
  tag="$(
    curl -fsSL \
      -H "Accept: application/vnd.github+json" \
      "$api" |
      sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1
  )"
fi

[ "$tag" ] || err "could not resolve latest release"
version="${tag#v}"

install_dir="${CLEO_INSTALL_DIR:-$HOME/.local/bin}"
archive="cleo_${version}_${os}_${arch}.tar.gz"
base_url="${github}/releases/download/${tag}"
tmp="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

say "Installing cleo ${version} for ${os}/${arch}"

curl -fL "${base_url}/${archive}" -o "${tmp}/${archive}"
curl -fL "${base_url}/checksums.txt" -o "${tmp}/checksums.txt"

if grep " ${archive}\$" "${tmp}/checksums.txt" > "${tmp}/cleo.checksum"; then
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp" && sha256sum -c cleo.checksum >/dev/null)
    say "Checksum verified"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$tmp" && shasum -a 256 -c cleo.checksum >/dev/null)
    say "Checksum verified"
  else
    say "Warning: shasum or sha256sum not found; skipping checksum verification"
  fi
else
  err "checksum entry not found for ${archive}"
fi

tar -xzf "${tmp}/${archive}" -C "$tmp"
[ -f "${tmp}/cleo" ] || err "release archive did not contain cleo"

mkdir -p "$install_dir"
cp "${tmp}/cleo" "${install_dir}/cleo"
chmod 0755 "${install_dir}/cleo"

say "Installed ${install_dir}/cleo"

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) say "Warning: ${install_dir} is not on PATH" ;;
esac

if ! command -v tmux >/dev/null 2>&1; then
  say "Warning: tmux 3.0+ is required at runtime; install it with your package manager"
fi

say "Run: cleo --version"
