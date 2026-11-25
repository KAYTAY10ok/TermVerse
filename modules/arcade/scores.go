package arcade

import (
	"sort"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
)

// Score is a high-score entry.
type Score struct {
	Value  int       `json:"value"`
	Player string    `json:"player"`
	Detail string    `json:"detail,omitempty"`
	Date   time.Time `json:"date"`
}

// Scores persists the best scores of every game.
type Scores struct {
	path  string
	Games map[string][]Score `json:"games"`
}

// LoadScores reads scores.json (a missing or invalid file yields empty scores;
// an invalid file is backed up first).
func LoadScores(path string) *Scores {
	s := &Scores{path: path, Games: map[string][]Score{}}
	if _, err := fsutil.ReadJSON(path, s); err != nil {
		_, _ = fsutil.BackupCorrupt(path, time.Now().Format("20060102-150405"))
		s.Games = map[string][]Score{}
	}
	if s.Games == nil {
		s.Games = map[string][]Score{}
	}
	return s
}

// Add records a score and reports its rank (1-based, 0 if not in the top 10).
func (s *Scores) Add(game string, sc Score, lowerIsBetter bool) (int, error) {
	list := append(s.Games[game], sc)
	sort.SliceStable(list, func(i, j int) bool {
		if lowerIsBetter {
			return list[i].Value < list[j].Value
		}
		return list[i].Value > list[j].Value
	})
	if len(list) > 10 {
		list = list[:10]
	}
	s.Games[game] = list
	rank := 0
	for i, x := range list {
		if x.Date.Equal(sc.Date) && x.Value == sc.Value {
			rank = i + 1
			break
		}
	}
	return rank, fsutil.WriteJSONAtomic(s.path, s)
}

// Best returns the best score of a game.
func (s *Scores) Best(game string) (Score, bool) {
	l := s.Games[game]
	if len(l) == 0 {
		return Score{}, false
	}
	return l[0], true
}
