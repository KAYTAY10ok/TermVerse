// Package systemcenter implements the "System Center" section: live
// resources overview and a process manager.
package systemcenter

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

var tabs = []string{"Overview", "Processes"}

type procsMsg struct {
	gen   int
	procs []system.Proc
	err   error
}
type procTick struct{ gen int }

// Screen is the System Center.
type Screen struct {
	ctx     *ui.Context
	tab     int
	scroll  ui.Scroller
	active  bool
	gen     int
	loading bool

	procs   []system.Proc
	procErr error
	list    ui.List
	sortBy  string
	filter  ui.Prompt
	query   string
	confirm ui.Confirm
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen { return &Screen{ctx: ctx, sortBy: "cpu"} }

func (s *Screen) ID() string      { return "system" }
func (s *Screen) Title() string   { return ui.T("System Center") }
func (s *Screen) Capturing() bool { return s.filter.Active }
func (s *Screen) OnLeave()        { s.active = false }

func (s *Screen) Init() tea.Cmd {
	s.active = true
	s.gen++
	s.scroll.Vim, s.list.Vim = s.ctx.Vim(), s.ctx.Vim()
	if s.tab == 1 {
		return s.loadProcs()
	}
	return nil
}

// OpenSub opens the process manager with "procs".
func (s *Screen) OpenSub(sub string) tea.Cmd {
	if sub == "procs" && s.tab != 1 {
		s.tab = 1
		return s.loadProcs()
	}
	return nil
}

func (s *Screen) Keys() []ui.Key {
	if s.filter.Active {
		return []ui.Key{{Keys: "enter", Desc: "apply"}, {Keys: "esc", Desc: "cancel"}}
	}
	if s.tab == 1 {
		return []ui.Key{{Keys: "tab", Desc: "switch tab"}, {Keys: "c/m/p/n", Desc: "sort"}, {Keys: "/", Desc: "filter"},
			{Keys: "t", Desc: "terminate"}, {Keys: "K", Desc: "kill"}, {Keys: "esc", Desc: "back"}}
	}
	return []ui.Key{{Keys: "tab", Desc: "switch tab"}, {Keys: "↑↓", Desc: "scroll"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) loadProcs() tea.Cmd {
	if s.loading {
		return nil
	}
	s.loading = true
	gen, c := s.gen, s.ctx.Collector
	return func() tea.Msg {
		ps, err := c.Processes()
		return procsMsg{gen: gen, procs: ps, err: err}
	}
}

func (s *Screen) applyProcs() {
	system.SortProcs(s.procs, s.sortBy)
	q := strings.ToLower(s.query)
	var items []ui.Item
	for _, p := range s.procs {
		if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.User+" "+p.Cmdline+fmt.Sprint(p.PID)), q) {
			continue
		}
		items = append(items, ui.Item{Title: procLine(p), Value: p})
	}
	s.list.SetItems(items)
}

func procLine(p system.Proc) string {
	user := p.User
	if len(user) > 10 {
		user = user[:10]
	}
	cmd := p.Cmdline
	if cmd == "" {
		cmd = "[" + p.Name + "]"
	}
	return fmt.Sprintf("%7d %-10s %-2s %6.1f %9s  %s", p.PID, user, p.Status, p.CPU, system.HumanBytes(p.RSS), ui.Sanitize(cmd))
}

func (s *Screen) signal(sig syscall.Signal, name string) tea.Cmd {
	it, ok := s.list.Selected()
	if !ok {
		return nil
	}
	p := it.Value.(system.Proc)
	s.confirm.Ask(ui.Tf("Send %s to %s (PID %d)?", name, p.Name, p.PID), func() tea.Cmd {
		if err := system.Signal(p.PID, sig); err != nil {
			return ui.Notify(ui.Error, ui.Tf("Signal failed: %v", err))
		}
		return ui.Batch(ui.Notify(ui.Success, ui.Tf("%s sent to PID %d", name, p.PID)), s.loadProcs())
	})
	return nil
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case procsMsg:
		s.loading = false
		if msg.gen != s.gen {
			return s, nil
		}
		s.procs, s.procErr = msg.procs, msg.err
		s.applyProcs()
		if s.active && s.tab == 1 {
			gen := s.gen
			return s, tea.Tick(time.Duration(s.ctx.Cfg.RefreshSeconds)*time.Second, func(time.Time) tea.Msg { return procTick{gen} })
		}
		return s, nil
	case procTick:
		if msg.gen == s.gen && s.active && s.tab == 1 {
			return s, s.loadProcs()
		}
		return s, nil
	case tea.MouseMsg:
		if s.tab == 0 {
			s.scroll.HandleMouse(msg)
		} else {
			s.list.HandleMouse(msg)
		}
		return s, nil
	case tea.KeyMsg:
		if s.filter.Active {
			return s, s.filter.Update(msg)
		}
		if s.confirm.Active {
			return s, s.confirm.Update(msg)
		}
		k := msg.String()
		switch k {
		case "esc", "q":
			if s.query != "" && s.tab == 1 {
				s.query = ""
				s.applyProcs()
				return s, nil
			}
			return s, ui.BackCmd
		case "tab", "shift+tab":
			s.tab = 1 - s.tab
			s.gen++
			if s.tab == 1 {
				return s, s.loadProcs()
			}
			return s, nil
		}
		if s.tab == 0 {
			s.scroll.HandleKey(msg)
			return s, nil
		}
		switch k {
		case "c", "m", "p", "n":
			s.sortBy = map[string]string{"c": "cpu", "m": "mem", "p": "pid", "n": "name"}[k]
			s.applyProcs()
		case "/":
			return s, s.filter.Open(s.ctx.S, "Filter processes (name, user, command, PID)", "firefox", s.query, func(v string) tea.Cmd {
				s.query = strings.TrimSpace(v)
				s.applyProcs()
				return nil
			})
		case "t":
			return s, s.signal(syscall.SIGTERM, "SIGTERM")
		case "K":
			return s, s.signal(syscall.SIGKILL, "SIGKILL")
		case "r":
			return s, s.loadProcs()
		default:
			s.list.HandleKey(msg)
		}
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	iw, ih := st.PanelInner(w, h)
	head := st.Tabs(tabs, s.tab) + "\n" + st.Sep(iw) + "\n"
	var body string
	if s.tab == 0 {
		s.scroll.SetContent(s.overview(iw))
		body = s.scroll.View(st, iw, ih-2)
	} else {
		body = s.procView(iw, ih-2)
	}
	return st.Panel(s.Title(), head+body, w, h, true)
}

func (s *Screen) overview(width int) string {
	st := s.ctx.S
	if !s.ctx.SysOK {
		return st.Muted.Render(ui.T("Collecting system information…"))
	}
	sn := s.ctx.Sys
	barW := min(50, width-22)
	row := func(label string, ok bool, pct float64, extra string) string {
		l := st.Muted.Render(ui.PadRight(label, 12))
		if !ok {
			return l + st.Text.Render(system.NA)
		}
		return l + st.Bar(pct, barW) + "  " + st.Muted.Render(extra)
	}
	var b strings.Builder
	b.WriteString(st.Title.Render(ui.T("Resources")) + "\n")
	b.WriteString(row(ui.T("CPU"), sn.CPUOK, sn.CPUPercent, fmt.Sprintf("%d %s", sn.Threads, ui.T("threads"))) + "\n")
	b.WriteString(row(ui.T("Memory"), sn.MemOK, sn.MemPercent, fmt.Sprintf("%s / %s (%s %s)", system.HumanBytes(sn.MemUsed), system.HumanBytes(sn.MemTotal), system.HumanBytes(sn.MemAvail), ui.T("available"))) + "\n")
	if sn.SwapOK && sn.SwapTotal > 0 {
		b.WriteString(row(ui.T("Swap"), true, sn.SwapPercent, fmt.Sprintf("%s / %s", system.HumanBytes(sn.SwapUsed), system.HumanBytes(sn.SwapTotal))) + "\n")
	} else {
		b.WriteString(st.Muted.Render(ui.PadRight(ui.T("Swap"), 12)) + st.Text.Render(ui.T("none")) + "\n")
	}
	load := system.NA
	if sn.LoadOK {
		load = fmt.Sprintf("%.2f  %.2f  %.2f", sn.Load1, sn.Load5, sn.Load15)
	}
	up := system.NA
	if sn.UptimeOK {
		up = system.HumanDuration(sn.Uptime)
	}
	procs := system.NA
	if sn.ProcsOK {
		procs = fmt.Sprint(sn.Procs)
	}
	b.WriteString(st.KV([][2]string{{ui.T("Load avg"), load}, {ui.T("Uptime"), up}, {ui.T("Processes"), procs}}, 11) + "\n\n")

	if len(sn.PerCore) > 0 {
		b.WriteString(st.Title.Render(ui.T("CPU cores")) + "\n")
		cols := 1
		if width >= 90 {
			cols = 2
		}
		if width >= 140 {
			cols = 3
		}
		cw := width / cols
		var line []string
		for i, p := range sn.PerCore {
			line = append(line, ui.PadRight(st.Muted.Render(fmt.Sprintf("#%-3d", i))+st.Bar(p, cw-7), cw))
			if len(line) == cols || i == len(sn.PerCore)-1 {
				b.WriteString(strings.Join(line, "") + "\n")
				line = nil
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(st.Title.Render(ui.T("Disks")) + "\n")
	if len(sn.Disks) == 0 {
		b.WriteString(st.Muted.Render(ui.T("Collecting…")) + "\n")
	}
	for _, d := range sn.Disks {
		b.WriteString(row(ui.Truncate(d.Mount, 11), true, d.Percent,
			fmt.Sprintf("%s / %s  %s %s", system.HumanBytes(d.Used), system.HumanBytes(d.Total), d.FSType, d.Device)) + "\n")
	}
	b.WriteString("\n" + st.Title.Render(ui.T("Hardware")) + "\n")
	b.WriteString(st.KV([][2]string{
		{ui.T("CPU model"), sn.CPUModel},
		{ui.T("Cores"), fmt.Sprintf("%d %s / %d %s", sn.Cores, ui.T("physical"), sn.Threads, ui.T("logical"))},
		{ui.T("Arch"), sn.Arch},
		{ui.T("Kernel"), sn.Kernel},
		{ui.T("OS"), sn.OS},
		{ui.T("Virtualization"), sn.Virt},
	}, 14) + "\n")
	if len(sn.Temps) > 0 {
		b.WriteString("\n" + st.Title.Render(ui.T("Temperatures")) + "\n")
		for _, t := range sn.Temps {
			style := st.Success
			if t.Value >= 80 {
				style = st.Error
			} else if t.Value >= 65 {
				style = st.Warning
			}
			b.WriteString(st.Muted.Render(ui.PadRight(ui.Truncate(t.Name, 24), 26)) + style.Render(fmt.Sprintf("%.0f °C", t.Value)) + "\n")
		}
	} else {
		b.WriteString("\n" + st.Muted.Render(ui.T("Temperature sensors: not available")) + "\n")
	}
	return b.String()
}

func (s *Screen) procView(width, height int) string {
	st := s.ctx.S
	sortLabel := map[string]string{"cpu": "CPU", "mem": ui.T("memory"), "pid": "PID", "name": ui.T("name")}[s.sortBy]
	info := st.Muted.Render(ui.Tf("%d processes · sorted by %s", len(s.list.Items), sortLabel))
	if s.query != "" {
		info += st.Accent.Render("  " + ui.T("filter: ") + s.query)
	}
	header := st.Bold.Render(fmt.Sprintf("  %7s %-10s %-2s %6s %9s  %s", "PID", ui.T("USER"), "S", "CPU%", "RSS", ui.T("COMMAND")))
	bottom := ""
	switch {
	case s.filter.Active:
		bottom = s.filter.View(st, width)
	case s.confirm.Active:
		bottom = s.confirm.View(st)
	case s.procErr != nil:
		bottom = st.Error.Render(s.procErr.Error())
	}
	bh := 0
	if bottom != "" {
		bh = lipgloss.Height(bottom) + 1
	}
	var listView string
	if s.procs == nil {
		listView = st.Muted.Render(ui.T("Loading processes…"))
	} else {
		listView = s.list.View(st, width, height-2-bh)
	}
	out := info + "\n" + header + "\n" + ui.Fit(listView, width, height-2-bh)
	if bottom != "" {
		out += "\n" + st.Sep(width) + "\n" + bottom
	}
	return out
}
