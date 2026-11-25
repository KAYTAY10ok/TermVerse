package arcade

import (
	"fmt"
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/modules/arcade/online"
)

type netMsg struct {
	gen int
	m   online.Msg
}
type netClosed struct{ gen int }
type connectedMsg struct {
	gen    int
	client *online.Client
	server *online.Server
	err    error
}

type chatLine struct {
	at         time.Time
	from, text string
	kind       string // chat, sys, error
}

type onlineGame struct {
	a        *Screen
	opts     ui.Options
	prompt   ui.Prompt
	confirm  ui.Confirm
	nick     string
	client   *online.Client
	server   *online.Server
	gen      int
	busy     bool
	err      string
	log      []chatLine
	roster   []string
	game     *online.GameState
	input    ui.Prompt
	focus    int // 0 chat, 1 board
	col      int
	chatView ui.Scroller
	hostAddr string
}

func newOnline(a *Screen) *onlineGame {
	g := &onlineGame{a: a, nick: online.CleanName(a.player), col: 3}
	g.opts.Items = []ui.Option{
		{Label: "Your name", Value: func() string { return g.nick }, Enter: func() tea.Cmd {
			return g.prompt.Open(a.ctx.S, "Your name in the room", "neo", g.nick, func(v string) tea.Cmd {
				g.nick = online.CleanName(v)
				return nil
			})
		}},
		{Label: "Host a room", Enter: g.askHost},
		{Label: "Join a room", Enter: g.askJoin},
	}
	return g
}

func (g *onlineGame) id() string    { return "online" }
func (g *onlineGame) name_() string { return g.nick }
func (g *onlineGame) name() string  { return "Online Room" }
func (g *onlineGame) desc() string {
	return "Chat with other TermVerse users on your network and play Four in a Row."
}
func (g *onlineGame) scoreIDs() [][2]string { return nil }
func (g *onlineGame) capturing() bool {
	return g.prompt.Active || (g.client != nil && g.focus == 0)
}
func (g *onlineGame) pause() {}
func (g *onlineGame) keys() []ui.Key {
	if g.client == nil {
		return []ui.Key{{Keys: "↑↓", Desc: "select"}, {Keys: "enter", Desc: "choose"}}
	}
	if g.focus == 0 {
		return []ui.Key{{Keys: "enter", Desc: "send"}, {Keys: "tab", Desc: "board"}, {Keys: "pgup/pgdn", Desc: "scroll"}, {Keys: "esc", Desc: "leave room"}}
	}
	return []ui.Key{{Keys: "←→", Desc: "column"}, {Keys: "enter", Desc: "drop"}, {Keys: "j", Desc: "sit/stand"}, {Keys: "n", Desc: "new game"}, {Keys: "tab", Desc: "chat"}, {Keys: "esc", Desc: "menu (stay connected)"}}
}

func (g *onlineGame) start() tea.Cmd {
	g.opts.Vim = g.a.ctx.Vim()
	if g.client != nil && g.focus == 0 {
		return g.input.Input.Focus()
	}
	return nil
}

func (g *onlineGame) askHost() tea.Cmd {
	st := g.a.ctx.S
	return g.prompt.Open(st, "Port to listen on (others connect to your-IP:port)", online.DefaultPort, online.DefaultPort, func(port string) tea.Cmd {
		port = strings.TrimSpace(port)
		if port == "" {
			port = online.DefaultPort
		}
		return g.prompt.Open(st, "Room password (optional, recommended)", "", "", func(pass string) tea.Cmd {
			g.busy, g.err = true, ""
			g.gen++
			gen, name := g.gen, g.nick
			return func() tea.Msg {
				srv, err := online.Listen(net.JoinHostPort("", port), pass)
				if err != nil {
					return connectedMsg{gen: gen, err: err}
				}
				_, p, _ := net.SplitHostPort(srv.Addr().String())
				c, err := online.Dial(net.JoinHostPort("127.0.0.1", p), name, pass)
				if err != nil {
					srv.Close()
					return connectedMsg{gen: gen, err: err}
				}
				return connectedMsg{gen: gen, client: c, server: srv}
			}
		})
	})
}

func (g *onlineGame) askJoin() tea.Cmd {
	st := g.a.ctx.S
	return g.prompt.Open(st, "Room address (host:port)", "192.168.1.20:7777", "", func(addr string) tea.Cmd {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return nil
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, online.DefaultPort)
		}
		return g.prompt.Open(st, "Room password (empty if none)", "", "", func(pass string) tea.Cmd {
			g.busy, g.err = true, ""
			g.gen++
			gen, name := g.gen, g.nick
			return func() tea.Msg {
				c, err := online.Dial(addr, name, pass)
				return connectedMsg{gen: gen, client: c, err: err}
			}
		})
	})
}

func (g *onlineGame) listen() tea.Cmd {
	c, gen := g.client, g.gen
	return func() tea.Msg {
		m, ok := <-c.Incoming
		if !ok {
			return netClosed{gen}
		}
		return netMsg{gen, m}
	}
}

func (g *onlineGame) disconnect() {
	if g.client != nil {
		g.client.Close()
	}
	if g.server != nil {
		g.server.Close()
	}
	g.client, g.server, g.game, g.roster = nil, nil, nil, nil
	g.gen++
}

func (g *onlineGame) add(kind, from, text string) {
	g.log = append(g.log, chatLine{at: time.Now(), from: from, text: text, kind: kind})
	if len(g.log) > 500 {
		g.log = g.log[len(g.log)-500:]
	}
	g.chatView.Bottom()
}

func (g *onlineGame) openInput() tea.Cmd {
	return g.input.Open(g.a.ctx.S, "", "Say something…", "", func(v string) tea.Cmd {
		var cmd tea.Cmd
		if t := online.CleanText(v, online.MaxChat); t != "" && g.client != nil {
			if err := g.client.Send(online.Msg{T: "chat", Text: t}); err != nil {
				g.add("error", "", err.Error())
			}
		}
		if g.client != nil && g.focus == 0 {
			cmd = g.openInput()
		}
		return cmd
	})
}

func (g *onlineGame) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case connectedMsg:
		if m.gen != g.gen {
			if m.client != nil {
				m.client.Close()
			}
			if m.server != nil {
				m.server.Close()
			}
			return nil
		}
		g.busy = false
		if m.err != nil {
			g.err = m.err.Error()
			return nil
		}
		g.client, g.server, g.log, g.focus = m.client, m.server, nil, 0
		g.nick = m.client.Name
		if m.server != nil {
			_, port, _ := net.SplitHostPort(m.server.Addr().String())
			g.hostAddr = port
			g.add("sys", "", ui.Tf("Room open on port %s. Others can join with one of: %s", port, joinAddrs(port)))
		} else {
			g.add("sys", "", ui.T("Connected. Say hello!"))
		}
		return tea.Batch(g.listen(), g.openInput())
	case netMsg:
		if m.gen != g.gen || g.client == nil {
			return nil
		}
		switch m.m.T {
		case "chat":
			g.add("chat", online.CleanName(m.m.From), online.CleanText(m.m.Text, online.MaxChat))
			if !g.a.visible || g.a.active != g {
				return tea.Batch(g.listen(), ui.Notify(ui.Info, ui.Tf("💬 %s: %s", m.m.From, online.CleanText(m.m.Text, 60))))
			}
		case "sys":
			if m.m.Text != "" {
				g.add("sys", "", online.CleanText(m.m.Text, online.MaxChat))
			}
		case "error":
			g.add("error", "", online.CleanText(m.m.Text, online.MaxChat))
		case "roster":
			g.roster = m.m.Names
		case "game":
			g.game = m.m.Game
		}
		return g.listen()
	case netClosed:
		if m.gen != g.gen {
			return nil
		}
		g.disconnect()
		g.err = ui.T("Disconnected from the room.")
		return ui.Notify(ui.Warn, g.err)
	case tea.KeyMsg:
		return g.key(m)
	}
	if g.input.Active {
		return g.input.Update(msg)
	}
	if g.prompt.Active {
		return g.prompt.Update(msg)
	}
	return nil
}

func joinAddrs(port string) string {
	ips := online.LANAddresses()
	if len(ips) == 0 {
		return "127.0.0.1:" + port
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip + ":" + port
	}
	return strings.Join(out, ", ")
}

func (g *onlineGame) key(m tea.KeyMsg) tea.Cmd {
	k := m.String()
	if g.prompt.Active {
		return g.prompt.Update(m)
	}
	if g.confirm.Active {
		return g.confirm.Update(m)
	}
	if g.client == nil {
		if g.busy {
			return nil
		}
		_, cmd := g.opts.HandleKey(m)
		return cmd
	}
	switch k {
	case "esc":
		g.input.Active = false
		q := ui.T("Leave the room?")
		if g.server != nil {
			q = ui.T("Close your room? Everyone will be disconnected.")
		}
		g.confirm.Ask(q, func() tea.Cmd {
			g.disconnect()
			g.log = nil
			return ui.Notify(ui.Info, ui.T("You left the room"))
		})
		return nil
	case "tab":
		g.focus = 1 - g.focus
		if g.focus == 0 {
			return g.openInput()
		}
		g.input.Active = false
		return nil
	case "pgup", "pgdown":
		g.chatView.HandleKey(m)
		return nil
	}
	if g.focus == 0 {
		if !g.input.Active {
			return g.openInput()
		}
		return g.input.Update(m)
	}
	send := func(msg online.Msg) tea.Cmd {
		if err := g.client.Send(msg); err != nil {
			g.add("error", "", err.Error())
		}
		return nil
	}
	switch k {
	case "left", "h", "a", "q":
		g.col = (g.col + online.Cols - 1) % online.Cols
	case "right", "l", "d":
		g.col = (g.col + 1) % online.Cols
	case "enter", " ", "down", "s":
		return send(online.Msg{T: "move", Col: g.col})
	case "j":
		if g.game != nil && (g.game.Players[0] == g.nick || g.game.Players[1] == g.nick) {
			return send(online.Msg{T: "leave"})
		}
		return send(online.Msg{T: "join"})
	case "n":
		return send(online.Msg{T: "reset"})
	default:
		if n := int(k[0] - '1'); len(k) == 1 && n >= 0 && n < online.Cols {
			g.col = n
			return send(online.Msg{T: "move", Col: n})
		}
	}
	return nil
}

func (g *onlineGame) view(w, h int) string {
	st := g.a.ctx.S
	if g.client == nil {
		var b strings.Builder
		b.WriteString(st.Title.Render(ui.T("Online Room")) + "\n")
		b.WriteString(st.Muted.Render(ui.Wrap(ui.T("Host a room on this machine or join a friend's room. Works on a local network, or over the Internet if the host forwards the port. Messages are not encrypted."), w)) + "\n\n")
		b.WriteString(ui.Fit(g.opts.View(st, w, 3), w, 3) + "\n\n")
		switch {
		case g.prompt.Active:
			b.WriteString(g.prompt.View(st, w))
		case g.busy:
			b.WriteString(st.Secondary.Render(ui.T("Connecting…")))
		case g.err != "":
			b.WriteString(st.Error.Render(g.err))
		}
		return b.String()
	}
	boardW := online.Cols*4 + 4
	chatW := w - boardW - 3
	if chatW < 30 {
		chatW, boardW = w, 0
	}
	// Chat column
	var lines []string
	for _, l := range g.log {
		ts := st.Muted.Render(l.at.Format("15:04") + " ")
		switch l.kind {
		case "chat":
			nameStyle := st.Secondary
			if l.from == g.nick {
				nameStyle = st.Accent
			}
			lines = append(lines, ui.Wrap(ts+nameStyle.Bold(true).Render(l.from)+st.Muted.Render(": ")+st.Text.Render(l.text), chatW))
		case "sys":
			lines = append(lines, ui.Wrap(ts+st.Muted.Italic(true).Render("• "+l.text), chatW))
		default:
			lines = append(lines, ui.Wrap(ts+st.Error.Render("! "+l.text), chatW))
		}
	}
	g.chatView.SetContent(strings.Join(lines, "\n"))
	bottom := ""
	switch {
	case g.confirm.Active:
		bottom = g.confirm.View(st)
	case g.focus == 0 && g.input.Active:
		g.input.Input.Width = chatW - 4
		bottom = g.input.Input.View()
	default:
		bottom = st.Muted.Render(ui.T("tab: back to chat"))
	}
	bh := strings.Count(bottom, "\n") + 1
	who := st.Muted.Render(ui.Tf("%d online: %s", len(g.roster), strings.Join(g.roster, ", ")))
	chat := ui.Truncate(who, chatW) + "\n" + st.Sep(chatW) + "\n" + ui.Fit(g.chatView.View(st, chatW, h-bh-3), chatW, h-bh-3) + "\n" + st.Sep(chatW) + "\n" + bottom
	if boardW == 0 {
		return chat
	}
	return joinCols2(chat, g.boardView(boardW, h), chatW, boardW, h)
}

func joinCols2(a, b string, wa, wb, h int) string {
	al := strings.Split(ui.Fit(a, wa, h), "\n")
	bl := strings.Split(ui.Fit(b, wb, h), "\n")
	out := make([]string, h)
	for i := range out {
		out[i] = al[i] + " │ " + bl[i]
	}
	return strings.Join(out, "\n")
}

func (g *onlineGame) boardView(w, h int) string {
	st := g.a.ctx.S
	var b strings.Builder
	title := st.Title.Render(ui.T("Four in a Row"))
	if g.focus == 1 {
		title = st.Selected.Render(" " + ui.T("Four in a Row") + " ")
	}
	b.WriteString(title + "\n\n")
	gs := g.game
	if gs == nil {
		return b.String() + st.Muted.Render(ui.T("Waiting for game state…"))
	}
	discs := []string{st.Muted.Render(" · "), st.Error.Render(" ● "), st.Warning.Render(" ● ")}
	for i, p := range gs.Players {
		name := p
		if name == "" {
			name = st.Muted.Render(ui.T("(free seat)"))
		}
		turn := "  "
		if gs.Winner == 0 && gs.Turn == i+1 && p != "" {
			turn = st.Accent.Render("▶ ")
		}
		b.WriteString(turn + discs[i+1] + st.Text.Render(name) + st.Muted.Render(fmt.Sprintf("  (%d)", gs.Score[i])) + "\n")
	}
	b.WriteString("\n")
	arrow := strings.Repeat("    ", g.col) + "  " + st.Accent.Render("▼")
	if g.focus == 1 {
		b.WriteString(arrow + "\n")
	} else {
		b.WriteString("\n")
	}
	for r := 0; r < online.Rows; r++ {
		b.WriteString(st.Muted.Render("│"))
		for c := 0; c < online.Cols; c++ {
			b.WriteString(discs[gs.Board[r][c]] + st.Muted.Render("│"))
		}
		b.WriteString("\n")
	}
	b.WriteString(st.Muted.Render("└"+strings.Repeat("───┴", online.Cols-1)+"───┘") + "\n")
	b.WriteString(st.Muted.Render("  1   2   3   4   5   6   7") + "\n\n")
	switch {
	case gs.Winner == 3:
		b.WriteString(st.Warning.Render(ui.T("Draw! Press n for a rematch.")))
	case gs.Winner > 0:
		b.WriteString(st.Success.Bold(true).Render(ui.Tf("%s wins! Press n for a rematch.", gs.Players[gs.Winner-1])))
	case gs.Players[0] == "" || gs.Players[1] == "":
		b.WriteString(st.Muted.Render(ui.Wrap(ui.T("Press tab then j to take a seat. The game starts with two players."), w)))
	case gs.Players[gs.Turn-1] == g.nick:
		b.WriteString(st.Accent.Render(ui.T("Your turn!")))
	}
	return b.String()
}
