package system

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 1 << 30: "1.0 GiB"}
	for in, want := range cases {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d)=%s want %s", in, got, want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	if got := HumanDuration(26*time.Hour + 5*time.Minute); got != "1d 2h 5m" {
		t.Error(got)
	}
	if got := HumanDuration(42 * time.Minute); got != "42m" {
		t.Error(got)
	}
}

func TestUsage(t *testing.T) {
	prev := cpu.TimesStat{User: 100, Idle: 100}
	cur := cpu.TimesStat{User: 150, Idle: 150}
	if u := Usage(prev, cur); u != 50 {
		t.Errorf("usage %v", u)
	}
	if u := Usage(cur, cur); u != 0 {
		t.Errorf("zero delta %v", u)
	}
}

func TestCollectReal(t *testing.T) {
	c := NewCollector()
	s := c.Collect(true)
	if s.Hostname == "" || s.Arch == "" {
		t.Error("static info empty")
	}
	if !s.MemOK || s.MemTotal == 0 {
		t.Log("memory info unavailable on this host")
	}
}

func TestSignalRefusesInit(t *testing.T) {
	if err := Signal(1, 15); err == nil {
		t.Error("expected refusal")
	}
}
