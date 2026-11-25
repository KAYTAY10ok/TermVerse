package netinfo

import (
	"net"
	"testing"
	"time"
)

func TestParseTarget(t *testing.T) {
	ok := map[string]string{"example.com:443": "example.com:443", "example.com 80": "example.com:80", "[::1]:22": "[::1]:22", "h:https": "h:https"}
	for in, want := range ok {
		got, err := ParseTarget(in)
		if err != nil || got != want {
			t.Errorf("ParseTarget(%q)=%q,%v", in, got, err)
		}
	}
	for _, bad := range []string{"", "example.com", ":80", "host:notaport"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("ParseTarget(%q) should fail", bad)
		}
	}
}

func TestTCPTestLocal(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	ps := TCPTest(l.Addr().String(), 2, time.Second)
	for _, p := range ps {
		if p.Err != nil {
			t.Fatal(p.Err)
		}
	}
}

func TestInterfaces(t *testing.T) {
	ifs, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ifs) == 0 {
		t.Skip("no interfaces")
	}
}

func TestLookupEmpty(t *testing.T) {
	if r := Lookup(" "); r.Err == nil {
		t.Error("expected error")
	}
}
