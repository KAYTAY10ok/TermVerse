#!/usr/bin/env bash
# TermVerse installer: downloads a published release, verifies its SHA-256
# checksum and installs the binary. No root needed by default.
#
#   curl -fsSL https://raw.githubusercontent.com/KAYTAY10ok/TermVerse/main/scripts/install.sh | bash
#
# Environment variables:
#   TERMVERSE_VERSION   version to install (default: latest release)
#   TERMVERSE_BIN_DIR   install directory (default: ~/.local/bin)
set -euo pipefail

REPO="KAYTAY10ok/TermVerse"
BIN_DIR="${TERMVERSE_BIN_DIR:-$HOME/.local/bin}"
VERSION="${TERMVERSE_VERSION:-}"

info() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but not installed"; }

need curl
need tar
need uname

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
[ "$os" = "linux" ] || fail "unsupported OS '$os' (TermVerse releases target Linux)"
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture '$(uname -m)' (supported: amd64, arm64)" ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  sha() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "sha256sum or shasum is required to verify the download"
fi

if [ -z "$VERSION" ]; then
  info "Looking up the latest release..."
  json="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")" ||
    fail "no published release found for $REPO (or GitHub is unreachable)"
  VERSION="$(printf '%s' "$json" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$VERSION" ] || fail "could not read the latest release version"
fi
VERSION="${VERSION#v}"

archive="termverse_${VERSION}_linux_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v${VERSION}"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT

info "Downloading TermVerse $VERSION ($arch)..."
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "download failed: $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "could not download checksums.txt"

expected="$(grep " ${archive}\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
[ -n "$expected" ] || fail "no checksum listed for $archive"
actual="$(sha "$tmp/$archive")"
[ "$expected" = "$actual" ] || fail "checksum mismatch (expected $expected, got $actual)"
info "SHA-256 verified"

tar -xzf "$tmp/$archive" -C "$tmp" termverse || fail "archive does not contain the termverse binary"
mkdir -p "$BIN_DIR" || fail "cannot create $BIN_DIR"
install -m 0755 "$tmp/termverse" "$BIN_DIR/termverse" || fail "cannot write to $BIN_DIR (set TERMVERSE_BIN_DIR)"
info "Installed $BIN_DIR/termverse"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) printf '\nAdd it to your PATH:\n  echo '\''export PATH="%s:$PATH"'\'' >> ~/.bashrc\n' "$BIN_DIR" ;;
esac
printf '\nRun \033[1mtermverse\033[0m to start.\n'
