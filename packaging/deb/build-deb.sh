#!/usr/bin/env bash
# Builds termverse_<version>_<arch>.deb from an already compiled binary.
# Usage: VERSION=0.1.0 ARCH=amd64 packaging/deb/build-deb.sh ./termverse
set -euo pipefail
BIN="${1:-./termverse}"
VERSION="${VERSION:-0.1.0}"
VERSION="${VERSION#v}"
ARCH="${ARCH:-amd64}"
command -v dpkg-deb >/dev/null || { echo "dpkg-deb is required" >&2; exit 1; }
[ -x "$BIN" ] || { echo "binary $BIN not found (run make build)" >&2; exit 1; }

root="$(mktemp -d)"
trap 'rm -rf -- "$root"' EXIT
install -Dm0755 "$BIN" "$root/usr/bin/termverse"
install -Dm0644 LICENSE "$root/usr/share/doc/termverse/copyright"
install -Dm0644 README.md "$root/usr/share/doc/termverse/README.md"
install -Dm0644 CHANGELOG.md "$root/usr/share/doc/termverse/CHANGELOG.md"
mkdir -p "$root/DEBIAN"
cat >"$root/DEBIAN/control" <<CTRL
Package: termverse
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: TermVerse contributors <https://github.com/KAYTAY10ok/TermVerse>
Homepage: https://github.com/KAYTAY10ok/TermVerse
Description: Modern, customizable terminal environment
 TermVerse is an interactive TUI with a live system dashboard, developer
 tools, a terminal arcade, network and file utilities, productivity tools,
 automation shortcuts and a full theme system.
CTRL
out="termverse_${VERSION}_${ARCH}.deb"
dpkg-deb --root-owner-group --build "$root" "$out" >/dev/null
echo "Built $out"
