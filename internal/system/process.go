package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Proc describes a running process.
type Proc struct {
	PID     int32
	Name    string
	User    string
	Status  string
	CPU     float64 // % of one core over the last interval
	RSS     uint64
	MemPct  float32
	Cmdline string
}

type procSample struct {
	cpuSeconds float64
}

// Processes lists running processes. CPU usage is computed between two
// successive calls; the first call reports 0% for every process.
func (c *Collector) Processes() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ps, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	prev, prevAt := c.procPrev, c.procAt
	c.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(prevAt).Seconds()
	next := make(map[int32]procSample, len(ps))
	out := make([]Proc, 0, len(ps))
	for _, p := range ps {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue // process exited
		}
		pr := Proc{PID: p.Pid, Name: name}
		if t, err := p.TimesWithContext(ctx); err == nil {
			total := t.User + t.System
			next[p.Pid] = procSample{cpuSeconds: total}
			if old, ok := prev[p.Pid]; ok && elapsed > 0 {
				pr.CPU = (total - old.cpuSeconds) / elapsed * 100
				if pr.CPU < 0 {
					pr.CPU = 0
				}
			}
		}
		if mi, err := p.MemoryInfoWithContext(ctx); err == nil {
			pr.RSS = mi.RSS
		}
		pr.MemPct, _ = p.MemoryPercentWithContext(ctx)
		pr.User, _ = p.UsernameWithContext(ctx)
		if st, err := p.StatusWithContext(ctx); err == nil && len(st) > 0 {
			pr.Status = st[0]
		}
		pr.Cmdline, _ = p.CmdlineWithContext(ctx)
		out = append(out, pr)
	}
	c.mu.Lock()
	c.procPrev, c.procAt = next, now
	c.mu.Unlock()
	return out, nil
}

// SortProcs sorts processes by "cpu", "mem", "pid" or "name".
func SortProcs(ps []Proc, by string) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		switch by {
		case "mem":
			return a.RSS > b.RSS
		case "pid":
			return a.PID < b.PID
		case "name":
			return a.Name < b.Name
		default:
			if a.CPU == b.CPU {
				return a.RSS > b.RSS
			}
			return a.CPU > b.CPU
		}
	})
}

// Signal sends sig to pid. It refuses PID 1 and TermVerse itself.
func Signal(pid int32, sig syscall.Signal) error {
	if pid <= 1 {
		return errors.New("refusing to signal PID <= 1")
	}
	if int(pid) == os.Getpid() {
		return errors.New("refusing to signal TermVerse itself")
	}
	if err := syscall.Kill(int(pid), sig); err != nil {
		if errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("permission denied (process belongs to another user)")
		}
		return err
	}
	return nil
}
