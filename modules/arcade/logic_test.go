package arcade

import (
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"
)

func rng() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

func TestSnakeEatsAndDies(t *testing.T) {
	s := NewSnake(10, 5, rng())
	h := s.Body[0]
	s.Food = Point{h.X + 1, h.Y}
	if !s.Step() || len(s.Body) != 4 || s.Score != 10 {
		t.Fatalf("did not eat: %+v", s)
	}
	s.Turn(Point{-1, 0}) // reversing is ignored
	if s.next != (Point{1, 0}) {
		t.Error("reverse accepted")
	}
	for i := 0; i < 20 && !s.Over; i++ {
		s.Food = Point{0, 0}
		s.Step()
	}
	if !s.Over {
		t.Error("snake should hit the wall")
	}
}

func TestSnakeCanFollowTail(t *testing.T) {
	s := NewSnake(10, 10, rng())
	s.Body = []Point{{2, 2}, {2, 3}, {3, 3}, {3, 2}} // head next to tail
	s.Dir, s.next = Point{0, -1}, Point{1, 0}
	s.Food = Point{9, 9}
	s.Step()
	if s.Over {
		t.Error("moving into the cell the tail leaves must be allowed")
	}
}

func TestSlideRow(t *testing.T) {
	cases := []struct {
		in, out [4]int
		gain    int
	}{
		{[4]int{2, 2, 2, 2}, [4]int{4, 4, 0, 0}, 8},
		{[4]int{0, 2, 0, 2}, [4]int{4, 0, 0, 0}, 4},
		{[4]int{4, 4, 8, 0}, [4]int{8, 8, 0, 0}, 8},
		{[4]int{2, 4, 8, 16}, [4]int{2, 4, 8, 16}, 0},
	}
	for _, c := range cases {
		out, g, _ := slideRow(c.in)
		if out != c.out || g != c.gain {
			t.Errorf("%v -> %v (%d), want %v (%d)", c.in, out, g, c.out, c.gain)
		}
	}
}

func TestBoardMove(t *testing.T) {
	b := Board2048{{2, 0, 0, 0}, {2, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}}
	g, moved := b.Move(2) // up
	if !moved || g != 4 || b[0][0] != 4 || b[1][0] != 0 {
		t.Fatalf("%v", b)
	}
	full := Board2048{{2, 4, 2, 4}, {4, 2, 4, 2}, {2, 4, 2, 4}, {4, 2, 4, 2}}
	if full.CanMove() {
		t.Error("should be stuck")
	}
}

func TestMinesFirstClickSafeAndWin(t *testing.T) {
	f := NewField(9, 9, 10)
	f.Reveal(4, 4, rng())
	if f.Lost || !f.Cells[4][4].Open || f.Cells[4][4].Adj != 0 {
		t.Fatal("first click must be safe and open an empty area")
	}
	mines := 0
	for y := range f.Cells {
		for x := range f.Cells[y] {
			if f.Cells[y][x].Mine {
				mines++
			}
		}
	}
	if mines != 10 {
		t.Fatalf("mines=%d", mines)
	}
	for y := range f.Cells {
		for x := range f.Cells[y] {
			if !f.Cells[y][x].Mine {
				f.Reveal(x, y, rng())
			}
		}
	}
	if !f.Won {
		t.Error("should be won")
	}
}

func TestWPM(t *testing.T) {
	w, a := WPM(250, 260, 60)
	if w != 50 || a < 96 || a > 96.2 {
		t.Error(w, a)
	}
}

func TestScores(t *testing.T) {
	p := filepath.Join(t.TempDir(), "scores.json")
	s := LoadScores(p)
	now := time.Now()
	for i := 0; i < 12; i++ {
		s.Add("snake", Score{Value: i * 10, Date: now.Add(time.Duration(i))}, false)
	}
	rank, _ := s.Add("guess", Score{Value: 5, Date: now}, true)
	r2, _ := s.Add("guess", Score{Value: 3, Date: now.Add(1)}, true)
	if rank != 1 || r2 != 1 {
		t.Error(rank, r2)
	}
	s2 := LoadScores(p)
	if len(s2.Games["snake"]) != 10 || s2.Games["snake"][0].Value != 110 {
		t.Fatalf("%v", s2.Games["snake"])
	}
	if b, _ := s2.Best("guess"); b.Value != 3 {
		t.Error(b)
	}
}
