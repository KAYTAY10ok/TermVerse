package online

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type client struct {
	conn   net.Conn
	name   string
	out    chan Msg
	tokens float64
	last   time.Time
}

// Server is an Online Room host.
type Server struct {
	ln       net.Listener
	password string
	mu       sync.Mutex
	clients  map[*client]bool
	game     GameState
	queue    []string // seated players wanting to play (max 2)
	closed   bool
	wg       sync.WaitGroup
}

// Listen starts a server on addr (e.g. ":7777"). An empty password means open room.
func Listen(addr, password string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, password: password, clients: map[*client]bool{}}
	s.resetGame()
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

// Addr returns the listening address.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// Close stops the server and disconnects everyone.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for c := range s.clients {
		c.conn.Close()
	}
	s.mu.Unlock()
	err := s.ln.Close()
	s.wg.Wait()
	return err
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

func send(conn net.Conn, m Msg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(append(b, '\n'))
	return err
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 1024), MaxLine)
	// Handshake within 10 seconds.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if !sc.Scan() {
		return
	}
	var hello Msg
	if json.Unmarshal(sc.Bytes(), &hello) != nil || hello.T != "hello" {
		send(conn, Msg{T: "error", Text: "invalid handshake"})
		return
	}
	if hello.V != ProtocolVersion {
		send(conn, Msg{T: "error", Text: fmt.Sprintf("protocol version mismatch (server %d, client %d)", ProtocolVersion, hello.V)})
		return
	}
	if s.password != "" && subtle.ConstantTimeCompare([]byte(hello.Pass), []byte(s.password)) != 1 {
		time.Sleep(500 * time.Millisecond)
		send(conn, Msg{T: "error", Text: "wrong room password"})
		return
	}
	c, err := s.register(conn, CleanName(hello.Name))
	if err != nil {
		send(conn, Msg{T: "error", Text: err.Error()})
		return
	}
	conn.SetReadDeadline(time.Time{})
	done := make(chan struct{})
	go func() { // writer
		for {
			select {
			case m := <-c.out:
				if send(conn, m) != nil {
					conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()
	c.out <- Msg{T: "welcome", Name: c.name}
	s.broadcastState(Msg{T: "sys", Text: c.name + " joined the room"})
	for sc.Scan() {
		var m Msg
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		if !c.allow() {
			c.trySend(Msg{T: "error", Text: "slow down"})
			continue
		}
		s.dispatch(c, m)
	}
	close(done)
	s.unregister(c)
}

// allow implements a token bucket: 5 messages/s, burst 10.
func (c *client) allow() bool {
	now := time.Now()
	c.tokens = min(10, c.tokens+now.Sub(c.last).Seconds()*5)
	c.last = now
	if c.tokens < 1 {
		return false
	}
	c.tokens--
	return true
}

func (c *client) trySend(m Msg) {
	select {
	case c.out <- m:
	default: // slow client: drop message rather than block the room
	}
}

func (s *Server) register(conn net.Conn, name string) (*client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("room closed")
	}
	if len(s.clients) >= MaxClients {
		return nil, fmt.Errorf("room is full (%d players)", MaxClients)
	}
	base, n := name, 2
	for s.nameTaken(name) {
		name = CleanName(fmt.Sprintf("%s%d", base, n))
		n++
	}
	c := &client{conn: conn, name: name, out: make(chan Msg, 64), tokens: 10, last: time.Now()}
	s.clients[c] = true
	return c, nil
}

func (s *Server) nameTaken(n string) bool {
	for c := range s.clients {
		if c.name == n {
			return true
		}
	}
	return false
}

func (s *Server) unregister(c *client) {
	s.mu.Lock()
	delete(s.clients, c)
	left := false
	for i, p := range s.game.Players {
		if p == c.name {
			s.game.Players[i] = ""
			left = true
		}
	}
	if left && s.game.Winner == 0 {
		s.game.Board = Board{}
		s.game.Turn = 1
	}
	s.mu.Unlock()
	s.broadcastState(Msg{T: "sys", Text: c.name + " left the room"})
}

func (s *Server) resetGame() {
	score := s.game.Score
	players := s.game.Players
	s.game = GameState{Turn: 1, LastCol: -1, Score: score, Players: players}
}

// broadcastState sends m (if any), the roster and the game state to everyone.
func (s *Server) broadcastState(m Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.clients))
	for c := range s.clients {
		names = append(names, c.name)
	}
	g := s.game
	for c := range s.clients {
		if m.T != "" {
			c.trySend(m)
		}
		c.trySend(Msg{T: "roster", Names: names})
		gg := g
		c.trySend(Msg{T: "game", Game: &gg})
	}
}

func (s *Server) broadcast(m Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.trySend(m)
	}
}

func (s *Server) dispatch(c *client, m Msg) {
	switch m.T {
	case "chat":
		text := CleanText(m.Text, MaxChat)
		if text != "" {
			s.broadcast(Msg{T: "chat", From: c.name, Text: text})
		}
	case "join":
		s.mu.Lock()
		seat := -1
		for i, p := range s.game.Players {
			if p == c.name {
				seat = i
			}
		}
		if seat < 0 {
			for i, p := range s.game.Players {
				if p == "" {
					s.game.Players[i] = c.name
					seat = i
					break
				}
			}
		}
		s.mu.Unlock()
		if seat < 0 {
			c.trySend(Msg{T: "error", Text: "both seats are taken"})
			return
		}
		s.broadcastState(Msg{T: "sys", Text: fmt.Sprintf("%s sits as player %d", c.name, seat+1)})
	case "leave":
		s.mu.Lock()
		for i, p := range s.game.Players {
			if p == c.name {
				s.game.Players[i] = ""
				s.game.Board, s.game.Turn, s.game.Winner = Board{}, 1, 0
			}
		}
		s.mu.Unlock()
		s.broadcastState(Msg{T: "sys", Text: c.name + " left the game"})
	case "reset":
		s.mu.Lock()
		isPlayer := s.game.Players[0] == c.name || s.game.Players[1] == c.name
		if isPlayer {
			s.resetGame()
		}
		s.mu.Unlock()
		if isPlayer {
			s.broadcastState(Msg{T: "sys", Text: c.name + " started a new game"})
		}
	case "move":
		s.mu.Lock()
		g := &s.game
		var errText, sys string
		switch {
		case g.Players[0] == "" || g.Players[1] == "":
			errText = "waiting for two players"
		case g.Winner != 0:
			errText = "game over — press n for a new game"
		case g.Players[g.Turn-1] != c.name:
			errText = "not your turn"
		default:
			if g.Board.Drop(m.Col, g.Turn) < 0 {
				errText = "column is full"
				break
			}
			g.LastCol = m.Col
			if w := g.Board.Winner(); w != 0 {
				g.Winner = w
				if w == 3 {
					sys = "draw!"
				} else {
					g.Score[w-1]++
					sys = g.Players[w-1] + " wins!"
				}
			} else {
				g.Turn = 3 - g.Turn
			}
		}
		s.mu.Unlock()
		if errText != "" {
			c.trySend(Msg{T: "error", Text: errText})
			return
		}
		s.broadcastState(Msg{T: "sys", Text: sys})
	}
}
