package online

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"
)

// Client is a connection to an Online Room.
type Client struct {
	conn     net.Conn
	Name     string
	Incoming chan Msg
	mu       sync.Mutex
	closed   bool
}

// Dial connects to addr and performs the handshake.
func Dial(addr, name, password string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, Incoming: make(chan Msg, 128)}
	if err := send(conn, Msg{T: "hello", V: ProtocolVersion, Name: name, Pass: password}); err != nil {
		conn.Close()
		return nil, err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 1024), 64*1024)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if !sc.Scan() {
		conn.Close()
		return nil, errors.New("no answer from the room")
	}
	var first Msg
	if err := json.Unmarshal(sc.Bytes(), &first); err != nil {
		conn.Close()
		return nil, errors.New("invalid answer (is this a TermVerse room?)")
	}
	if first.T == "error" {
		conn.Close()
		return nil, errors.New(first.Text)
	}
	if first.T != "welcome" {
		conn.Close()
		return nil, errors.New("unexpected handshake answer")
	}
	conn.SetReadDeadline(time.Time{})
	c.Name = first.Name
	go func() {
		for sc.Scan() {
			var m Msg
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				c.Incoming <- m
			}
		}
		close(c.Incoming)
	}()
	return c, nil
}

// Send sends a message to the room.
func (c *Client) Send(m Msg) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("disconnected")
	}
	return send(c.conn, m)
}

// Close disconnects.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// LANAddresses lists the IPv4 addresses others can use to join.
func LANAddresses() []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				out = append(out, ipn.IP.String())
			}
		}
	}
	return out
}
