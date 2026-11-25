# Changelog

All notable changes to TermVerse are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/) and the
project uses [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-11-23

### Added
- Full-screen interactive TUI dashboard with live system information.
- Main menu with 11 sections (0–10), keyboard navigation and shortcuts.
- Five built-in themes (Cyberpunk, Matrix, Dracula, Nord, Minimal) and
  user-defined JSON themes.
- Live customization: colors, borders, separators, selector, ASCII art,
  widgets, animations — saved to `~/.config/termverse/config.json`.
- Updates & News: local changelog and manual GitHub release check.
- Developer Tools: Git repository info, JSON formatter/validator, UUID
  generator, Base64 encoder/decoder, listening ports.
- Terminal Arcade: Snake and Number Guess with persistent high scores.
- System Center: live CPU (per core), memory, swap, disks, load, top processes.
- Network Tools: interfaces, DNS lookup, TCP connection test, connection stats.
- File Manager: read-only browser with sizes, permissions and metadata.
- Productivity: tasks, quick notes and a Pomodoro timer with local persistence.
- Automation Lab: saved command shortcuts executed only after confirmation.
- Plugin Hub: module registry, enable/disable optional modules, local plugin
  manifest discovery (no code execution).
- Settings & About screen.
- CLI: `--help`, `--version`, `config path|reset`, `theme list|set`, `system`.
- Makefile, install script with SHA-256 verification, Debian packaging,
  GoReleaser config and GitHub Actions CI/release workflows.
