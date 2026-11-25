package ui

import (
	"os"
	"strings"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
)

// Context is the state shared by the app and every screen.
type Context struct {
	Paths     config.Paths
	Cfg       *config.Config
	Themes    *theme.Registry
	Theme     theme.Theme
	S         Styles
	Collector *system.Collector

	Sys   system.Snapshot
	SysOK bool

	// ASCII is the raw ASCII art loaded from the configured file.
	ASCII    string
	ASCIIErr string

	Warnings []string

	// Module registry info for the dashboard.
	ModuleCount int

	// Update check state.
	LastUpdateCheck time.Time
	LatestVersion   string

	// Changelog is the embedded CHANGELOG.md.
	Changelog string

	// FallbackASCII is used when the configured file cannot be read.
	FallbackASCII string
}

// Vim reports whether vim navigation keys are enabled.
func (c *Context) Vim() bool { return c.Cfg.Navigation.VimKeys }

// Apply recomputes the theme, styles, language and ASCII art from Cfg.
func (c *Context) Apply() {
	SetLanguage(c.Cfg.Language)
	t, ok := c.Themes.Get(c.Cfg.Theme)
	if !ok {
		t, _ = c.Themes.Get(theme.DefaultName)
	}
	a := c.Cfg.Appearance
	c.Theme = t.Apply(theme.Overrides{Primary: a.PrimaryColor, Secondary: a.SecondaryColor, Accent: a.AccentColor})
	c.S = NewStyles(c.Theme, a)
	c.loadASCII()
}

func (c *Context) loadASCII() {
	path := c.Paths.ASCIIPath(c.Cfg.Appearance.ASCIIFile)
	data, err := os.ReadFile(path)
	c.ASCIIErr = ""
	if err != nil {
		c.ASCII = c.FallbackASCII
		c.ASCIIErr = err.Error()
		return
	}
	art := strings.TrimRight(Sanitize(strings.ReplaceAll(string(data), "\t", "    ")), "\n ")
	lines := strings.Split(art, "\n")
	if len(lines) > 30 {
		lines = lines[:30]
	}
	c.ASCII = strings.Join(lines, "\n")
}

// Save persists the configuration atomically.
func (c *Context) Save() error { return config.Save(c.Paths, c.Cfg) }

// SaveAndApply validates, persists and applies the configuration, returning
// a notification command.
func (c *Context) SaveAndApply(okMsg string) Cmd {
	c.Cfg.Validate(func(n string) bool { _, ok := c.Themes.Get(n); return ok })
	c.Apply()
	if err := c.Save(); err != nil {
		return Notify(Error, T("Could not save settings: ")+err.Error())
	}
	if okMsg == "" {
		return ConfigChanged
	}
	return Batch(ConfigChanged, Notify(Success, T(okMsg)))
}
