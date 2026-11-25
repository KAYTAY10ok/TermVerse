package online

import (
	"strings"
	"testing"
	"time"
)

func TestConnect4(t *testing.T) {
	var b Board
	for i := 0; i < 3; i++ {
		b.Drop(0, 1)
		b.Drop(1, 2)
	}
	if b.Winner() != 0 {
		t.Fatal("no winner yet")
	}
	b.Drop(0, 1)
	if b.Winner() != 1 {
		t.Fatal("vertical win not detected")
	}
	var d Board
	// diagonal for player 2
	d.Drop(0, 2)
	d.Drop(1, 1)
	d.Drop(1, 2)
	d.Drop(2, 1)
	d.Drop(2, 1)
	d.Drop(2, 2)
	d.Drop(3, 1)
	d.Drop(3, 1)
	d.Drop(3, 1)
	d.Drop(3, 2)
	if d.Winner() != 2 {
		t.Fatal("diagonal win not detected")
	}
	var full Board
	for c := 0; c < Cols; c++ {
		for r := 0; r < Rows; r++ {
			full.Drop(c, 1)
		}
		if full.Drop(c, 1) != -1 {
			t.Fatal("full column accepted")
		}
	}
}

func TestCleanText(t *testing.T) {
	if got := CleanText("hi\x1b[31m‮ there\n", 50); got != "hi[31m there" {
		t.Errorf("%q", got)
	}
	if got := CleanName("  bob the builder with a very long name "); got != "bob_the_builder" {
		t.Errorf("%q", got)
	}
	if CleanName("\x00") != "player" {
		t.Error("empty name")
	}
}

func wait(t *testing.T, c *Client, pred func(Msg) bool) Msg {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case m, ok := <-c.Incoming:
			if !ok {
				t.Fatal("connection closed")
			}
			if pred(m) {
				return m
			}
		case <-timeout:
			t.Fatal("timeout waiting for message")
		}
	}
}

func TestRoomEndToEnd(t *testing.T) {
	srv, err := Listen("127.0.0.1:0", "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	addr := srv.Addr().String()
	if _, err := Dial(addr, "eve", "wrong"); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("wrong password accepted: %v", err)
	}
	a, err := Dial(addr, "alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Dial(addr, "alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if b.Name == "alice" {
		t.Fatal("duplicate name not renamed")
	}
	a.Send(Msg{T: "chat", Text: "hello \x07world"})
	m := wait(t, b, func(m Msg) bool { return m.T == "chat" })
	if m.From != "alice" || m.Text != "hello world" {
		t.Fatalf("%+v", m)
	}
	a.Send(Msg{T: "join"})
	wait(t, b, func(m Msg) bool { return m.T == "game" && m.Game.Players[0] == "alice" })
	b.Send(Msg{T: "join"})
	wait(t, a, func(m Msg) bool { return m.T == "game" && m.Game.Players[1] == b.Name })
	b.Send(Msg{T: "move", Col: 3})
	wait(t, b, func(m Msg) bool { return m.T == "error" && m.Text == "not your turn" })
	for i := 0; i < 3; i++ {
		a.Send(Msg{T: "move", Col: 0})
		wait(t, b, func(m Msg) bool { return m.T == "game" && m.Game.Turn == 2 })
		b.Send(Msg{T: "move", Col: 1})
		wait(t, a, func(m Msg) bool { return m.T == "game" && m.Game.Turn == 1 })
	}
	a.Send(Msg{T: "move", Col: 0})
	g := wait(t, b, func(m Msg) bool { return m.T == "game" && m.Game.Winner != 0 })
	if g.Game.Winner != 1 || g.Game.Score[0] != 1 {
		t.Fatalf("%+v", g.Game)
	}
}

func TestRoomFullAndVersion(t *testing.T) {
	srv, err := Listen("127.0.0.1:0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	var cs []*Client
	for i := 0; i < MaxClients; i++ {
		c, err := Dial(srv.Addr().String(), "p", "")
		if err != nil {
			t.Fatal(err)
		}
		cs = append(cs, c)
	}
	if _, err := Dial(srv.Addr().String(), "late", ""); err == nil {
		t.Fatal("room should be full")
	}
	for _, c := range cs {
		c.Close()
	}
}
