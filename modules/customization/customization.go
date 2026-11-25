// Package customization implements the "Customization" section: live theme
// switching, color overrides, borders, separators, ASCII art and widgets.
package customization

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// Screen is the customization screen.
type Screen struct {
	ctx     *ui.Context
	opts    ui.Options
	prompt  ui.Prompt
	confirm ui.Confirm
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx}
	s.build()
	return s
}

func (s *Screen) ID() string      { return "customize" }
func (s *Screen) Title() string   { return ui.T("Customization") }
func (s *Screen) Capturing() bool { return s.prompt.Active }
func (s *Screen) Init() tea.Cmd {
	// Pick up theme files added since the last visit.
	s.ctx.Themes.LoadDir(s.ctx.Paths.ThemesDir)
	s.opts.Vim = s.ctx.Vim()
	return nil
}

func (s *Screen) Keys() []ui.Key {
	if s.prompt.Active {
		return []ui.Key{{Keys: "enter", Desc: "confirm"}, {Keys: "esc", Desc: "cancel"}}
	}
	return []ui.Key{{Keys: "↑↓", Desc: "select"}, {Keys: "←→", Desc: "change"}, {Keys: "enter", Desc: "edit"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) save() tea.Cmd { return s.ctx.SaveAndApply("") }

func (s *Screen) asciiFiles() []string {
	entries, _ := os.ReadDir(s.ctx.Paths.ASCIIDir)
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	cur := s.ctx.Cfg.Appearance.ASCIIFile
	found := false
	for _, f := range out {
		if f == cur {
			found = true
		}
	}
	if !found && cur != "" {
		out = append(out, cur)
	}
	return out
}

func (s *Screen) build() {
	c := s.ctx.Cfg
	a := &c.Appearance
	w := &c.Widgets
	toggle := func(label string, p *bool) ui.Option {
		return ui.Option{Label: label, Value: func() string { return ui.OnOff(*p) },
			Cycle: func(int) tea.Cmd { *p = !*p; return s.save() }}
	}
	colorOpt := func(label string, p *string, themeColor func() string) ui.Option {
		return ui.Option{Label: label,
			Value: func() string {
				if *p == "" {
					return s.ctx.S.Muted.Render(ui.T("theme")+" ") + swatch(s.ctx, themeColor())
				}
				return swatch(s.ctx, *p)
			},
			Enter: func() tea.Cmd {
				return s.prompt.Open(s.ctx.S, ui.Tf("%s — hex color (#RRGGBB), empty = theme default", ui.T(label)), "#FF00AA", *p,
					func(v string) tea.Cmd {
						v = strings.TrimSpace(v)
						if v == "" {
							*p = ""
							return s.save()
						}
						n, ok := theme.NormalizeHex(v)
						if !ok {
							return ui.Notify(ui.Error, ui.Tf("Invalid color %q", v))
						}
						*p = n
						return s.save()
					})
			}}
	}
	cycleStr := func(label string, p *string, list func() []string) ui.Option {
		return ui.Option{Label: label, Value: func() string { return *p },
			Cycle: func(d int) tea.Cmd { *p = ui.CycleIndex(list(), *p, d); return s.save() }}
	}
	s.opts.Items = []ui.Option{
		{Label: "Theme", Header: true},
		cycleStr("Theme", &c.Theme, s.ctx.Themes.Names),
		colorOpt("Primary color", &a.PrimaryColor, func() string { return s.baseTheme().Colors.Primary }),
		colorOpt("Secondary color", &a.SecondaryColor, func() string { return s.baseTheme().Colors.Secondary }),
		colorOpt("Accent color", &a.AccentColor, func() string { return s.baseTheme().Colors.Accent }),
		{Label: "Export as custom theme", Enter: s.exportTheme},
		{Label: "Borders & symbols", Header: true},
		toggle("Borders", &a.Borders),
		cycleStr("Border style", &a.BorderStyle, func() []string { return config.BorderStyles }),
		cycleStr("Separator", &a.Separator, func() []string { return config.Separators }),
		cycleStr("Selector", &a.Selector, func() []string { return config.Selectors }),
		toggle("Animations", &a.Animations),
		{Label: "ASCII art", Header: true},
		cycleStr("ASCII file", &a.ASCIIFile, s.asciiFiles),
		{Label: "Custom ASCII path", Value: func() string { return "" }, Enter: func() tea.Cmd {
			return s.prompt.Open(s.ctx.S, "Path to an ASCII art text file (absolute, ~/… or name in ascii/)", "~/my-art.txt", a.ASCIIFile,
				func(v string) tea.Cmd {
					v = strings.TrimSpace(v)
					if v == "" {
						return nil
					}
					if _, err := os.Stat(s.ctx.Paths.ASCIIPath(v)); err != nil {
						return ui.Notify(ui.Error, ui.T("Cannot read file: ")+err.Error())
					}
					a.ASCIIFile = v
					return s.save()
				})
		}},
		toggle("Gradient", &a.ASCIIGradient),
		{Label: "Dashboard widgets", Header: true},
		toggle("Clock", &w.Clock),
		toggle("ASCII banner", &w.ASCII),
		toggle("System info", &w.SystemInfo),
		toggle("TermVerse info", &w.TermVerseInfo),
		toggle("Tips", &w.Tips),
		{Label: "Reset", Header: true},
		{Label: "Restore default appearance", Enter: func() tea.Cmd {
			s.confirm.Ask(ui.T("Restore default theme, colors, borders, ASCII and widgets?"), func() tea.Cmd {
				d := config.Default()
				c.Theme, c.Appearance, c.Widgets = d.Theme, d.Appearance, d.Widgets
				s.build()
				return s.ctx.SaveAndApply("Default appearance restored")
			})
			return nil
		}},
	}
	s.opts.Fix()
}

func (s *Screen) baseTheme() theme.Theme {
	t, _ := s.ctx.Themes.Get(s.ctx.Cfg.Theme)
	return t
}

func swatch(ctx *ui.Context, hex string) string {
	return ctx.S.Text.Foreground(lipColor(hex)).Render("████") + " " + hex
}

// exportTheme writes the current effective palette as a new theme file.
// It refuses to overwrite an existing file.
func (s *Screen) exportTheme() tea.Cmd {
	return s.prompt.Open(s.ctx.S, "Name of the new theme (letters, digits, - or _)", "my-theme", "",
		func(v string) tea.Cmd {
			name := strings.ToLower(strings.TrimSpace(v))
			t := s.ctx.Theme
			t.Name, t.Description, t.Author = name, "Custom theme exported from TermVerse", os.Getenv("USER")
			data, err := t.Marshal()
			if err != nil {
				return ui.Notify(ui.Error, err.Error())
			}
			if _, err := theme.Parse(data, theme.Colors{}); err != nil {
				return ui.Notify(ui.Error, ui.T("Invalid theme name: ")+err.Error())
			}
			path := filepath.Join(s.ctx.Paths.ThemesDir, name+".json")
			if _, ok := s.ctx.Themes.Get(name); ok || fsutil.Exists(path) {
				return ui.Notify(ui.Error, ui.Tf("Theme %q already exists — choose another name", name))
			}
			if err := fsutil.WriteFileAtomic(path, data, 0o644); err != nil {
				return ui.Notify(ui.Error, err.Error())
			}
			s.ctx.Themes.LoadDir(s.ctx.Paths.ThemesDir)
			s.ctx.Cfg.Theme = name
			a := &s.ctx.Cfg.Appearance
			a.PrimaryColor, a.SecondaryColor, a.AccentColor = "", "", ""
			return ui.Batch(s.save(), ui.Notify(ui.Success, ui.Tf("Theme saved to %s", path)))
		})
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if s.prompt.Active {
		return s, s.prompt.Update(msg)
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.confirm.Active {
		return s, s.confirm.Update(msg)
	}
	if km.String() == "esc" || km.String() == "q" {
		return s, ui.BackCmd
	}
	_, cmd := s.opts.HandleKey(km)
	return s, cmd
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	leftW := w
	showPreview := w >= 90
	if showPreview {
		leftW = w * 55 / 100
	}
	iw, ih := st.PanelInner(leftW, h)
	bottom := ""
	switch {
	case s.prompt.Active:
		bottom = s.prompt.View(st, iw)
	case s.confirm.Active:
		bottom = s.confirm.View(st)
	default:
		bottom = st.Muted.Render(ui.T("Changes apply instantly and are saved to config.json."))
	}
	bh := strings.Count(bottom, "\n") + 1
	body := ui.Fit(s.opts.View(st, iw, ih-bh-1), iw, ih-bh-1) + "\n" + st.Sep(iw) + "\n" + bottom
	left := st.Panel(s.Title(), body, leftW, h, true)
	if !showPreview {
		return left
	}
	rw := w - leftW - 1
	return joinH(left, st.Panel(ui.T("Live preview"), s.preview(rw-4), rw, h, false))
}

func (s *Screen) preview(width int) string {
	st := s.ctx.S
	t := s.ctx.Theme
	var b strings.Builder
	b.WriteString(st.Title.Render(t.Name) + " " + st.Muted.Render(ui.Truncate(ui.T(t.Description), width-len(t.Name)-1)) + "\n")
	src := ui.T("built-in")
	if t.Source != "builtin" {
		src = ui.Tildify(t.Source)
	}
	b.WriteString(st.Muted.Render(ui.T("Source: ")+ui.Truncate(src, width-8)) + "\n\n")
	c := t.Colors
	for _, p := range [][2]string{{"primary", c.Primary}, {"secondary", c.Secondary}, {"accent", c.Accent},
		{"text", c.Text}, {"muted", c.Muted}, {"success", c.Success}, {"warning", c.Warning}, {"error", c.Error}} {
		b.WriteString(st.Muted.Render(fmt.Sprintf("%-10s", p[0])) + swatch(s.ctx, p[1]) + "\n")
	}
	b.WriteString("\n" + st.Bar(42, min(width, 36)) + "\n" + st.Bar(73, min(width, 36)) + "\n" + st.Bar(91, min(width, 36)) + "\n\n")
	b.WriteString(st.Selected.Render(" "+st.Selector+" "+ui.T("Selected item")+" ") + "\n")
	b.WriteString("  " + st.Text.Render(ui.T("Normal item")) + "\n")
	b.WriteString(st.Sep(min(width, 36)) + "\n")
	if s.ctx.Cfg.Widgets.ASCII && s.ctx.ASCII != "" {
		art := s.ctx.ASCII
		if s.ctx.Cfg.Appearance.ASCIIGradient {
			art = ui.Gradient(art, c.Primary, c.Secondary, 0)
		} else {
			art = st.Primary.Render(art)
		}
		b.WriteString(art)
	}
	if s.ctx.ASCIIErr != "" {
		b.WriteString("\n" + st.Error.Render(ui.T("ASCII file error: ")+s.ctx.ASCIIErr))
	}
	return b.String()
}
