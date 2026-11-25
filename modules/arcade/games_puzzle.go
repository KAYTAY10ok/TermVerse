package arcade

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// ---- Number Guess ------------------------------------------------------------

type guessGame struct {
	a       *Screen
	in      textinput.Model
	max     int
	secret  int
	tries   []int
	lo, hi  int
	won     bool
	message string
}

func newGuess(a *Screen) *guessGame  { return &guessGame{a: a, max: 100} }
func (g *guessGame) id() string      { return "guess" }
func (g *guessGame) name() string    { return "Number Guess" }
func (g *guessGame) desc() string    { return "Find the secret number with as few tries as possible." }
func (g *guessGame) capturing() bool { return !g.won }
func (g *guessGame) pause()          {}
func (g *guessGame) scoreIDs() [][2]string {
	return [][2]string{{"guess", "1 – 100 (tries)"}, {"guess1000", "1 – 1000 (tries)"}}
}
func (g *guessGame) keys() []ui.Key {
	return []ui.Key{{Keys: "enter", Desc: "guess"}, {Keys: "tab", Desc: "1-100 / 1-1000"}, {Keys: "esc", Desc: "menu"}}
}
func (g *guessGame) start() tea.Cmd {
	g.in = g.a.ctx.S.NewInput("Your guess", 5)
	g.secret = 1 + g.a.rng.IntN(g.max)
	g.tries, g.won, g.message = nil, false, ""
	g.lo, g.hi = 1, g.max
	return g.in.Focus()
}
func (g *guessGame) update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if g.won {
		if km.String() == "enter" || km.String() == "r" || km.String() == " " {
			return g.start()
		}
		if km.String() == "tab" {
			g.toggle()
			return g.start()
		}
		return nil
	}
	switch km.String() {
	case "esc":
		return exitGame
	case "tab":
		g.toggle()
		return g.start()
	case "enter":
		n, err := strconv.Atoi(strings.TrimSpace(g.in.Value()))
		g.in.SetValue("")
		if err != nil || n < 1 || n > g.max {
			g.message = ui.Tf("Enter a number between 1 and %d", g.max)
			return nil
		}
		g.tries = append(g.tries, n)
		switch {
		case n < g.secret:
			g.message = ui.Tf("%d is too low ↑", n)
			g.lo = max(g.lo, n+1)
		case n > g.secret:
			g.message = ui.Tf("%d is too high ↓", n)
			g.hi = min(g.hi, n-1)
		default:
			g.won = true
			g.message = ui.Tf("🎉 Found %d in %d tries!", n, len(g.tries))
			table := "guess"
			if g.max == 1000 {
				table = "guess1000"
			}
			return reportScore(table, len(g.tries), fmt.Sprintf("1-%d", g.max), true)
		}
		return nil
	}
	var cmd tea.Cmd
	g.in, cmd = g.in.Update(msg)
	return cmd
}
func (g *guessGame) toggle() {
	if g.max == 100 {
		g.max = 1000
	} else {
		g.max = 100
	}
}
func (g *guessGame) view(w, h int) string {
	st := g.a.ctx.S
	g.in.Width = 10
	var b strings.Builder
	b.WriteString(st.Title.Render(ui.Tf("I'm thinking of a number between 1 and %d", g.max)) + "\n\n")
	if !g.won {
		b.WriteString(g.in.View() + "\n\n")
		b.WriteString(st.Muted.Render(ui.Tf("Range: %d – %d", g.lo, g.hi)) + "\n")
		span := float64(g.hi-g.lo+1) / float64(g.max) * 100
		b.WriteString(st.Bar(100-span, 30) + "\n\n")
	}
	style := st.Warning
	if g.won {
		style = st.Success.Bold(true)
	}
	b.WriteString(style.Render(g.message) + "\n\n")
	if len(g.tries) > 0 {
		parts := make([]string, len(g.tries))
		for i, t := range g.tries {
			parts[i] = strconv.Itoa(t)
		}
		b.WriteString(st.Muted.Render(ui.T("Tries: ")+strings.Join(parts, " · ")) + "\n")
	}
	if g.won {
		b.WriteString("\n" + st.Muted.Render(ui.T("enter: play again · tab: change range · esc: menu")))
	}
	return center(b.String(), w, h)
}

// ---- 2048 --------------------------------------------------------------------

type mergeGame struct {
	a     *Screen
	b     Board2048
	prev  *Board2048
	score int
	prevS int
	over  bool
	won   bool
	moves int
}

func newMerge(a *Screen) *mergeGame        { return &mergeGame{a: a} }
func (g *mergeGame) id() string            { return "merge" }
func (g *mergeGame) name() string          { return "2048" }
func (g *mergeGame) desc() string          { return "Slide tiles, merge equal numbers, reach 2048 and beyond." }
func (g *mergeGame) capturing() bool       { return false }
func (g *mergeGame) pause()                {}
func (g *mergeGame) scoreIDs() [][2]string { return [][2]string{{"merge", "2048"}} }
func (g *mergeGame) keys() []ui.Key {
	return []ui.Key{{Keys: "←↑→↓/wasd", Desc: "slide"}, {Keys: "u", Desc: "undo"}, {Keys: "r", Desc: "new game"}}
}
func (g *mergeGame) start() tea.Cmd {
	if g.moves > 0 && !g.over {
		return nil // resume
	}
	g.b, g.prev, g.score, g.over, g.won, g.moves = Board2048{}, nil, 0, false, false, 0
	g.b.Spawn(g.a.rng)
	g.b.Spawn(g.a.rng)
	return nil
}
func (g *mergeGame) update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	k := km.String()
	switch k {
	case "r":
		g.over, g.moves = true, 0
		return g.start()
	case "u":
		if g.prev != nil && !g.over {
			g.b, g.score, g.prev = *g.prev, g.prevS, nil
		}
		return nil
	}
	if g.over {
		if k == "enter" || k == " " {
			g.moves = 0
			return g.start()
		}
		return nil
	}
	d, ok := dirKey(k)
	if !ok {
		return nil
	}
	dir := map[Point]int{{-1, 0}: 0, {1, 0}: 1, {0, -1}: 2, {0, 1}: 3}[d]
	before := g.b
	gained, moved := g.b.Move(dir)
	if !moved {
		return nil
	}
	g.prev, g.prevS = &before, g.score
	g.score += gained
	g.moves++
	g.b.Spawn(g.a.rng)
	if g.b.MaxTile() >= 2048 && !g.won {
		g.won = true
	}
	if !g.b.CanMove() {
		g.over = true
		return reportScore("merge", g.score, fmt.Sprintf("max %d", g.b.MaxTile()), false)
	}
	return nil
}

var tileColors = map[int][2]string{
	2: {"#EEE4DA", "#776E65"}, 4: {"#EDE0C8", "#776E65"}, 8: {"#F2B179", "#F9F6F2"}, 16: {"#F59563", "#F9F6F2"},
	32: {"#F67C5F", "#F9F6F2"}, 64: {"#F65E3B", "#F9F6F2"}, 128: {"#EDCF72", "#F9F6F2"}, 256: {"#EDCC61", "#F9F6F2"},
	512: {"#EDC850", "#F9F6F2"}, 1024: {"#EDC53F", "#F9F6F2"}, 2048: {"#EDC22E", "#F9F6F2"},
}

func (g *mergeGame) view(w, h int) string {
	st := g.a.ctx.S
	cw, chh := 8, 3
	if w < 40 || h < 18 {
		cw, chh = 6, 1
	}
	var rows []string
	for _, r := range g.b {
		var cells []string
		for _, v := range r {
			style := lipgloss.NewStyle().Width(cw).Height(chh).Align(lipgloss.Center, lipgloss.Center).Bold(true)
			label := ""
			if v == 0 {
				style = style.Background(lipgloss.Color(st.C.Highlight))
			} else {
				c, ok := tileColors[v]
				if !ok {
					c = [2]string{"#3C3A32", "#F9F6F2"}
				}
				style = style.Background(lipgloss.Color(c[0])).Foreground(lipgloss.Color(c[1]))
				label = strconv.Itoa(v)
			}
			cells = append(cells, style.Render(label))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, interleave(cells, " ")...))
	}
	board := strings.Join(rows, "\n")
	if chh > 1 {
		board = strings.Join(rows, "\n\n")
	}
	status := st.Text.Render(ui.Tf("Score %d · moves %d", g.score, g.moves))
	if best, ok := g.a.scores.Best("merge"); ok {
		status += st.Muted.Render(ui.Tf(" · best %d", best.Value))
	}
	switch {
	case g.over:
		status += "\n" + st.Error.Bold(true).Render(ui.T("No more moves!")) + st.Muted.Render(" — "+ui.T("enter for a new game"))
	case g.won:
		status += "\n" + st.Success.Bold(true).Render(ui.T("2048 reached! Keep going…"))
	}
	return center(status+"\n\n"+board, w, h)
}

func interleave(items []string, sep string) []string {
	out := make([]string, 0, len(items)*2)
	for i, it := range items {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, it)
	}
	return out
}

// ---- Mines -------------------------------------------------------------------

type minesLevel struct {
	id, label string
	w, h, n   int
}

var minesLevels = []minesLevel{
	{"mines-beginner", "Beginner 9×9", 9, 9, 10},
	{"mines-intermediate", "Intermediate 16×16", 16, 16, 40},
	{"mines-expert", "Expert 30×16", 30, 16, 99},
}

type minesGame struct {
	a       *Screen
	level   int
	f       *Field
	cx, cy  int
	started time.Time
	ended   time.Duration
	gen     int
	choose  bool
}

func newMines(a *Screen) *minesGame { return &minesGame{a: a, choose: true} }
func (g *minesGame) id() string     { return "mines" }
func (g *minesGame) name() string   { return "Mines" }
func (g *minesGame) desc() string {
	return "Reveal every safe cell. Numbers tell how many mines touch a cell."
}
func (g *minesGame) capturing() bool { return false }
func (g *minesGame) pause()          {}
func (g *minesGame) scoreIDs() [][2]string {
	out := make([][2]string, len(minesLevels))
	for i, l := range minesLevels {
		out[i] = [2]string{l.id, l.label + " (seconds)"}
	}
	return out
}
func (g *minesGame) keys() []ui.Key {
	if g.choose {
		return []ui.Key{{Keys: "1/2/3", Desc: "difficulty"}}
	}
	return []ui.Key{{Keys: "←↑→↓", Desc: "move"}, {Keys: "space", Desc: "reveal"}, {Keys: "f", Desc: "flag"}, {Keys: "r", Desc: "restart"}, {Keys: "n", Desc: "difficulty"}}
}
func (g *minesGame) start() tea.Cmd {
	if g.f != nil && !g.f.Lost && !g.f.Won && !g.choose {
		return g.clock()
	}
	g.choose = true
	return nil
}
func (g *minesGame) newField() tea.Cmd {
	l := minesLevels[g.level]
	g.f = NewField(l.w, l.h, l.n)
	g.cx, g.cy = l.w/2, l.h/2
	g.started, g.ended, g.choose = time.Time{}, 0, false
	g.gen++
	return nil
}
func (g *minesGame) clock() tea.Cmd {
	gen := g.gen
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{"mines", gen} })
}
func (g *minesGame) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tickMsg:
		if m.game == "mines" && m.gen == g.gen && g.f != nil && !g.f.Lost && !g.f.Won && !g.started.IsZero() {
			return g.clock()
		}
	case tea.KeyMsg:
		k := m.String()
		if g.choose {
			if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(minesLevels) {
				g.level = n - 1
				return g.newField()
			}
			return nil
		}
		f := g.f
		switch k {
		case "n":
			g.choose = true
			return nil
		case "r":
			return g.newField()
		}
		if f.Lost || f.Won {
			if k == "enter" || k == " " {
				return g.newField()
			}
			return nil
		}
		if d, ok := dirKey(k); ok && k != "f" {
			g.cx = (g.cx + d.X + f.W) % f.W
			g.cy = (g.cy + d.Y + f.H) % f.H
			return nil
		}
		switch k {
		case " ", "enter", "x":
			first := !f.Placed
			if f.Cells[g.cy][g.cx].Open {
				f.Chord(g.cx, g.cy, g.a.rng)
			} else {
				f.Reveal(g.cx, g.cy, g.a.rng)
			}
			var cmd tea.Cmd
			if first {
				g.started = time.Now()
				g.gen++
				cmd = g.clock()
			}
			if f.Won {
				g.ended = time.Since(g.started)
				secs := max(1, int(g.ended.Seconds()))
				return reportScore(minesLevels[g.level].id, secs, "", true)
			}
			if f.Lost {
				g.ended = time.Since(g.started)
			}
			return cmd
		case "f", "m":
			f.ToggleFlag(g.cx, g.cy)
		}
	}
	return nil
}

func (g *minesGame) view(w, h int) string {
	st := g.a.ctx.S
	if g.choose {
		var b strings.Builder
		b.WriteString(st.Title.Render(ui.T("Choose a difficulty")) + "\n\n")
		for i, l := range minesLevels {
			b.WriteString(st.Key.Render(fmt.Sprintf("  %d  ", i+1)) + st.Text.Render(ui.T(l.label)) + st.Muted.Render(ui.Tf("  · %d mines", l.n)) + "\n")
		}
		if w < 64 {
			b.WriteString("\n" + st.Warning.Render(ui.T("Expert needs a terminal at least 64 columns wide.")))
		}
		return center(b.String(), w, h)
	}
	f := g.f
	numColor := []string{"", "#4FC3F7", "#81C784", "#E57373", "#BA68C8", "#FFB74D", "#4DD0E1", "#F06292", "#BDBDBD"}
	var b strings.Builder
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			c := f.Cells[y][x]
			txt, style := "■ ", st.Muted
			switch {
			case c.Open && c.Mine:
				txt, style = "✹ ", st.Error.Bold(true)
			case (f.Lost) && c.Mine && !c.Flag:
				txt, style = "✹ ", st.Error
			case c.Flag:
				txt, style = "⚑ ", st.Warning.Bold(true)
				if f.Lost && !c.Mine {
					txt, style = "✗ ", st.Error
				}
			case c.Open && c.Adj == 0:
				txt, style = "· ", st.Muted.Faint(true)
			case c.Open:
				txt = strconv.Itoa(c.Adj) + " "
				style = lipgloss.NewStyle().Foreground(lipgloss.Color(numColor[c.Adj])).Bold(true)
			}
			if x == g.cx && y == g.cy {
				style = st.Selected
			}
			b.WriteString(style.Render(txt))
		}
		b.WriteString("\n")
	}
	elapsed := g.ended
	if elapsed == 0 && !g.started.IsZero() {
		elapsed = time.Since(g.started)
	}
	status := st.Text.Render(ui.Tf("%s · mines %d · flags %d · time %ds", ui.T(minesLevels[g.level].label), f.Mines, f.Flags(), int(elapsed.Seconds())))
	switch {
	case f.Won:
		status += "\n" + st.Success.Bold(true).Render(ui.T("Field cleared! 🎉")) + st.Muted.Render(" — "+ui.T("enter to play again"))
	case f.Lost:
		status += "\n" + st.Error.Bold(true).Render(ui.T("BOOM! You hit a mine.")) + st.Muted.Render(" — "+ui.T("enter to retry"))
	}
	return center(status+"\n\n"+strings.TrimRight(b.String(), "\n"), w, h)
}

// ---- Typing Test -------------------------------------------------------------

var wordsEN = strings.Fields(`the be to of and a in that have it for not on with he as you do at this but his by from they we say her
she or an will my one all would there their what so up out if about who get which go me when make can like time no just him know take
people into year your good some could them see other than then now look only come its over think also back after use two how our work
first well way even new want because any these give day most us system code terminal window shell file theme green light quick river
music planet coffee garden simple bridge orange silver future market energy little number sound world place again right house small`)

var wordsFR = strings.Fields(`le de un être et à il avoir ne je son que se qui ce dans en du elle au pour pas que vous par sur faire plus
dire me on mon lui nous comme mais pouvoir avec tout aller voir bien où sans tu ou leur homme si deux mari moi vouloir te femme venir
quand grand celui notre devoir là jour prendre même votre rien petit encore aussi quelque dont tout mer trouver donner temps ça peu
falloir sous parler alors main chose ton mettre vie savoir yeux passer autre après regarder toujours puis jamais cela aimer non heure
terminal fichier thème rivière musique planète jardin simple pont orange argent avenir marché énergie nombre monde place maison`)

type typingGame struct {
	a        *Screen
	words    []string
	idx      int
	typed    string
	results  []bool
	correct  int
	total    int
	started  time.Time
	duration time.Duration
	running  bool
	done     bool
	gen      int
	wpm, acc float64
	// pausedElapsed is the elapsed time when the test was paused.
	pausedElapsed time.Duration
}

func newTyping(a *Screen) *typingGame       { return &typingGame{a: a, duration: 30 * time.Second} }
func (g *typingGame) id() string            { return "typing" }
func (g *typingGame) name() string          { return "Typing Test" }
func (g *typingGame) desc() string          { return "Type as many words as you can in 30 seconds." }
func (g *typingGame) capturing() bool       { return g.running }
func (g *typingGame) scoreIDs() [][2]string { return [][2]string{{"typing", "Words per minute"}} }
func (g *typingGame) keys() []ui.Key {
	return []ui.Key{{Keys: "type", Desc: "start"}, {Keys: "space", Desc: "next word"}, {Keys: "tab", Desc: "restart"}, {Keys: "esc", Desc: "stop"}}
}
func (g *typingGame) pause() {
	if g.running {
		g.running = false
		g.gen++
	}
}
func (g *typingGame) start() tea.Cmd {
	src := wordsEN
	if ui.Language() == "fr" {
		src = wordsFR
	}
	g.words = make([]string, 120)
	for i := range g.words {
		g.words[i] = src[g.a.rng.IntN(len(src))]
	}
	g.idx, g.typed, g.results, g.correct, g.total = 0, "", nil, 0, 0
	g.running, g.done, g.pausedElapsed = false, false, 0
	g.gen++
	return nil
}
func (g *typingGame) finish() tea.Cmd {
	g.running, g.done = false, true
	g.gen++
	secs := time.Since(g.started).Seconds()
	g.wpm, g.acc = WPM(g.correct, g.total, secs)
	return reportScore("typing", int(g.wpm+0.5), fmt.Sprintf("%.0f%% acc", g.acc), false)
}
func (g *typingGame) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tickMsg:
		if m.game != "typing" || m.gen != g.gen || !g.running {
			return nil
		}
		if time.Since(g.started) >= g.duration {
			return g.finish()
		}
		return tick("typing", g.gen, 200*time.Millisecond)
	case tea.KeyMsg:
		k := m.String()
		if k == "tab" {
			return g.start()
		}
		if k == "esc" && g.running {
			g.pause()
			return nil
		}
		if g.done {
			if k == "enter" {
				return g.start()
			}
			return nil
		}
		var cmd tea.Cmd
		if !g.running && (m.Type == tea.KeyRunes || k == " ") {
			if g.idx == 0 && g.typed == "" {
				g.started = time.Now()
			} else {
				// resume after pause: keep elapsed time
				g.started = time.Now().Add(-g.elapsedAtPause())
			}
			g.running = true
			g.gen++
			cmd = tick("typing", g.gen, 200*time.Millisecond)
		}
		if !g.running {
			return nil
		}
		switch {
		case k == " ":
			if g.typed == "" {
				return cmd
			}
			target := g.words[g.idx]
			ok := g.typed == target
			if ok {
				g.correct += len([]rune(target)) + 1
			} else {
				g.correct += commonPrefix(g.typed, target)
			}
			g.total += len([]rune(g.typed)) + 1
			g.results = append(g.results, ok)
			g.idx++
			g.typed = ""
			if g.idx >= len(g.words) {
				return tea.Batch(cmd, g.finish())
			}
		case k == "backspace":
			if r := []rune(g.typed); len(r) > 0 {
				g.typed = string(r[:len(r)-1])
			}
		case m.Type == tea.KeyRunes:
			if len([]rune(g.typed)) < 30 {
				g.typed += string(m.Runes)
			}
		}
		return cmd
	}
	return nil
}

func (g *typingGame) elapsedAtPause() time.Duration { return g.pausedElapsed }

func commonPrefix(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := 0
	for n < len(ra) && n < len(rb) && ra[n] == rb[n] {
		n++
	}
	return n
}

func (g *typingGame) view(w, h int) string {
	st := g.a.ctx.S
	if g.words == nil {
		g.start()
	}
	tw := min(70, w-4)
	var elapsed time.Duration
	if g.running {
		elapsed = time.Since(g.started)
		g.pausedElapsed = elapsed
	} else if g.done {
		elapsed = g.duration
	} else {
		elapsed = g.pausedElapsed
	}
	remaining := max(0, g.duration-elapsed)
	var lines []string
	line, lineW := "", 0
	for i := max(0, g.idx-8); i < len(g.words) && len(lines) < 4; i++ {
		word := g.words[i]
		var rendered string
		switch {
		case i < g.idx:
			if g.results[i] {
				rendered = st.Success.Render(word)
			} else {
				rendered = st.Error.Strikethrough(true).Render(word)
			}
		case i == g.idx:
			rendered = renderCurrent(st, word, g.typed)
		default:
			rendered = st.Muted.Render(word)
		}
		ww := ui.Width(word)
		if lineW+ww+1 > tw && line != "" {
			lines = append(lines, line)
			line, lineW = "", 0
		}
		line += rendered + " "
		lineW += ww + 1
	}
	if line != "" && len(lines) < 4 {
		lines = append(lines, line)
	}
	secs := elapsed.Seconds()
	wpm, acc := WPM(g.correct, g.total, secs)
	if g.done {
		wpm, acc = g.wpm, g.acc
	}
	status := st.Text.Render(ui.Tf("⏱ %ds   WPM %.0f   accuracy %.0f%%", int(remaining.Seconds()+0.99), wpm, acc))
	if best, ok := g.a.scores.Best("typing"); ok {
		status += st.Muted.Render(ui.Tf("   best %d", best.Value))
	}
	bar := st.Bar(float64(elapsed)/float64(g.duration)*100, tw)
	body := status + "\n" + bar + "\n\n" + strings.Join(lines, "\n") + "\n\n" + st.Accent.Render("› ") + st.Text.Render(g.typed) + st.Accent.Render("▏")
	switch {
	case g.done:
		body += "\n\n" + st.Success.Bold(true).Render(ui.Tf("Result: %.0f WPM · %.0f%% accuracy", g.wpm, g.acc)) + "\n" + st.Muted.Render(ui.T("enter or tab to try again"))
	case !g.running && g.idx == 0 && g.typed == "":
		body += "\n\n" + st.Muted.Render(ui.T("Start typing to begin the 30-second test."))
	case !g.running:
		body += "\n\n" + st.Warning.Render(ui.T("Paused — type to resume, tab to restart."))
	}
	return center(body, w, h)
}

func renderCurrent(st ui.Styles, word, typed string) string {
	rw, rt := []rune(word), []rune(typed)
	var b strings.Builder
	for i, r := range rw {
		switch {
		case i < len(rt) && rt[i] == r:
			b.WriteString(st.Success.Underline(true).Render(string(r)))
		case i < len(rt):
			b.WriteString(st.Error.Underline(true).Render(string(r)))
		default:
			b.WriteString(st.Accent.Underline(true).Render(string(r)))
		}
	}
	if len(rt) > len(rw) {
		b.WriteString(st.Error.Render(string(rt[len(rw):])))
	}
	return b.String()
}
