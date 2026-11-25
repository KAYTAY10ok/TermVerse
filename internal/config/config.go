// Package config manages the persistent TermVerse configuration.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
)

// SchemaVersion is the current configuration format version.
const SchemaVersion = 1

// Config is the content of config.json.
type Config struct {
	SchemaVersion  int             `json:"schema_version"`
	Language       string          `json:"language"` // auto, en, fr
	Theme          string          `json:"theme"`
	RefreshSeconds int             `json:"refresh_seconds"`
	Appearance     Appearance      `json:"appearance"`
	Widgets        Widgets         `json:"widgets"`
	Navigation     Navigation      `json:"navigation"`
	Modules        map[string]bool `json:"modules"`
	Productivity   Productivity    `json:"productivity"`
	Automation     Automation      `json:"automation"`
	Updates        Updates         `json:"updates"`
}

// Appearance holds visual preferences.
type Appearance struct {
	// Color overrides applied on top of the theme ("" = use theme color).
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
	AccentColor    string `json:"accent_color"`
	Borders        bool   `json:"borders"`
	BorderStyle    string `json:"border_style"` // rounded, normal, double, thick
	Separator      string `json:"separator"`
	Selector       string `json:"selector"`
	// ASCIIFile is a file name inside ascii/ or an absolute path.
	ASCIIFile     string `json:"ascii_file"`
	ASCIIGradient bool   `json:"ascii_gradient"`
	Animations    bool   `json:"animations"`
}

// Widgets toggles dashboard widgets.
type Widgets struct {
	Clock         bool `json:"clock"`
	ASCII         bool `json:"ascii"`
	SystemInfo    bool `json:"system_info"`
	TermVerseInfo bool `json:"termverse_info"`
	Tips          bool `json:"tips"`
}

// Navigation holds input preferences.
type Navigation struct {
	VimKeys     bool `json:"vim_keys"`
	Mouse       bool `json:"mouse"`
	ConfirmQuit bool `json:"confirm_quit"`
}

// Productivity holds Pomodoro durations in minutes.
type Productivity struct {
	PomodoroWork  int `json:"pomodoro_work_minutes"`
	PomodoroBreak int `json:"pomodoro_break_minutes"`
}

// Automation holds command runner settings.
type Automation struct {
	TimeoutSeconds int    `json:"timeout_seconds"`
	Shell          string `json:"shell"`
}

// Updates holds update-check settings. TermVerse never installs updates by
// itself; it only checks when the user asks (or on startup if enabled).
type Updates struct {
	CheckOnStartup bool   `json:"check_on_startup"`
	Repository     string `json:"repository"` // owner/name
}

// Allowed values.
var (
	BorderStyles = []string{"rounded", "normal", "double", "thick"}
	Separators   = []string{"─", "━", "═", "┄", "·", "-", "~"}
	Selectors    = []string{"▶", "❯", "→", "●", "◆", ">"}
	Refreshes    = []int{1, 2, 3, 5, 10, 30}
	Languages    = []string{"auto", "en", "fr"}
)

// Default returns the default configuration.
func Default() Config {
	return Config{
		SchemaVersion:  SchemaVersion,
		Language:       "auto",
		Theme:          theme.DefaultName,
		RefreshSeconds: 2,
		Appearance: Appearance{
			Borders:       true,
			BorderStyle:   "rounded",
			Separator:     "─",
			Selector:      "▶",
			ASCIIFile:     "default.txt",
			ASCIIGradient: true,
			Animations:    true,
		},
		Widgets:    Widgets{Clock: true, ASCII: true, SystemInfo: true, TermVerseInfo: true, Tips: true},
		Navigation: Navigation{VimKeys: true, Mouse: false, ConfirmQuit: false},
		Modules:    map[string]bool{},
		Productivity: Productivity{
			PomodoroWork:  25,
			PomodoroBreak: 5,
		},
		Automation: Automation{TimeoutSeconds: 60, Shell: "/bin/sh"},
		Updates:    Updates{CheckOnStartup: false, Repository: "KAYTAY10ok/TermVerse"},
	}
}

func contains[T comparable](list []T, v T) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Validate fixes out-of-range values in place and returns a description of
// every correction. themeExists reports whether a theme name is known (nil
// skips the theme check).
func (c *Config) Validate(themeExists func(string) bool) []string {
	var w []string
	d := Default()
	fix := func(msg string, args ...any) { w = append(w, fmt.Sprintf(msg, args...)) }

	c.SchemaVersion = SchemaVersion
	c.Theme = strings.ToLower(strings.TrimSpace(c.Theme))
	if c.Theme == "" || (themeExists != nil && !themeExists(c.Theme)) {
		fix("unknown theme %q, using %q", c.Theme, d.Theme)
		c.Theme = d.Theme
	}
	if !contains(Languages, c.Language) {
		fix("unknown language %q, using auto", c.Language)
		c.Language = "auto"
	}
	if c.RefreshSeconds < 1 || c.RefreshSeconds > 60 {
		fix("refresh_seconds must be between 1 and 60, using %d", d.RefreshSeconds)
		c.RefreshSeconds = d.RefreshSeconds
	}
	a := &c.Appearance
	for _, p := range []*string{&a.PrimaryColor, &a.SecondaryColor, &a.AccentColor} {
		if *p == "" {
			continue
		}
		if n, ok := theme.NormalizeHex(*p); ok {
			*p = n
		} else {
			fix("invalid color override %q ignored", *p)
			*p = ""
		}
	}
	if !contains(BorderStyles, a.BorderStyle) {
		fix("unknown border_style %q, using %q", a.BorderStyle, d.Appearance.BorderStyle)
		a.BorderStyle = d.Appearance.BorderStyle
	}
	if a.Separator == "" || len([]rune(a.Separator)) > 2 {
		a.Separator = d.Appearance.Separator
	}
	if a.Selector == "" || len([]rune(a.Selector)) > 2 {
		a.Selector = d.Appearance.Selector
	}
	if a.ASCIIFile == "" {
		a.ASCIIFile = d.Appearance.ASCIIFile
	}
	if c.Modules == nil {
		c.Modules = map[string]bool{}
	}
	p := &c.Productivity
	if p.PomodoroWork < 1 || p.PomodoroWork > 180 {
		fix("pomodoro_work_minutes out of range, using %d", d.Productivity.PomodoroWork)
		p.PomodoroWork = d.Productivity.PomodoroWork
	}
	if p.PomodoroBreak < 1 || p.PomodoroBreak > 60 {
		fix("pomodoro_break_minutes out of range, using %d", d.Productivity.PomodoroBreak)
		p.PomodoroBreak = d.Productivity.PomodoroBreak
	}
	if c.Automation.TimeoutSeconds < 1 || c.Automation.TimeoutSeconds > 3600 {
		fix("automation timeout out of range, using %d", d.Automation.TimeoutSeconds)
		c.Automation.TimeoutSeconds = d.Automation.TimeoutSeconds
	}
	if c.Automation.Shell == "" {
		c.Automation.Shell = d.Automation.Shell
	}
	if strings.Count(c.Updates.Repository, "/") != 1 {
		c.Updates.Repository = d.Updates.Repository
	}
	return w
}

// ModuleEnabled reports whether an optional module is enabled (default true).
func (c *Config) ModuleEnabled(id string) bool {
	v, ok := c.Modules[id]
	return !ok || v
}

// Load reads the configuration. A missing file yields the defaults (and is
// written). An invalid file is backed up as config.json.invalid-<time>, the
// defaults are used and a warning is returned — user data is never lost.
func Load(p Paths, themeExists func(string) bool) (*Config, []string, error) {
	cfg := Default()
	var warnings []string
	found, err := fsutil.ReadJSON(p.ConfigFile, &cfg)
	if err != nil {
		if !found {
			return nil, nil, fmt.Errorf("read config: %w", err)
		}
		backup, berr := fsutil.BackupCorrupt(p.ConfigFile, time.Now().Format("20060102-150405"))
		if berr != nil {
			return nil, nil, fmt.Errorf("%v (and backup failed: %v)", err, berr)
		}
		warnings = append(warnings, fmt.Sprintf("config.json was invalid and has been saved to %s; defaults restored", filepath.Base(backup)))
		cfg = Default()
		found = false
	}
	warnings = append(warnings, cfg.Validate(themeExists)...)
	if !found {
		if err := Save(p, &cfg); err != nil {
			return &cfg, warnings, err
		}
	}
	return &cfg, warnings, nil
}

// Save writes the configuration atomically.
func Save(p Paths, c *Config) error {
	return fsutil.WriteJSONAtomic(p.ConfigFile, c)
}

// Reset overwrites config.json with the defaults. Themes, ASCII files and
// user data are left untouched.
func Reset(p Paths) (*Config, error) {
	c := Default()
	return &c, Save(p, &c)
}

// Bootstrap creates the directory layout and seeds built-in themes and the
// default ASCII art. Existing files are never overwritten.
func Bootstrap(p Paths, themes fs.FS, ascii fs.FS) error {
	for _, d := range []string{p.ConfigDir, p.ThemesDir, p.ASCIIDir, p.PluginsDir, p.DataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	seed := func(src fs.FS, dst string) error {
		return fs.WalkDir(src, ".", func(path string, de fs.DirEntry, err error) error {
			if err != nil || de.IsDir() {
				return err
			}
			data, err := fs.ReadFile(src, path)
			if err != nil {
				return err
			}
			_, err = fsutil.WriteIfMissing(filepath.Join(dst, path), data, 0o644)
			return err
		})
	}
	if err := seed(themes, p.ThemesDir); err != nil {
		return fmt.Errorf("seed themes: %w", err)
	}
	if err := seed(ascii, p.ASCIIDir); err != nil {
		return fmt.Errorf("seed ascii: %w", err)
	}
	return nil
}

// ASCIIPath resolves the configured ASCII art file.
func (p Paths) ASCIIPath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	if strings.HasPrefix(name, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, name[2:])
		}
	}
	return filepath.Join(p.ASCIIDir, filepath.Clean("/" + name)[1:])
}
