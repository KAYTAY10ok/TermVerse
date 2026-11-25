// Package arcade implements the "Terminal Arcade" section.
package arcade

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// game is one playable game.
type game interface {
	id() string
	name() string
	desc() string
	start() tea.Cmd
	update(msg tea.Msg) tea.Cmd
	view(w, h int) string
	keys() []ui.Key
	capturing() bool
	pause()
	// scoreIDs lists the score tables of this game (id → label).
	scoreIDs() [][2]string
}

// scoreMsg is emitted by games when a round ends.
type scoreMsg struct {
	table  string
	value  int
	detail string
	lower  bool
}

// exitGameMsg asks the arcade to return to the game menu.
type exitGameMsg struct{}

func exitGame() tea.Msg { return exitGameMsg{} }

func reportScore(table string, value int, detail string, lower bool) tea.Cmd {
	return func() tea.Msg { return scoreMsg{table, value, detail, lower} }
}

// Screen is the arcade hub.
type Screen struct {
	ctx     *ui.Context
	scores  *Scores
	games   []game
	list    ui.List
	active  game
	visible bool
	player  string
	rng     *rand.Rand
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx, scores: LoadScores(ctx.Paths.DataFile("scores.json")),
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(os.Getpid())))}
	s.player = os.Getenv("USER")
	if s.player == "" {
		s.player = "player"
	}
	s.games = []game{
		newSnake(s), newHopper(s), newGuess(s), newMerge(s), newMines(s), newTyping(s), newOnline(s),
	}
	return s
}

func (s *Screen) ID() string    { return "arcade" }
func (s *Screen) Title() string { return ui.T("Terminal Arcade") }
func (s *Screen) Capturing() bool {
	return s.active != nil && s.active.capturing()
}
func (s *Screen) OnLeave() {
	s.visible = false
	if s.active != nil {
		s.active.pause()
	}
}

func (s *Screen) Init() tea.Cmd {
	s.visible = true
	s.list.Vim = s.ctx.Vim()
	s.refresh()
	return nil
}

// OpenSub starts a game by id (snake, hopper, guess, merge, mines, typing, online).
func (s *Screen) OpenSub(sub string) tea.Cmd {
	for i, g := range s.games {
		if g.id() == sub {
			s.list.Cursor = i
			s.active = g
			return g.start()
		}
	}
	return nil
}

func (s *Screen) refresh() {
	items := make([]ui.Item, len(s.games))
	for i, g := range s.games {
		right := ""
		ids := g.scoreIDs()
		if len(ids) > 0 {
			if b, ok := s.scores.Best(ids[0][0]); ok {
				right = ui.Tf("best %d", b.Value)
			}
		}
		items[i] = ui.Item{Title: ui.T(g.name()), Desc: ui.T(g.desc()), Right: right, Value: i}
	}
	s.list.SetItems(items)
}

func (s *Screen) Keys() []ui.Key {
	if s.active != nil {
		return append(s.active.keys(), ui.Key{Keys: "esc", Desc: "menu"})
	}
	return []ui.Key{{Keys: "↑↓", Desc: "select"}, {Keys: "enter", Desc: "play"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case exitGameMsg:
		if s.active != nil {
			s.active.pause()
			s.active = nil
			s.refresh()
		}
		return s, nil
	case scoreMsg:
		rank, err := s.scores.Add(m.table, Score{Value: m.value, Player: s.player, Detail: m.detail, Date: time.Now()}, m.lower)
		s.refresh()
		switch {
		case err != nil:
			return s, ui.Notify(ui.Error, ui.T("Could not save score: ")+err.Error())
		case rank == 1:
			return s, ui.Notify(ui.Success, ui.Tf("🏆 New high score: %d!", m.value))
		case rank > 0:
			return s, ui.Notify(ui.Success, ui.Tf("Score %d — rank #%d", m.value, rank))
		}
		return s, nil
	case tea.KeyMsg:
		if s.active != nil {
			if m.String() == "esc" && !s.active.capturing() {
				s.active.pause()
				s.active = nil
				s.refresh()
				return s, nil
			}
			return s, s.active.update(msg)
		}
		switch m.String() {
		case "esc", "q":
			return s, ui.BackCmd
		case "enter", " ", "right", "l":
			if it, ok := s.list.Selected(); ok {
				s.active = s.games[it.Value.(int)]
				return s, s.active.start()
			}
		default:
			s.list.HandleKey(m)
		}
		return s, nil
	case tea.MouseMsg:
		if s.active == nil {
			s.list.HandleMouse(m)
		}
		return s, nil
	}
	var cmds []tea.Cmd
	for _, g := range s.games {
		cmds = append(cmds, g.update(msg))
	}
	return s, tea.Batch(cmds...)
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	if s.active != nil {
		iw, ih := st.PanelInner(w, h)
		return st.Panel(s.Title()+" › "+ui.T(s.active.name()), s.active.view(iw, ih), w, h, true)
	}
	wide := w >= 90
	leftW := w
	if wide {
		leftW = w / 2
	}
	iw, ih := st.PanelInner(leftW, h)
	left := st.Panel(s.Title(), s.list.View(st, iw, ih), leftW, h, true)
	if !wide {
		return left
	}
	rw := w - leftW - 1
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", st.Panel(ui.T("High scores"), s.scoreView(rw-4), rw, h, false))
}

func (s *Screen) scoreView(w int) string {
	st := s.ctx.S
	it, ok := s.list.Selected()
	if !ok {
		return ""
	}
	g := s.games[it.Value.(int)]
	var b strings.Builder
	b.WriteString(st.Title.Render(ui.T(g.name())) + "\n" + st.Muted.Render(ui.Wrap(ui.T(g.desc()), w)) + "\n\n")
	ids := g.scoreIDs()
	if len(ids) == 0 {
		b.WriteString(st.Muted.Render(ui.T("This game has no score table.")))
	}
	for _, id := range ids {
		b.WriteString(st.Accent.Render(ui.T(id[1])) + "\n")
		list := s.scores.Games[id[0]]
		if len(list) == 0 {
			b.WriteString(st.Muted.Render("  "+ui.T("no score yet")) + "\n\n")
			continue
		}
		for i, sc := range list {
			medal := fmt.Sprintf("%2d.", i+1)
			line := fmt.Sprintf("%s %6d  %-12s %s  %s", medal, sc.Value, ui.Truncate(sc.Player, 12), sc.Date.Format("2006-01-02"), sc.Detail)
			if i == 0 {
				b.WriteString(st.Warning.Render(ui.Truncate(line, w)) + "\n")
			} else {
				b.WriteString(st.Text.Render(ui.Truncate(line, w)) + "\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(st.Muted.Render(ui.T("Controls")) + "\n")
	for _, k := range g.keys() {
		b.WriteString(st.Key.Render(ui.PadRight(k.Keys, 12)) + st.Muted.Render(ui.T(k.Desc)) + "\n")
	}
	return b.String()
}

// center places a block in the middle of w×h.
func center(block string, w, h int) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

func dirKey(k string) (Point, bool) {
	switch k {
	case "up", "w", "k", "z":
		return Point{0, -1}, true
	case "down", "s", "j":
		return Point{0, 1}, true
	case "left", "a", "h", "q":
		return Point{-1, 0}, true
	case "right", "d", "l":
		return Point{1, 0}, true
	}
	return Point{}, false
}
