package arcade

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

type tickMsg struct {
	game string
	gen  int
}

func tick(game string, gen int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{game, gen} })
}

// ---- Snake -------------------------------------------------------------------

type snakeGame struct {
	a       *Screen
	st      *SnakeState
	gen     int
	paused  bool
	w, h    int
	started bool
}

func newSnake(a *Screen) *snakeGame { return &snakeGame{a: a, w: 30, h: 16} }
func (g *snakeGame) id() string     { return "snake" }
func (g *snakeGame) name() string   { return "Snake" }
func (g *snakeGame) desc() string {
	return "Eat, grow, don't bite yourself. Speed increases as you grow."
}
func (g *snakeGame) capturing() bool {
	return false
}
func (g *snakeGame) scoreIDs() [][2]string { return [][2]string{{"snake", "Snake"}} }
func (g *snakeGame) keys() []ui.Key {
	return []ui.Key{{Keys: "←↑→↓/wasd", Desc: "move"}, {Keys: "p", Desc: "pause"}, {Keys: "r", Desc: "restart"}}
}
func (g *snakeGame) pause() {
	if g.st != nil && !g.st.Over {
		g.paused = true
		g.gen++
	}
}
func (g *snakeGame) start() tea.Cmd {
	if g.st != nil && !g.st.Over && g.paused { // resume a paused game
		g.paused = false
		g.gen++
		return tick("snake", g.gen, g.speed())
	}
	g.st = NewSnake(g.w, g.h, g.a.rng)
	g.paused, g.started = false, true
	g.gen++
	return tick("snake", g.gen, g.speed())
}
func (g *snakeGame) speed() time.Duration {
	d := 150*time.Millisecond - time.Duration(len(g.st.Body))*3*time.Millisecond
	return max(55*time.Millisecond, d)
}
func (g *snakeGame) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tickMsg:
		if m.game != "snake" || m.gen != g.gen || g.paused || g.st == nil || g.st.Over {
			return nil
		}
		g.st.Step()
		if g.st.Over {
			return reportScore("snake", g.st.Score, fmt.Sprintf("len %d", len(g.st.Body)), false)
		}
		return tick("snake", g.gen, g.speed())
	case tea.KeyMsg:
		k := m.String()
		switch k {
		case "p", " ":
			if g.st.Over {
				return g.start()
			}
			g.paused = !g.paused
			g.gen++
			if !g.paused {
				return tick("snake", g.gen, g.speed())
			}
		case "r", "enter":
			g.st = nil
			return g.start()
		default:
			if d, ok := dirKey(k); ok && !g.paused {
				g.st.Turn(d)
			}
		}
	}
	return nil
}
func (g *snakeGame) view(w, h int) string {
	st := g.a.ctx.S
	// Remember the available size for the next game.
	g.w, g.h = max(10, min(40, (w-2)/2)), max(8, min(22, h-4))
	s := g.st
	if s == nil {
		return ""
	}
	cells := make([][]string, s.H)
	for y := range cells {
		cells[y] = make([]string, s.W)
		for x := range cells[y] {
			cells[y][x] = "  "
		}
	}
	food := st.Error.Render("●")
	cells[s.Food.Y][s.Food.X] = food + " "
	for i, p := range s.Body {
		if i == 0 {
			cells[p.Y][p.X] = st.Accent.Render("██")
		} else if i%2 == 0 {
			cells[p.Y][p.X] = st.Primary.Render("██")
		} else {
			cells[p.Y][p.X] = st.Secondary.Render("██")
		}
	}
	bs := lipgloss.NewStyle().Foreground(st.BorderColor)
	var b strings.Builder
	b.WriteString(bs.Render("┌"+strings.Repeat("─", s.W*2)+"┐") + "\n")
	for _, row := range cells {
		b.WriteString(bs.Render("│") + strings.Join(row, "") + bs.Render("│") + "\n")
	}
	b.WriteString(bs.Render("└" + strings.Repeat("─", s.W*2) + "┘"))
	status := st.Text.Render(ui.Tf("Score %d · length %d", s.Score, len(s.Body)))
	if best, ok := g.a.scores.Best("snake"); ok {
		status += st.Muted.Render(ui.Tf(" · best %d", best.Value))
	}
	switch {
	case s.Over:
		status += "   " + st.Error.Bold(true).Render(ui.T("GAME OVER")) + st.Muted.Render(" — "+ui.T("r to restart"))
	case g.paused:
		status += "   " + st.Warning.Render(ui.T("PAUSED"))
	}
	return center(status+"\n"+b.String(), w, h)
}

// ---- Sky Hopper --------------------------------------------------------------

type pillar struct {
	x      float64
	gap    int
	passed bool
}

type hopperGame struct {
	a       *Screen
	gen     int
	w, h    int
	fw, fh  int
	y, vy   float64
	pillars []pillar
	score   int
	over    bool
	paused  bool
	waiting bool
	frame   int
}

const hopperX = 8

func newHopper(a *Screen) *hopperGame { return &hopperGame{a: a, fw: 56, fh: 18} }
func (g *hopperGame) id() string      { return "hopper" }
func (g *hopperGame) name() string    { return "Sky Hopper" }
func (g *hopperGame) desc() string {
	return "Flap through the gaps between the pillars. One key, endless sky."
}
func (g *hopperGame) capturing() bool       { return false }
func (g *hopperGame) scoreIDs() [][2]string { return [][2]string{{"hopper", "Sky Hopper"}} }
func (g *hopperGame) keys() []ui.Key {
	return []ui.Key{{Keys: "space/↑", Desc: "flap"}, {Keys: "p", Desc: "pause"}, {Keys: "r", Desc: "restart"}}
}
func (g *hopperGame) pause() {
	if !g.over && !g.waiting {
		g.paused = true
		g.gen++
	}
}
func (g *hopperGame) start() tea.Cmd {
	if g.paused && !g.over {
		g.paused = false
		g.gen++
		return tick("hopper", g.gen, 50*time.Millisecond)
	}
	g.fw, g.fh = g.w, g.h
	g.y, g.vy = float64(g.fh)/2, 0
	g.pillars = nil
	g.score, g.over, g.paused, g.waiting, g.frame = 0, false, false, true, 0
	g.gen++
	return nil
}
func (g *hopperGame) gapSize() int { return max(4, 7-g.score/10) }
func (g *hopperGame) spawn(x float64) {
	gs := g.gapSize()
	g.pillars = append(g.pillars, pillar{x: x, gap: 1 + g.a.rng.IntN(max(1, g.fh-gs-2))})
}
func (g *hopperGame) step() {
	g.frame++
	g.vy = min(g.vy+0.22, 1.2)
	g.y += g.vy
	for i := range g.pillars {
		g.pillars[i].x -= 0.6
		if !g.pillars[i].passed && g.pillars[i].x+2 < hopperX {
			g.pillars[i].passed = true
			g.score++
		}
	}
	if len(g.pillars) > 0 && g.pillars[0].x < -3 {
		g.pillars = g.pillars[1:]
	}
	if len(g.pillars) == 0 || g.pillars[len(g.pillars)-1].x < float64(g.fw)-22 {
		g.spawn(float64(g.fw))
	}
	row := int(g.y)
	if row < 0 || row >= g.fh {
		g.over = true
		return
	}
	for _, p := range g.pillars {
		px := int(p.x)
		if hopperX+1 >= px && hopperX <= px+2 && (row < p.gap || row >= p.gap+g.gapSize()) {
			g.over = true
		}
	}
}
func (g *hopperGame) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tickMsg:
		if m.game != "hopper" || m.gen != g.gen || g.paused || g.over || g.waiting {
			return nil
		}
		g.step()
		if g.over {
			return reportScore("hopper", g.score, "", false)
		}
		return tick("hopper", g.gen, 50*time.Millisecond)
	case tea.KeyMsg:
		switch m.String() {
		case " ", "up", "w", "k", "z", "enter":
			if g.over {
				return g.start()
			}
			if g.paused {
				return nil
			}
			g.vy = -1.25
			if g.waiting {
				g.waiting = false
				g.gen++
				return tick("hopper", g.gen, 50*time.Millisecond)
			}
		case "p":
			if g.over || g.waiting {
				return nil
			}
			g.paused = !g.paused
			g.gen++
			if !g.paused {
				return tick("hopper", g.gen, 50*time.Millisecond)
			}
		case "r":
			g.paused = false
			g.over = true
			return g.start()
		}
	}
	return nil
}
func (g *hopperGame) view(w, h int) string {
	st := g.a.ctx.S
	g.w, g.h = max(30, min(80, w-2)), max(10, min(22, h-4))
	if g.fw == 0 {
		return ""
	}
	grid := make([][]string, g.fh)
	for y := range grid {
		grid[y] = make([]string, g.fw)
		for x := range grid[y] {
			grid[y][x] = " "
			if (x*7+y*13+g.frame/6)%53 == 0 { // drifting stars
				grid[y][x] = st.Muted.Render("·")
			}
		}
	}
	gs := g.gapSize()
	for _, p := range g.pillars {
		for dx := 0; dx < 3; dx++ {
			x := int(p.x) + dx
			if x < 0 || x >= g.fw {
				continue
			}
			for y := 0; y < g.fh; y++ {
				if y < p.gap || y >= p.gap+gs {
					ch := "█"
					if y == p.gap-1 || y == p.gap+gs {
						ch = "▓"
					}
					grid[y][x] = st.Success.Render(ch)
				}
			}
		}
	}
	if row := int(g.y); row >= 0 && row < g.fh {
		body := "◢"
		if g.vy < 0 {
			body = "◥"
		}
		grid[row][hopperX] = st.Warning.Render(body)
		grid[row][hopperX+1] = st.Accent.Render("▶")
	}
	bs := lipgloss.NewStyle().Foreground(st.BorderColor)
	var b strings.Builder
	b.WriteString(bs.Render("┌"+strings.Repeat("─", g.fw)+"┐") + "\n")
	for _, r := range grid {
		b.WriteString(bs.Render("│") + strings.Join(r, "") + bs.Render("│") + "\n")
	}
	b.WriteString(bs.Render("└" + strings.Repeat("▔", g.fw) + "┘"))
	status := st.Text.Render(ui.Tf("Score %d", g.score))
	if best, ok := g.a.scores.Best("hopper"); ok {
		status += st.Muted.Render(ui.Tf(" · best %d", best.Value))
	}
	switch {
	case g.over:
		status += "   " + st.Error.Bold(true).Render(ui.T("CRASH!")) + st.Muted.Render(" — "+ui.T("space to retry"))
	case g.waiting:
		status += "   " + st.Accent.Render(ui.T("Press space to take off"))
	case g.paused:
		status += "   " + st.Warning.Render(ui.T("PAUSED"))
	}
	return center(status+"\n"+b.String(), w, h)
}
