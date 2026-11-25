// Package cli implements the non-interactive TermVerse commands.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// Env carries what the commands need.
type Env struct {
	Paths  config.Paths
	Themes *theme.Registry
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Assets fs.FS
}

// Usage is the --help text.
const Usage = `TermVerse — a modern, customizable terminal environment.

Usage:
  termverse                      Start the interactive interface
  termverse --help, -h           Show this help
  termverse --version, -v        Show version information
  termverse config path          Print configuration and data paths
  termverse config reset [--yes] Reset config.json to defaults (asks first)
  termverse theme list           List available themes
  termverse theme set <name>     Select a theme
  termverse system               Print a system information summary

Environment:
  XDG_CONFIG_HOME   Base directory of config files (default ~/.config)
  XDG_DATA_HOME     Base directory of user data (default ~/.local/share)
  NO_COLOR          Disable colors

Inside the interface press ? for keyboard shortcuts and Ctrl+P for the
command palette. Documentation: ` + "https://github.com/KAYTAY10ok/TermVerse" + `
`

// ExitUsage is returned for invalid arguments.
const ExitUsage = 2

// Run executes a CLI command. interactive is true when no command was given
// (the caller must then start the TUI).
func Run(args []string, env Env) (code int, interactive bool) {
	if len(args) == 0 {
		return 0, true
	}
	out, errw := env.Stdout, env.Stderr
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(out, Usage)
		return 0, false
	case "-v", "--version", "version":
		fmt.Fprintln(out, version.String())
		return 0, false
	case "config":
		return configCmd(args[1:], env), false
	case "theme":
		return themeCmd(args[1:], env), false
	case "system":
		return systemCmd(env), false
	}
	fmt.Fprintf(errw, "termverse: unknown command %q\nRun 'termverse --help' for usage.\n", args[0])
	return ExitUsage, false
}

func themeExists(r *theme.Registry) func(string) bool {
	return func(n string) bool { _, ok := r.Get(n); return ok }
}

func configCmd(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: termverse config path|reset [--yes]")
		return ExitUsage
	}
	switch args[0] {
	case "path":
		p := env.Paths
		fmt.Fprintf(env.Stdout, "config:  %s\nthemes:  %s\nascii:   %s\nplugins: %s\ndata:    %s\n",
			p.ConfigFile, p.ThemesDir, p.ASCIIDir, p.PluginsDir, p.DataDir)
		return 0
	case "reset":
		yes := len(args) > 1 && (args[1] == "--yes" || args[1] == "-y")
		if !yes {
			fmt.Fprintf(env.Stdout, "Reset %s to defaults? Themes, notes, tasks and scores are kept. [y/N] ", env.Paths.ConfigFile)
			line, _ := bufio.NewReader(env.Stdin).ReadString('\n')
			a := strings.ToLower(strings.TrimSpace(line))
			if a != "y" && a != "yes" && a != "o" && a != "oui" {
				fmt.Fprintln(env.Stdout, "Aborted.")
				return 1
			}
		}
		if _, err := config.Reset(env.Paths); err != nil {
			fmt.Fprintln(env.Stderr, "termverse: reset failed:", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, "Configuration reset to defaults.")
		return 0
	}
	fmt.Fprintf(env.Stderr, "termverse: unknown config command %q\n", args[0])
	return ExitUsage
}

func themeCmd(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: termverse theme list | theme set <name>")
		return ExitUsage
	}
	cfg, _, err := config.Load(env.Paths, themeExists(env.Themes))
	if err != nil {
		fmt.Fprintln(env.Stderr, "termverse:", err)
		return 1
	}
	switch args[0] {
	case "list", "ls":
		for _, n := range env.Themes.Names() {
			t, _ := env.Themes.Get(n)
			mark := "  "
			if n == cfg.Theme {
				mark = "* "
			}
			src := "built-in"
			if t.Source != "builtin" {
				src = t.Source
			}
			fmt.Fprintf(env.Stdout, "%s%-12s %-34s %s\n", mark, n, t.Description, src)
		}
		return 0
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(env.Stderr, "usage: termverse theme set <name>")
			return ExitUsage
		}
		name := strings.ToLower(args[1])
		if _, ok := env.Themes.Get(name); !ok {
			fmt.Fprintf(env.Stderr, "termverse: unknown theme %q (available: %s)\n", name, strings.Join(env.Themes.Names(), ", "))
			return 1
		}
		cfg.Theme = name
		if err := config.Save(env.Paths, cfg); err != nil {
			fmt.Fprintln(env.Stderr, "termverse:", err)
			return 1
		}
		fmt.Fprintf(env.Stdout, "Theme set to %s.\n", name)
		return 0
	}
	fmt.Fprintf(env.Stderr, "termverse: unknown theme command %q\n", args[0])
	return ExitUsage
}

func systemCmd(env Env) int {
	s := system.NewCollector().Collect(true)
	na := func(ok bool, v string) string {
		if !ok {
			return system.NA
		}
		return v
	}
	rows := [][2]string{
		{"User", s.User}, {"Hostname", s.Hostname}, {"OS", s.OS}, {"Kernel", s.Kernel}, {"Arch", s.Arch},
		{"CPU", fmt.Sprintf("%s (%d cores / %d threads)", s.CPUModel, s.Cores, s.Threads)},
		{"CPU usage", na(s.CPUOK, fmt.Sprintf("%.1f%%", s.CPUPercent))},
		{"Memory", na(s.MemOK, fmt.Sprintf("%s used / %s total (%.1f%%), %s available",
			system.HumanBytes(s.MemUsed), system.HumanBytes(s.MemTotal), s.MemPercent, system.HumanBytes(s.MemAvail)))},
		{"Disk /", na(s.RootDiskOK, fmt.Sprintf("%s used / %s total (%.1f%%), %s free",
			system.HumanBytes(s.RootDisk.Used), system.HumanBytes(s.RootDisk.Total), s.RootDisk.Percent, system.HumanBytes(s.RootDisk.Free)))},
		{"Uptime", na(s.UptimeOK, system.HumanDuration(s.Uptime))},
		{"Load", na(s.LoadOK, fmt.Sprintf("%.2f %.2f %.2f", s.Load1, s.Load5, s.Load15))},
		{"Processes", na(s.ProcsOK, fmt.Sprint(s.Procs))},
		{"Local IP", s.LocalIP},
	}
	for _, r := range rows {
		fmt.Fprintf(env.Stdout, "%-10s %s\n", r[0]+":", r[1])
	}
	return 0
}
