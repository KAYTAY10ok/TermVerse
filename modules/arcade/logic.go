package arcade

import (
	"math/rand/v2"
)

// ---- Snake -------------------------------------------------------------------

// Point is a grid coordinate.
type Point struct{ X, Y int }

// SnakeState is the pure state of a snake game.
type SnakeState struct {
	W, H  int
	Body  []Point // head first
	Dir   Point
	next  Point
	Food  Point
	Score int
	Over  bool
	rng   *rand.Rand
}

// NewSnake starts a game on a w×h board.
func NewSnake(w, h int, rng *rand.Rand) *SnakeState {
	s := &SnakeState{W: w, H: h, Dir: Point{1, 0}, next: Point{1, 0}, rng: rng}
	cx, cy := w/2, h/2
	s.Body = []Point{{cx, cy}, {cx - 1, cy}, {cx - 2, cy}}
	s.placeFood()
	return s
}

func (s *SnakeState) occupied(p Point) bool {
	for _, b := range s.Body {
		if b == p {
			return true
		}
	}
	return false
}

func (s *SnakeState) placeFood() {
	free := s.W*s.H - len(s.Body)
	if free <= 0 {
		s.Over = true
		return
	}
	n := s.rng.IntN(free)
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			p := Point{x, y}
			if s.occupied(p) {
				continue
			}
			if n == 0 {
				s.Food = p
				return
			}
			n--
		}
	}
}

// Turn changes direction (reversing is ignored).
func (s *SnakeState) Turn(d Point) {
	if d.X == -s.Dir.X && d.Y == -s.Dir.Y {
		return
	}
	s.next = d
}

// Step advances the snake one cell. It returns true if food was eaten.
func (s *SnakeState) Step() bool {
	if s.Over {
		return false
	}
	s.Dir = s.next
	h := s.Body[0]
	nh := Point{h.X + s.Dir.X, h.Y + s.Dir.Y}
	if nh.X < 0 || nh.Y < 0 || nh.X >= s.W || nh.Y >= s.H {
		s.Over = true
		return false
	}
	ate := nh == s.Food
	body := s.Body
	if !ate {
		body = body[:len(body)-1] // the tail moves away this turn
	}
	for _, b := range body {
		if b == nh {
			s.Over = true
			return false
		}
	}
	s.Body = append([]Point{nh}, body...)
	if ate {
		s.Score += 10
		s.placeFood()
	}
	return ate
}

// ---- 2048 --------------------------------------------------------------------

// Board2048 is a 4×4 grid.
type Board2048 [4][4]int

// slideRow slides a row to the left and merges equal tiles once.
func slideRow(r [4]int) ([4]int, int, bool) {
	var out [4]int
	gained, n := 0, 0
	var vals []int
	for _, v := range r {
		if v != 0 {
			vals = append(vals, v)
		}
	}
	for i := 0; i < len(vals); i++ {
		if i+1 < len(vals) && vals[i] == vals[i+1] {
			out[n] = vals[i] * 2
			gained += out[n]
			i++
		} else {
			out[n] = vals[i]
		}
		n++
	}
	return out, gained, out != r
}

// Move slides the board in direction d (0 left, 1 right, 2 up, 3 down).
func (b *Board2048) Move(d int) (int, bool) {
	total, moved := 0, false
	for i := 0; i < 4; i++ {
		var row [4]int
		for j := 0; j < 4; j++ {
			switch d {
			case 0:
				row[j] = b[i][j]
			case 1:
				row[j] = b[i][3-j]
			case 2:
				row[j] = b[j][i]
			case 3:
				row[j] = b[3-j][i]
			}
		}
		nr, g, m := slideRow(row)
		total += g
		moved = moved || m
		for j := 0; j < 4; j++ {
			switch d {
			case 0:
				b[i][j] = nr[j]
			case 1:
				b[i][3-j] = nr[j]
			case 2:
				b[j][i] = nr[j]
			case 3:
				b[3-j][i] = nr[j]
			}
		}
	}
	return total, moved
}

// Spawn adds a 2 (90%) or 4 tile on a random empty cell.
func (b *Board2048) Spawn(rng *rand.Rand) bool {
	var empty []Point
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if b[y][x] == 0 {
				empty = append(empty, Point{x, y})
			}
		}
	}
	if len(empty) == 0 {
		return false
	}
	p := empty[rng.IntN(len(empty))]
	b[p.Y][p.X] = 2
	if rng.IntN(10) == 0 {
		b[p.Y][p.X] = 4
	}
	return true
}

// CanMove reports whether any move is possible.
func (b *Board2048) CanMove() bool {
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if b[y][x] == 0 || x < 3 && b[y][x] == b[y][x+1] || y < 3 && b[y][x] == b[y+1][x] {
				return true
			}
		}
	}
	return false
}

// MaxTile returns the largest tile.
func (b *Board2048) MaxTile() int {
	m := 0
	for _, r := range b {
		for _, v := range r {
			m = max(m, v)
		}
	}
	return m
}

// ---- Mines -------------------------------------------------------------------

// Cell of a minefield.
type Cell struct {
	Mine, Open, Flag bool
	Adj              int
}

// Field is a minefield.
type Field struct {
	W, H, Mines int
	Cells       [][]Cell
	Placed      bool
	Lost, Won   bool
	opened      int
}

// NewField creates an empty field; mines are placed on the first reveal so
// the first click is always safe.
func NewField(w, h, mines int) *Field {
	f := &Field{W: w, H: h, Mines: mines, Cells: make([][]Cell, h)}
	for y := range f.Cells {
		f.Cells[y] = make([]Cell, w)
	}
	return f
}

func (f *Field) in(x, y int) bool { return x >= 0 && y >= 0 && x < f.W && y < f.H }

func (f *Field) each(x, y int, fn func(nx, ny int)) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && f.in(x+dx, y+dy) {
				fn(x+dx, y+dy)
			}
		}
	}
}

// Place puts mines randomly, avoiding (sx, sy) and its neighbours.
func (f *Field) Place(sx, sy int, rng *rand.Rand) {
	safe := func(x, y int) bool { return abs(x-sx) <= 1 && abs(y-sy) <= 1 }
	var spots []Point
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			if !safe(x, y) {
				spots = append(spots, Point{x, y})
			}
		}
	}
	rng.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
	for i := 0; i < f.Mines && i < len(spots); i++ {
		f.Cells[spots[i].Y][spots[i].X].Mine = true
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			n := 0
			f.each(x, y, func(nx, ny int) {
				if f.Cells[ny][nx].Mine {
					n++
				}
			})
			f.Cells[y][x].Adj = n
		}
	}
	f.Placed = true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Reveal opens a cell (flood-filling empty areas).
func (f *Field) Reveal(x, y int, rng *rand.Rand) {
	if f.Lost || f.Won || !f.in(x, y) {
		return
	}
	if !f.Placed {
		f.Place(x, y, rng)
	}
	c := &f.Cells[y][x]
	if c.Open || c.Flag {
		return
	}
	if c.Mine {
		c.Open = true
		f.Lost = true
		return
	}
	stack := []Point{{x, y}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		cc := &f.Cells[p.Y][p.X]
		if cc.Open || cc.Flag || cc.Mine {
			continue
		}
		cc.Open = true
		f.opened++
		if cc.Adj == 0 {
			f.each(p.X, p.Y, func(nx, ny int) { stack = append(stack, Point{nx, ny}) })
		}
	}
	if f.opened == f.W*f.H-f.Mines {
		f.Won = true
	}
}

// Chord opens the neighbours of an open number whose flags are all placed.
func (f *Field) Chord(x, y int, rng *rand.Rand) {
	c := f.Cells[y][x]
	if !c.Open || c.Adj == 0 {
		return
	}
	flags := 0
	f.each(x, y, func(nx, ny int) {
		if f.Cells[ny][nx].Flag {
			flags++
		}
	})
	if flags == c.Adj {
		f.each(x, y, func(nx, ny int) { f.Reveal(nx, ny, rng) })
	}
}

// ToggleFlag flags or unflags a closed cell.
func (f *Field) ToggleFlag(x, y int) {
	if f.Lost || f.Won {
		return
	}
	c := &f.Cells[y][x]
	if !c.Open {
		c.Flag = !c.Flag
	}
}

// Flags counts flagged cells.
func (f *Field) Flags() int {
	n := 0
	for _, r := range f.Cells {
		for _, c := range r {
			if c.Flag {
				n++
			}
		}
	}
	return n
}

// ---- Typing ------------------------------------------------------------------

// WPM computes words per minute (5 chars = 1 word) and accuracy.
func WPM(correctChars, typedChars int, secs float64) (float64, float64) {
	if secs <= 0 {
		return 0, 0
	}
	wpm := float64(correctChars) / 5 / (secs / 60)
	acc := 100.0
	if typedChars > 0 {
		acc = float64(correctChars) / float64(typedChars) * 100
	}
	return wpm, acc
}
