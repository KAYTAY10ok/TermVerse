// Package automation implements the "Automation Lab": saved command
// shortcuts that are only executed after explicit confirmation.
package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

type doneMsg Result

// Screen is the automation screen.
type Screen struct {
	ctx     *ui.Context
	store   *Store
	list    ui.List
	prompt  ui.Prompt
	confirm ui.Confirm
	out     ui.Scroller
	focus   int // 0 list, 1 output
	running *Command
	cancel  context.CancelFunc
	last    *Result
	history []Result
	loadErr error
	draft   Command
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx, store: &Store{Path: ctx.Paths.DataFile("automation.json")}}
	s.loadErr = s.store.Load()
	return s
}

func (s *Screen) ID() string      { return "automation" }
func (s *Screen) Title() string   { return ui.T("Automation Lab") }
func (s *Screen) Capturing() bool { return s.prompt.Active }

func (s *Screen) Init() tea.Cmd {
	s.list.Vim, s.out.Vim = s.ctx.Vim(), s.ctx.Vim()
	s.refresh()
	return nil
}

func (s *Screen) Keys() []ui.Key {
	if s.prompt.Active {
		return []ui.Key{{Keys: "enter", Desc: "next"}, {Keys: "esc", Desc: "cancel"}}
	}
	if s.running != nil {
		return []ui.Key{{Keys: "x", Desc: "cancel run"}, {Keys: "tab", Desc: "focus output"}}
	}
	return []ui.Key{{Keys: "enter", Desc: "run"}, {Keys: "a", Desc: "add"}, {Keys: "e", Desc: "edit"}, {Keys: "d", Desc: "delete"},
		{Keys: "tab", Desc: "focus output"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) refresh() {
	var items []ui.Item
	for _, c := range s.store.Commands {
		right := ""
		if !c.LastRun.IsZero() {
			mark := "✔"
			if c.LastExit != 0 {
				mark = fmt.Sprintf("✗%d", c.LastExit)
			}
			right = mark + " " + c.LastRun.Format("02 Jan 15:04")
		}
		items = append(items, ui.Item{Title: c.Name, Desc: c.Command, Right: right, Value: c.ID})
	}
	s.list.SetItems(items)
}

func (s *Screen) selected() (Command, bool) {
	it, ok := s.list.Selected()
	if !ok {
		return Command{}, false
	}
	for _, c := range s.store.Commands {
		if c.ID == it.Value.(int64) {
			return c, true
		}
	}
	return Command{}, false
}

// wizard asks name → command → description → directory.
func (s *Screen) wizard(c Command) tea.Cmd {
	s.draft = c
	st := s.ctx.S
	return s.prompt.Open(st, "1/4 Name", "Backup photos", c.Name, func(v string) tea.Cmd {
		s.draft.Name = v
		return s.prompt.Open(st, "2/4 Shell command", "rsync -a ~/Photos /mnt/backup", c.Command, func(v string) tea.Cmd {
			s.draft.Command = v
			return s.prompt.Open(st, "3/4 Description (optional)", "", c.Description, func(v string) tea.Cmd {
				s.draft.Description = strings.TrimSpace(v)
				return s.prompt.Open(st, "4/4 Working directory (empty = current)", "~/projects", c.Dir, func(v string) tea.Cmd {
					s.draft.Dir = strings.TrimSpace(v)
					_, err := s.store.Upsert(s.draft)
					s.refresh()
					if err != nil {
						return ui.Notify(ui.Error, err.Error())
					}
					return ui.Notify(ui.Success, ui.T("Command saved"))
				})
			})
		})
	})
}

func (s *Screen) run(c Command) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	s.running, s.cancel = &c, cancel
	s.focus = 1
	shell := s.ctx.Cfg.Automation.Shell
	timeout := time.Duration(s.ctx.Cfg.Automation.TimeoutSeconds) * time.Second
	return func() tea.Msg { return doneMsg(Run(ctx, shell, c, timeout)) }
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case doneMsg:
		r := Result(msg)
		s.running, s.cancel = nil, nil
		s.last = &r
		s.history = append([]Result{r}, s.history...)
		if len(s.history) > 20 {
			s.history = s.history[:20]
		}
		for i := range s.store.Commands {
			if s.store.Commands[i].ID == r.CommandID {
				s.store.Commands[i].LastRun, s.store.Commands[i].LastExit = r.Started, r.ExitCode
				s.store.Commands[i].Runs++
			}
		}
		_ = s.store.Save()
		s.refresh()
		s.out.Top()
		if r.ExitCode == 0 && r.Err == nil {
			return s, ui.Notify(ui.Success, ui.Tf("%s finished in %s", r.Name, r.Duration.Round(time.Millisecond)))
		}
		return s, ui.Notify(ui.Error, ui.Tf("%s failed (exit %d)", r.Name, r.ExitCode))
	case tea.MouseMsg:
		if s.focus == 1 {
			s.out.HandleMouse(msg)
		} else {
			s.list.HandleMouse(msg)
		}
	case tea.KeyMsg:
		if s.prompt.Active {
			return s, s.prompt.Update(msg)
		}
		if s.confirm.Active {
			return s, s.confirm.Update(msg)
		}
		k := msg.String()
		if k == "x" && s.running != nil {
			s.cancel()
			return s, nil
		}
		if k == "tab" {
			s.focus = 1 - s.focus
			return s, nil
		}
		if s.focus == 1 && s.out.HandleKey(msg) {
			return s, nil
		}
		switch k {
		case "esc", "q":
			if s.focus == 1 {
				s.focus = 0
				return s, nil
			}
			return s, ui.BackCmd
		case "enter", "r":
			if s.running != nil {
				return s, ui.Notify(ui.Warn, ui.T("A command is already running"))
			}
			if c, ok := s.selected(); ok {
				dir := ExpandDir(c.Dir)
				s.confirm.Ask(ui.Tf("Run `%s` in %s?", c.Command, dir), func() tea.Cmd { return s.run(c) })
			}
		case "a", "n":
			return s, s.wizard(Command{})
		case "e":
			if c, ok := s.selected(); ok {
				return s, s.wizard(c)
			}
		case "d", "delete":
			if c, ok := s.selected(); ok {
				s.confirm.Ask(ui.Tf("Delete shortcut %q? (the command itself is not run)", c.Name), func() tea.Cmd {
					err := s.store.Delete(c.ID)
					s.refresh()
					if err != nil {
						return ui.Notify(ui.Error, err.Error())
					}
					return ui.Notify(ui.Success, ui.T("Shortcut deleted"))
				})
			}
		default:
			s.list.HandleKey(msg)
		}
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	wide := w >= 100
	leftW := w
	if wide {
		leftW = w * 2 / 5
	}
	iw, ih := st.PanelInner(leftW, h)
	bottom := ""
	switch {
	case s.prompt.Active:
		bottom = s.prompt.View(st, iw)
	case s.confirm.Active:
		bottom = s.confirm.View(st)
	case s.loadErr != nil:
		bottom = st.Error.Render(ui.Wrap(s.loadErr.Error(), iw))
	default:
		bottom = st.Muted.Render(ui.Wrap(ui.T("Commands run only after you confirm. Stdin is closed: interactive programs are not supported."), iw))
	}
	bh := lipgloss.Height(bottom) + 1
	body := ui.Fit(s.list.View(st, iw, ih-bh), iw, ih-bh) + "\n" + st.Sep(iw) + "\n" + bottom
	if !wide {
		if s.focus == 1 {
			return st.Panel(ui.T("Output"), s.outputView(iw, ih), w, h, true)
		}
		return st.Panel(s.Title(), body, w, h, true)
	}
	left := st.Panel(s.Title(), body, leftW, h, s.focus == 0)
	rw := w - leftW - 1
	riw, rih := st.PanelInner(rw, h)
	right := st.Panel(ui.T("Output"), s.outputView(riw, rih), rw, h, s.focus == 1)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *Screen) outputView(w, h int) string {
	st := s.ctx.S
	if s.running != nil {
		return st.Secondary.Render(ui.T("Running: ")) + st.Accent.Render(s.running.Command) + "\n\n" +
			st.Muted.Render(ui.Tf("Timeout %ds · press x to cancel", s.ctx.Cfg.Automation.TimeoutSeconds))
	}
	if s.last == nil {
		c, ok := s.selected()
		if !ok {
			return st.Muted.Render(ui.T("No commands. Press a to add one."))
		}
		return st.Title.Render(c.Name) + "\n\n" + st.KV([][2]string{
			{ui.T("Command"), c.Command}, {ui.T("Directory"), ExpandDir(c.Dir)}, {ui.T("Runs"), fmt.Sprint(c.Runs)},
		}, 10) + "\n\n" + st.Text.Render(ui.Wrap(c.Description, w)) + "\n\n" + st.Muted.Render(ui.T("Press Enter to run (you will be asked to confirm)."))
	}
	r := s.last
	status := st.Success.Render(ui.Tf("✔ exit %d", r.ExitCode))
	if r.ExitCode != 0 || r.Err != nil {
		status = st.Error.Render(ui.Tf("✗ exit %d", r.ExitCode))
	}
	head := st.Title.Render(r.Name) + "  " + status + st.Muted.Render(fmt.Sprintf("  %s · %s", r.Duration.Round(time.Millisecond), r.Started.Format("15:04:05"))) + "\n" +
		st.Accent.Render("$ "+r.Command) + "\n"
	if r.Err != nil {
		head += st.Error.Render(r.Err.Error()) + "\n"
	}
	var b strings.Builder
	if out := strings.TrimRight(ui.Sanitize(r.Stdout), "\n"); out != "" {
		b.WriteString(st.Muted.Render("── stdout ──") + "\n" + st.Text.Render(out) + "\n")
	}
	if e := strings.TrimRight(ui.Sanitize(r.Stderr), "\n"); e != "" {
		b.WriteString(st.Muted.Render("── stderr ──") + "\n" + st.Error.Render(e) + "\n")
	}
	if b.Len() == 0 {
		b.WriteString(st.Muted.Render(ui.T("(no output)")))
	}
	if len(s.history) > 1 {
		b.WriteString("\n" + st.Muted.Render(ui.T("── history ──")) + "\n")
		for _, h := range s.history[1:] {
			b.WriteString(st.Muted.Render(fmt.Sprintf("%s  exit %-3d %-8s %s", h.Started.Format("15:04:05"), h.ExitCode, h.Duration.Round(time.Millisecond), h.Name)) + "\n")
		}
	}
	s.out.SetContent(b.String())
	hh := lipgloss.Height(head)
	return head + s.out.View(st, w, max(1, h-hh))
}
