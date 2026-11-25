package online

// Rows and Cols of the Four in a Row board.
const (
	Rows = 6
	Cols = 7
)

// Board is indexed [row][col], row 0 at the top. 0 = empty, 1/2 = players.
type Board [Rows][Cols]int

// Drop places a disc for player in col; it returns the row or -1.
func (b *Board) Drop(col, player int) int {
	if col < 0 || col >= Cols {
		return -1
	}
	for r := Rows - 1; r >= 0; r-- {
		if b[r][col] == 0 {
			b[r][col] = player
			return r
		}
	}
	return -1
}

// Winner returns 1 or 2 for a winner, 3 for a draw, 0 otherwise.
func (b *Board) Winner() int {
	dirs := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			p := b[r][c]
			if p == 0 {
				continue
			}
			for _, d := range dirs {
				n := 1
				for k := 1; k < 4; k++ {
					rr, cc := r+d[0]*k, c+d[1]*k
					if rr < 0 || rr >= Rows || cc < 0 || cc >= Cols || b[rr][cc] != p {
						break
					}
					n++
				}
				if n == 4 {
					return p
				}
			}
		}
	}
	for c := 0; c < Cols; c++ {
		if b[0][c] == 0 {
			return 0
		}
	}
	return 3
}
