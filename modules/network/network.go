// Package network implements the "Network Tools" section.
package network

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/KAYTAY10ok/TermVerse/internal/netinfo"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

var tabs = []string{"Interfaces", "DNS lookup", "TCP test", "Connections"}

type ifMsg struct {
	ifs []netinfo.Interface
	err error
}
type dnsMsg netinfo.DNSResult
type tcpMsg struct {
	target string
	probes []netinfo.Probe
}
type connMsg struct {
	conns []netinfo.Conn
	stats netinfo.ConnStats
	err   error
}

// Screen is the network tools screen.
type Screen struct {
	ctx    *ui.Context
	tab    int
	scroll ui.Scroller
	input  ui.Prompt
	busy   bool

	ifs    []netinfo.Interface
	ifErr  error
	dns    *netinfo.DNSResult
	tcp    *tcpMsg
	conns  *connMsg
	tcpErr string
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen { return &Screen{ctx: ctx} }

func (s *Screen) ID() string      { return "network" }
func (s *Screen) Title() string   { return ui.T("Network Tools") }
func (s *Screen) Capturing() bool { return s.input.Active }

func (s *Screen) Init() tea.Cmd {
	s.scroll.Vim = s.ctx.Vim()
	return s.loadIfs()
}

func (s *Screen) Keys() []ui.Key {
	if s.input.Active {
		return []ui.Key{{Keys: "enter", Desc: "run"}, {Keys: "esc", Desc: "cancel"}}
	}
	k := []ui.Key{{Keys: "tab", Desc: "switch tab"}, {Keys: "↑↓", Desc: "scroll"}}
	switch s.tab {
	case 0, 3:
		k = append(k, ui.Key{Keys: "r", Desc: "refresh"})
	case 1, 2:
		k = append(k, ui.Key{Keys: "enter", Desc: "new query"})
	}
	return append(k, ui.Key{Keys: "esc", Desc: "back"})
}

func (s *Screen) loadIfs() tea.Cmd {
	return func() tea.Msg { ifs, err := netinfo.Interfaces(); return ifMsg{ifs, err} }
}

func (s *Screen) loadConns() tea.Cmd {
	s.busy = true
	return func() tea.Msg { c, st, err := netinfo.Connections(); return connMsg{c, st, err} }
}

func (s *Screen) ask() tea.Cmd {
	switch s.tab {
	case 1:
		prev := ""
		if s.dns != nil {
			prev = s.dns.Query
		}
		return s.input.Open(s.ctx.S, "Host name or IP address to resolve", "example.com", prev, func(v string) tea.Cmd {
			s.busy = true
			return func() tea.Msg { return dnsMsg(netinfo.Lookup(v)) }
		})
	case 2:
		prev := ""
		if s.tcp != nil {
			prev = s.tcp.target
		}
		return s.input.Open(s.ctx.S, "Target to test (host:port)", "example.com:443", prev, func(v string) tea.Cmd {
			addr, err := netinfo.ParseTarget(v)
			if err != nil {
				s.tcpErr = err.Error()
				return nil
			}
			s.tcpErr, s.busy = "", true
			return func() tea.Msg { return tcpMsg{addr, netinfo.TCPTest(addr, 3, 5*time.Second)} }
		})
	}
	return nil
}

func (s *Screen) switchTab(t int) tea.Cmd {
	s.tab = t
	s.scroll.Top()
	switch t {
	case 0:
		return s.loadIfs()
	case 1:
		if s.dns == nil {
			return s.ask()
		}
	case 2:
		if s.tcp == nil {
			return s.ask()
		}
	case 3:
		if s.conns == nil {
			return s.loadConns()
		}
	}
	return nil
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case ifMsg:
		s.ifs, s.ifErr = msg.ifs, msg.err
	case dnsMsg:
		r := netinfo.DNSResult(msg)
		s.dns, s.busy = &r, false
	case tcpMsg:
		m := msg
		s.tcp, s.busy = &m, false
	case connMsg:
		m := msg
		s.conns, s.busy = &m, false
	case tea.MouseMsg:
		s.scroll.HandleMouse(msg)
	case tea.KeyMsg:
		if s.input.Active {
			return s, s.input.Update(msg)
		}
		switch msg.String() {
		case "esc", "q":
			return s, ui.BackCmd
		case "tab":
			return s, s.switchTab((s.tab + 1) % len(tabs))
		case "shift+tab":
			return s, s.switchTab((s.tab + len(tabs) - 1) % len(tabs))
		case "1", "2", "3", "4":
			return s, s.switchTab(int(msg.String()[0] - '1'))
		case "enter", "n", "/":
			return s, s.ask()
		case "r":
			if s.tab == 3 {
				return s, s.loadConns()
			}
			return s, s.loadIfs()
		default:
			s.scroll.HandleKey(msg)
		}
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	iw, ih := st.PanelInner(w, h)
	var content string
	switch s.tab {
	case 0:
		content = s.ifView()
	case 1:
		content = s.dnsView(iw)
	case 2:
		content = s.tcpView()
	case 3:
		content = s.connView(iw)
	}
	if s.busy {
		content = st.Secondary.Render(ui.T("Working…")) + "\n\n" + content
	}
	bottom := ""
	if s.input.Active {
		bottom = s.input.View(st, iw)
	}
	bh := 0
	if bottom != "" {
		bh = strings.Count(bottom, "\n") + 2
	}
	s.scroll.SetContent(content)
	body := st.Tabs(tabs, s.tab) + "\n" + st.Sep(iw) + "\n" + s.scroll.View(st, iw, ih-2-bh)
	if bottom != "" {
		body += "\n" + st.Sep(iw) + "\n" + bottom
	}
	return st.Panel(s.Title(), body, w, h, true)
}

func (s *Screen) ifView() string {
	st := s.ctx.S
	if s.ifErr != nil {
		return st.Error.Render(s.ifErr.Error())
	}
	if s.ifs == nil {
		return st.Muted.Render(ui.T("Loading…"))
	}
	var b strings.Builder
	b.WriteString(st.KV([][2]string{{ui.T("Primary local IP"), system.LocalIP()}}, 17) + "\n\n")
	for _, i := range s.ifs {
		state := st.Success.Render("● " + ui.T("up"))
		if !i.Up {
			state = st.Error.Render("● " + ui.T("down"))
		}
		kind := ""
		if i.Loopback {
			kind = st.Muted.Render(" (loopback)")
		}
		b.WriteString(st.Title.Render(i.Name) + "  " + state + kind + "\n")
		rows := [][2]string{{"MAC", orNA(i.MAC)}, {"MTU", fmt.Sprint(i.MTU)}}
		if i.HasCounters {
			rows = append(rows, [2]string{ui.T("Traffic"), fmt.Sprintf("↑ %s   ↓ %s", system.HumanBytes(i.BytesSent), system.HumanBytes(i.Recv))})
		}
		for _, a := range i.Addrs {
			rows = append(rows, [2]string{ui.T("Address"), a})
		}
		b.WriteString(indent(st.KV(rows, 9), "  ") + "\n\n")
	}
	return b.String()
}

func (s *Screen) dnsView(width int) string {
	st := s.ctx.S
	if s.dns == nil {
		return st.Muted.Render(ui.T("Press Enter to resolve a host name."))
	}
	r := s.dns
	var b strings.Builder
	b.WriteString(st.Title.Render(r.Query) + st.Muted.Render(fmt.Sprintf("  (%s)", r.Duration.Round(time.Millisecond))) + "\n\n")
	if r.Err != nil {
		b.WriteString(st.Error.Render(ui.T("Lookup failed: ") + r.Err.Error()))
		return b.String()
	}
	sect := func(name string, vals []string) {
		if len(vals) == 0 {
			return
		}
		b.WriteString(st.Accent.Render(name) + "\n")
		for _, v := range vals {
			b.WriteString("  " + st.Text.Render(ui.Truncate(ui.Sanitize(v), width-2)) + "\n")
		}
		b.WriteString("\n")
	}
	sect(ui.T("Addresses"), r.IPs)
	if r.CNAME != "" {
		sect("CNAME", []string{r.CNAME})
	}
	sect("MX", r.MX)
	sect("NS", r.NS)
	sect("TXT", r.TXT)
	sect(ui.T("Reverse (PTR)"), r.PTR)
	return b.String()
}

func (s *Screen) tcpView() string {
	st := s.ctx.S
	var b strings.Builder
	if s.tcpErr != "" {
		b.WriteString(st.Error.Render(s.tcpErr) + "\n\n")
	}
	if s.tcp == nil {
		b.WriteString(st.Muted.Render(ui.T("Press Enter to test a TCP connection (3 attempts, 5s timeout).")))
		return b.String()
	}
	b.WriteString(st.Title.Render(s.tcp.target) + "\n\n")
	okN := 0
	var total time.Duration
	for i, p := range s.tcp.probes {
		if p.Err != nil {
			b.WriteString(fmt.Sprintf("  #%d  ", i+1) + st.Error.Render("✗ "+p.Err.Error()) + "\n")
			continue
		}
		okN++
		total += p.Latency
		b.WriteString(fmt.Sprintf("  #%d  ", i+1) + st.Success.Render("✔ "+ui.T("connected")) +
			st.Text.Render(fmt.Sprintf("  %s  %s", p.Latency.Round(time.Microsecond*100), p.Remote)) + "\n")
	}
	b.WriteString("\n")
	if okN > 0 {
		b.WriteString(st.Success.Render(ui.Tf("%d/%d successful · average %s", okN, len(s.tcp.probes), (total / time.Duration(okN)).Round(time.Microsecond*100))))
	} else {
		b.WriteString(st.Error.Render(ui.T("Target unreachable")))
	}
	return b.String()
}

func (s *Screen) connView(width int) string {
	st := s.ctx.S
	if s.conns == nil {
		return st.Muted.Render(ui.T("Loading…"))
	}
	if s.conns.err != nil {
		return st.Error.Render(s.conns.err.Error())
	}
	var b strings.Builder
	stats := s.conns.stats
	b.WriteString(st.Title.Render(ui.Tf("%d sockets", stats.Total)) + "\n")
	b.WriteString(st.Muted.Render(summary(stats.ByProto)) + "\n" + st.Muted.Render(summary(stats.ByStatus)) + "\n\n")
	b.WriteString(st.Bold.Render(fmt.Sprintf("%-5s %-28s %-28s %-12s %s", "PROTO", ui.T("LOCAL"), ui.T("REMOTE"), ui.T("STATE"), ui.T("PROCESS"))) + "\n")
	for _, c := range s.conns.conns {
		proc := "-"
		if c.PID > 0 {
			proc = fmt.Sprintf("%s (%d)", c.Process, c.PID)
		}
		b.WriteString(st.Text.Render(ui.Truncate(fmt.Sprintf("%-5s %-28s %-28s %-12s %s", c.Proto, c.Local, c.Remote, c.Status, proc), width)) + "\n")
	}
	b.WriteString("\n" + st.Muted.Render(ui.T("Processes owned by other users are only visible with elevated privileges.")))
	return b.String()
}

func summary(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %d", k, m[k]))
	}
	return strings.Join(parts, " · ")
}

func orNA(s string) string {
	if s == "" {
		return system.NA
	}
	return s
}

func indent(s, p string) string { return p + strings.ReplaceAll(s, "\n", "\n"+p) }
