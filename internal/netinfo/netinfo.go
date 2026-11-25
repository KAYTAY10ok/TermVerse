// Package netinfo provides read-only network diagnostics. Every function is
// triggered explicitly by the user; nothing runs in the background.
package netinfo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// Interface describes a network interface.
type Interface struct {
	Name, MAC       string
	MTU             int
	Up, Loopback    bool
	Addrs           []string
	BytesSent, Recv uint64
	HasCounters     bool
}

// Interfaces lists network interfaces with their addresses and counters.
func Interfaces() ([]Interface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	counters := map[string]gnet.IOCountersStat{}
	if cs, err := gnet.IOCounters(true); err == nil {
		for _, c := range cs {
			counters[c.Name] = c
		}
	}
	out := make([]Interface, 0, len(ifs))
	for _, i := range ifs {
		it := Interface{Name: i.Name, MAC: i.HardwareAddr.String(), MTU: i.MTU,
			Up: i.Flags&net.FlagUp != 0, Loopback: i.Flags&net.FlagLoopback != 0}
		if addrs, err := i.Addrs(); err == nil {
			for _, a := range addrs {
				it.Addrs = append(it.Addrs, a.String())
			}
		}
		if c, ok := counters[i.Name]; ok {
			it.BytesSent, it.Recv, it.HasCounters = c.BytesSent, c.BytesRecv, true
		}
		out = append(out, it)
	}
	return out, nil
}

// DNSResult holds the result of a DNS lookup.
type DNSResult struct {
	Query    string
	Duration time.Duration
	IPs      []string
	CNAME    string
	MX, NS   []string
	TXT      []string
	PTR      []string
	Err      error
}

// Lookup resolves host (or reverse-resolves an IP) with a timeout.
func Lookup(host string) DNSResult {
	host = strings.TrimSpace(host)
	r := DNSResult{Query: host}
	if host == "" {
		r.Err = errors.New("empty host name")
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	res := net.DefaultResolver
	start := time.Now()
	if ip := net.ParseIP(host); ip != nil {
		r.PTR, r.Err = res.LookupAddr(ctx, host)
		r.Duration = time.Since(start)
		return r
	}
	addrs, err := res.LookupIPAddr(ctx, host)
	r.Duration = time.Since(start)
	if err != nil {
		r.Err = err
		return r
	}
	for _, a := range addrs {
		r.IPs = append(r.IPs, a.IP.String())
	}
	if c, err := res.LookupCNAME(ctx, host); err == nil && strings.TrimSuffix(c, ".") != host {
		r.CNAME = c
	}
	if mx, err := res.LookupMX(ctx, host); err == nil {
		for _, m := range mx {
			r.MX = append(r.MX, fmt.Sprintf("%d %s", m.Pref, m.Host))
		}
	}
	if ns, err := res.LookupNS(ctx, host); err == nil {
		for _, n := range ns {
			r.NS = append(r.NS, n.Host)
		}
	}
	if txt, err := res.LookupTXT(ctx, host); err == nil {
		r.TXT = txt
	}
	return r
}

// Probe is one TCP connection attempt.
type Probe struct {
	Latency time.Duration
	Remote  string
	Err     error
}

// ParseTarget accepts "host:port", "host port" or "[v6]:port".
func ParseTarget(s string) (string, error) {
	s = strings.TrimSpace(s)
	if f := strings.Fields(s); len(f) == 2 {
		s = net.JoinHostPort(f[0], f[1])
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", fmt.Errorf("expected host:port (e.g. example.com:443)")
	}
	if host == "" || port == "" {
		return "", fmt.Errorf("expected host:port (e.g. example.com:443)")
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return "", fmt.Errorf("invalid port %q", port)
	}
	return net.JoinHostPort(host, port), nil
}

// TCPTest opens up to n TCP connections to addr (closed immediately) and
// measures connection latency.
func TCPTest(addr string, n int, timeout time.Duration) []Probe {
	out := make([]Probe, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		c, err := net.DialTimeout("tcp", addr, timeout)
		p := Probe{Latency: time.Since(start), Err: err}
		if err == nil {
			p.Remote = c.RemoteAddr().String()
			c.Close()
		}
		out = append(out, p)
		if i < n-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return out
}

// Conn is a socket.
type Conn struct {
	Proto, Local, Remote, Status string
	PID                          int32
	Process                      string
}

// ConnStats summarizes sockets.
type ConnStats struct {
	ByStatus map[string]int
	ByProto  map[string]int
	Total    int
}

func protoName(family, typ uint32) string {
	p := "tcp"
	if typ == 2 {
		p = "udp"
	}
	if family == 10 {
		p += "6"
	}
	return p
}

func addr(a gnet.Addr) string {
	if a.IP == "" && a.Port == 0 {
		return "*"
	}
	return net.JoinHostPort(a.IP, fmt.Sprint(a.Port))
}

// Connections lists inet sockets. Without root, other users' PIDs are unknown.
func Connections() ([]Conn, ConnStats, error) {
	cs, err := gnet.Connections("inet")
	st := ConnStats{ByStatus: map[string]int{}, ByProto: map[string]int{}}
	if err != nil {
		return nil, st, err
	}
	names := map[int32]string{}
	out := make([]Conn, 0, len(cs))
	for _, c := range cs {
		status := c.Status
		if status == "" || status == "NONE" {
			status = "-"
		}
		cn := Conn{Proto: protoName(c.Family, c.Type), Local: addr(c.Laddr), Remote: addr(c.Raddr), Status: status, PID: c.Pid}
		if c.Pid > 0 {
			n, ok := names[c.Pid]
			if !ok {
				if p, err := process.NewProcess(c.Pid); err == nil {
					n, _ = p.Name()
				}
				names[c.Pid] = n
			}
			cn.Process = n
		}
		out = append(out, cn)
		st.ByStatus[status]++
		st.ByProto[cn.Proto]++
		st.Total++
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status < out[j].Status
		}
		return out[i].Local < out[j].Local
	})
	return out, st, nil
}

// Listening returns TCP sockets in LISTEN state and bound UDP sockets.
func Listening() ([]Conn, error) {
	all, _, err := Connections()
	if err != nil {
		return nil, err
	}
	var out []Conn
	seen := map[string]bool{}
	for _, c := range all {
		listen := c.Status == "LISTEN" || (strings.HasPrefix(c.Proto, "udp") && c.Remote == "*")
		key := c.Proto + c.Local
		if listen && !seen[key] {
			seen[key] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return portOf(out[i].Local) < portOf(out[j].Local) })
	return out, nil
}

func portOf(a string) int {
	_, p, err := net.SplitHostPort(a)
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscan(p, &n)
	return n
}
