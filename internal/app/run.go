package app

import (
	"context"
	"io/fs"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	termverse "github.com/KAYTAY10ok/TermVerse"
	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/internal/update"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// Setup bootstraps the configuration directories and loads themes.
func Setup() (config.Paths, *theme.Registry, []string, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return paths, nil, nil, err
	}
	themesFS, _ := fs.Sub(termverse.Assets, "assets/themes")
	asciiFS, _ := fs.Sub(termverse.Assets, "assets/ascii")
	var warnings []string
	if err := config.Bootstrap(paths, themesFS, asciiFS); err != nil {
		warnings = append(warnings, err.Error())
	}
	reg, err := theme.NewRegistry(themesFS)
	if err != nil {
		return paths, nil, nil, err
	}
	warnings = append(warnings, reg.LoadDir(paths.ThemesDir)...)
	return paths, reg, warnings, nil
}

// Context builds the shared UI context.
func Context(paths config.Paths, reg *theme.Registry, warnings []string) (*ui.Context, error) {
	cfg, w, err := config.Load(paths, func(n string) bool { _, ok := reg.Get(n); return ok })
	warnings = append(warnings, w...)
	if err != nil && cfg == nil {
		return nil, err
	}
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	changelog, _ := fs.ReadFile(termverse.Assets, "CHANGELOG.md")
	fallback, _ := fs.ReadFile(termverse.Assets, "assets/ascii/default.txt")
	ctx := &ui.Context{
		Paths: paths, Cfg: cfg, Themes: reg, Collector: system.NewCollector(),
		Warnings: warnings, Changelog: string(changelog), FallbackASCII: string(fallback),
		ModuleCount: registry.Count(),
	}
	st := update.LoadState(paths.DataFile("state.json"))
	ctx.LastUpdateCheck, ctx.LatestVersion = st.LastCheck, st.Latest
	ctx.Apply()
	return ctx, nil
}

type startupCheckMsg struct {
	latest string
	err    error
}

// Run starts the TUI and blocks until it exits.
func Run(ctx *ui.Context, factories map[string]Factory) error {
	m := New(ctx, factories)
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if ctx.Cfg.Navigation.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	if ctx.Cfg.Updates.CheckOnStartup {
		go func() {
			r, err := update.Latest(context.Background(), ctx.Cfg.Updates.Repository)
			if err == nil {
				_ = update.SaveState(ctx.Paths.DataFile("state.json"), update.State{LastCheck: time.Now(), Latest: r.Version()})
				ctx.LastUpdateCheck, ctx.LatestVersion = time.Now(), r.Version()
				if update.Compare(version.Version, r.Version()) < 0 {
					p.Send(ui.NotifyMsg{Level: ui.Info, Text: ui.Tf("TermVerse %s is available — see Updates [0]", r.Tag)})
				}
			}
		}()
	}
	_, err := p.Run()
	return err
}
