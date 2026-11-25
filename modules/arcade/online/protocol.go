// Package online implements the TermVerse Online Room: a small TCP server
// (hosted by one player) relaying chat messages and running a two-player
// Four in a Row game. The protocol is newline-delimited JSON.
//
// Security: the server only starts when a user explicitly chooses to host,
// accepts at most MaxClients, caps message sizes, rate-limits clients,
// strips control characters from all text and can require a password.
// Messages are not encrypted: do not share secrets in the chat.
package online

import (
	"strings"
	"unicode"
)

// Protocol limits.
const (
	ProtocolVersion = 1
	MaxClients      = 8
	MaxLine         = 4096
	MaxName         = 16
	MaxChat         = 300
	DefaultPort     = "7777"
)

// Msg is a protocol message. T is the type:
//
//	client → server: hello{Name,Pass,V}, chat{Text}, join, leave, move{Col}, reset
//	server → client: welcome{Name}, error{Text}, chat{From,Text}, sys{Text},
//	                 roster{Names}, game{Game}
type Msg struct {
	T     string     `json:"t"`
	V     int        `json:"v,omitempty"`
	Name  string     `json:"name,omitempty"`
	Pass  string     `json:"pass,omitempty"`
	From  string     `json:"from,omitempty"`
	Text  string     `json:"text,omitempty"`
	Col   int        `json:"col,omitempty"`
	Names []string   `json:"names,omitempty"`
	Game  *GameState `json:"game,omitempty"`
}

// GameState is the Four in a Row state broadcast to everyone.
type GameState struct {
	Board   Board     `json:"board"`
	Players [2]string `json:"players"`
	Turn    int       `json:"turn"`   // 1 or 2
	Winner  int       `json:"winner"` // 0 none, 1/2 player, 3 draw
	LastCol int       `json:"last_col"`
	Score   [2]int    `json:"score"`
}

// CleanText removes control characters, trims and truncates to n runes.
func CleanText(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '​' || unicode.Is(unicode.Bidi_Control, r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return s
}

// CleanName normalizes a player name.
func CleanName(s string) string {
	s = CleanText(s, MaxName)
	s = strings.Join(strings.Fields(s), "_")
	if s == "" {
		s = "player"
	}
	return s
}
