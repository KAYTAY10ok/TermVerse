# TermVerse

```
█████ █████ ████  █   █ █   █ █████ ████   ████ █████
 ░█░░░█░░░░░█░░░█ ██ ██░█░  █░█░░░░░█░░░█ █ ░░░░█░░░░░
  █░  ████  ████ ░█░█ █░█░  █░████  ████ ░ ███  ████
  █░  █░░░░ █░░█░ █░ ░█░ █ █ ░█░░░░ █░░█░   ░░█ █░░░░
  █░  █████ █░  █ █░  █░  █ ░ █████ █░  █ ████ ░█████
   ░   ░░░░░ ░   ░ ░   ░   ░   ░░░░░ ░   ░ ░░░░  ░░░░░
```

**TermVerse** turns your terminal into a personal, fully customizable
environment: a live system dashboard, developer tools, a terminal arcade
(with an online room to chat and play with friends), network and file
utilities, productivity tools, automation shortcuts and a theme system —
in one fast, single static binary written in Go.

Version **0.1.0** · Linux · MIT license · English / Français

## Features

| # | Section | What you get |
|---|---------|--------------|
| 0 | **Updates & News** | What's new, full changelog, manual GitHub release check (10 s timeout). Never installs anything by itself. |
| 1 | **Developer Tools** | Git inspector, JSON formatter/validator (line:column errors), UUID v4/v7, Base64, hashes (MD5/SHA-1/SHA-256/SHA-512/CRC32 of text or files), timestamp converter, password generator, regex tester, HTTP GET/HEAD client, listening ports. |
| 2 | **Terminal Arcade** | Snake, Sky Hopper, Number Guess, 2048, Mines, Typing Test, and an **Online Room** (LAN/Internet chat + two-player Four in a Row). Local high scores. |
| 3 | **System Center** | Live CPU (total + per core), memory, swap, load, uptime, all disks, sensors, hardware, and a **process manager** (sort, filter, SIGTERM/SIGKILL after confirmation). |
| 4 | **Network Tools** | Interfaces with addresses and traffic, DNS lookup (A/AAAA/CNAME/MX/NS/TXT/PTR), TCP connection test, socket statistics. |
| 5 | **File Manager** | Read-only browser: sizes, permissions, owner, metadata, text preview with line numbers, hex dump for binaries, filter, sort, hidden files. |
| 6 | **Productivity** | Tasks (priorities, filters), quick notes (multi-line editor), Pomodoro timer with statistics. Everything persists. |
| 7 | **Automation Lab** | Saved command shortcuts. Each run shows the exact command and asks for confirmation; stdout/stderr, exit code, duration, timeout and cancel. |
| 8 | **Customization** | Live theme switching, primary/secondary/accent colors, borders on/off and style, separators, selector, ASCII art file, gradient, animations, dashboard widgets, export your palette as a new theme, restore defaults. |
| 9 | **Plugin Hub** | Built-in modules with version and description, enable/disable optional modules, discovery of local plugin manifests (never executed). |
| 10 | **Settings & About** | Language, theme, refresh interval, vim keys, mouse, quit confirmation, Pomodoro and command timeouts, update check, reset, version/license/paths. |

Also: **command palette** (`Ctrl+P`, fuzzy search over every screen, tool,
theme and action), help overlay (`?`), responsive layout down to 40×12,
FR/EN interface (auto-detected from `$LANG`), real system data only (`N/A`
when something cannot be read).

## Installation

### Quick install (from a GitHub release)

```bash
curl -fsSL https://raw.githubusercontent.com/KAYTAY10ok/TermVerse/main/scripts/install.sh | bash
```

The script detects the architecture (amd64/arm64), downloads the release
archive, **verifies its SHA-256 checksum** and installs to `~/.local/bin`
(no `sudo`). Set `TERMVERSE_VERSION` or `TERMVERSE_BIN_DIR` to override.
It requires a published release (create one by pushing a `v*` tag — see
below).

### Debian / Ubuntu package

Download `termverse_VERSION_amd64.deb` from the releases page, or build it
locally with `make deb`, then:

```bash
sudo apt install ./termverse_0.1.0_amd64.deb
```

> An APT repository (`sudo apt install termverse`) is **not available yet**.
> It is planned: releases would be signed and published to a repository
> (e.g. GitHub Pages + `reprepro`/`aptly`), with a key installed under
> `/etc/apt/keyrings`.

### From source

Requires Go 1.24+.

```bash
go mod download
go build -o termverse ./cmd/termverse
./termverse
```

or with make: `make build`, `make run`, `make test`, `make install`
(to `~/.local/bin`, override with `PREFIX=`), `make clean`, `make deb`.

## Usage

```
termverse                      Start the interactive interface
termverse --help               Show help
termverse --version            Show version information
termverse config path          Print configuration and data paths
termverse config reset [--yes] Reset config.json to defaults (asks first)
termverse theme list           List available themes
termverse theme set matrix     Select a theme
termverse system               Print a system information summary
```

### Keys

| Key | Action |
|-----|--------|
| `↑` `↓` (`j` `k`) | Move |
| `Enter` | Open / confirm |
| `0`–`9` | Open a section directly (dashboard) — `[10]` via arrows + Enter |
| `Esc` | Back |
| `Ctrl+P` | Command palette |
| `?` | Shortcuts of the current screen |
| `q` | Quit (dashboard) · `Ctrl+C` quits anywhere |

## Configuration

TermVerse follows the XDG Base Directory spec (`$XDG_CONFIG_HOME`,
`$XDG_DATA_HOME`). Everything is created on first launch; existing files
are never overwritten.

```
~/.config/termverse/
├── config.json         preferences (written atomically)
├── themes/             cyberpunk, matrix, dracula, nord, minimal + your own
├── ascii/default.txt   dashboard ASCII art
└── plugins/            local plugin manifests (listed, never executed)

~/.local/share/termverse/
├── tasks.json  notes.json  pomodoro.json
├── scores.json automation.json  state.json
```

An invalid JSON file is kept as `<name>.invalid-<date>` and defaults are
used, so no data is lost.

### Custom themes

Create `~/.config/termverse/themes/ocean.json` — missing colors fall back to
the default theme:

```json
{
  "name": "ocean",
  "description": "Deep blue",
  "colors": {
    "primary": "#0077BE", "secondary": "#00B4D8", "accent": "#FFB703",
    "text": "#E0F7FA", "muted": "#5C7C8A", "border": "#0096C7",
    "highlight": "#023E58", "highlight_text": "#FFFFFF",
    "success": "#06D6A0", "warning": "#FFD166", "error": "#EF476F"
  }
}
```

Then `termverse theme set ocean` or pick it in Customization. You can also
export the current palette from Customization → *Export as custom theme*.

### Custom ASCII art

Put any text file in `~/.config/termverse/ascii/` and select it in
Customization, or give an absolute path.

## Online Room

Arcade → Online Room → **Host a room** opens a TCP port (default 7777) only
when you choose it, shows the addresses others can use, and lets you set a
password. Friends choose **Join a room** with `your-ip:7777`. Over the
Internet the host must forward the port. Limits: 8 players, 4 KiB messages,
rate limiting, control characters stripped. Traffic is **not encrypted**.

## Architecture

```
cmd/termverse/        entry point (CLI dispatch → TUI)
embed.go              embedded CHANGELOG, themes, ASCII art
internal/
  app/                root Bubble Tea model, navigation, palette, overlays
  dashboard/          home screen layout
  ui/                 shared context, styles, widgets, lists, forms, i18n
  config/             XDG paths, config schema, validation, bootstrap
  theme/              theme parsing, validation, registry
  system/             real system metrics (gopsutil) and processes
  netinfo/            network diagnostics
  update/             GitHub release check and semver compare
  registry/           module registry, plugin manifests, future plugin API
  cli/                non-interactive commands
  fsutil/             atomic writes, safe JSON helpers
modules/              one package per section (updates, devtools, arcade,
                      arcade/online, systemcenter, network, filemanager,
                      productivity, automation, customization, plugins, settings)
scripts/install.sh    release installer with SHA-256 verification
packaging/deb/        .deb builder
.github/workflows/    CI (fmt, vet, race tests, builds, shellcheck) and releases
```

Stack: [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss),
[Bubbles](https://github.com/charmbracelet/bubbles),
[gopsutil](https://github.com/shirou/gopsutil).

## Releasing

```bash
git tag v0.1.0
git push origin v0.1.0
```

The Release workflow runs the tests and GoReleaser, which publishes
`linux_amd64`/`linux_arm64` archives, `.deb` packages and `checksums.txt`.

## Roadmap

- Community plugins (out-of-process, JSON-RPC, explicit permissions — see `internal/registry/api.go`)
- Docker panel and API collections in Developer Tools
- Pong, Tetris-style and more arcade games; encrypted online room (TLS)
- Optional file operations in the File Manager (copy/rename, always confirmed)
- Signed APT repository, macOS builds
- Official ASCII logo

## Contributing

1. Fork and create a branch.
2. `make lint test` must pass (gofmt, vet, tests).
3. New UI strings: wrap them in `ui.T()` and add the French translation in
   `internal/ui/i18n_fr.go` (`TestFrenchCoverage` checks it).
4. Open a pull request describing the change.

## License

[MIT](LICENSE)
