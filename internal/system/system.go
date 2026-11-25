// Package system collects real information about the host machine. Values
// that cannot be read are reported as unavailable — never simulated.
package system

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

// NA is displayed for unavailable values.
const NA = "N/A"

// Static is information that does not change while TermVerse runs.
type Static struct {
	User     string
	Hostname string
	OS       string // e.g. "ubuntu 24.04"
	Kernel   string
	Arch     string
	CPUModel string
	Cores    int // physical
	Threads  int // logical
	Virt     string
}

// Disk describes a mounted filesystem.
type Disk struct {
	Mount, Device, FSType string
	Total, Used, Free     uint64
	Percent               float64
}

// Temp is a temperature sensor reading in °C.
type Temp struct {
	Name  string
	Value float64
}

// Snapshot is a point-in-time view of the system.
type Snapshot struct {
	Time time.Time
	Static

	CPUOK      bool
	CPUPercent float64
	PerCore    []float64

	MemOK                       bool
	MemTotal, MemUsed, MemAvail uint64
	MemPercent                  float64
	SwapOK                      bool
	SwapTotal, SwapUsed         uint64
	SwapPercent                 float64

	RootDiskOK bool
	RootDisk   Disk
	Disks      []Disk // detailed only

	UptimeOK bool
	Uptime   time.Duration

	ProcsOK bool
	Procs   int

	LoadOK               bool
	Load1, Load5, Load15 float64

	LocalIP string
	Temps   []Temp // detailed only
}

// Collector gathers snapshots. It keeps the previous CPU sample so CPU usage
// is computed over the refresh interval. It is safe for concurrent use.
type Collector struct {
	mu        sync.Mutex
	static    Static
	staticOK  bool
	prevTotal []cpu.TimesStat
	prevCores []cpu.TimesStat
	procPrev  map[int32]procSample
	procAt    time.Time
}

// NewCollector returns a collector.
func NewCollector() *Collector { return &Collector{procPrev: map[int32]procSample{}} }

func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return NA
	}
	return strings.TrimSpace(s)
}

// StaticInfo returns (and caches) static host information.
func (c *Collector) StaticInfo() Static {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.staticLocked()
}

func (c *Collector) staticLocked() Static {
	if c.staticOK {
		return c.static
	}
	s := Static{Arch: runtime.GOARCH}
	if u, err := user.Current(); err == nil {
		s.User = u.Username
	} else {
		s.User = os.Getenv("USER")
	}
	s.Hostname, _ = os.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if hi, err := host.InfoWithContext(ctx); err == nil {
		s.OS = strings.TrimSpace(hi.Platform + " " + hi.PlatformVersion)
		if s.OS == "" {
			s.OS = hi.OS
		}
		s.Kernel = hi.KernelVersion
		if hi.KernelArch != "" {
			s.Arch = hi.KernelArch
		}
		if hi.VirtualizationSystem != "" {
			s.Virt = hi.VirtualizationSystem + " (" + hi.VirtualizationRole + ")"
		}
	}
	if pretty := osReleasePretty(); pretty != "" {
		s.OS = pretty
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		s.CPUModel = infos[0].ModelName
	}
	if n, err := cpu.CountsWithContext(ctx, false); err == nil {
		s.Cores = n
	}
	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		s.Threads = n
	}
	s.User, s.Hostname, s.OS, s.Kernel = orNA(s.User), orNA(s.Hostname), orNA(s.OS), orNA(s.Kernel)
	s.CPUModel, s.Virt = orNA(s.CPUModel), orNA(s.Virt)
	c.static, c.staticOK = s, true
	return s
}

func osReleasePretty() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// busyTotal returns busy and total jiffies of a CPU sample.
func busyTotal(t cpu.TimesStat) (busy, total float64) {
	total = t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal
	busy = total - t.Idle - t.Iowait
	return
}

// Usage computes the CPU usage percentage between two samples.
func Usage(prev, cur cpu.TimesStat) float64 {
	pb, pt := busyTotal(prev)
	cb, ct := busyTotal(cur)
	dt := ct - pt
	if dt <= 0 {
		return 0
	}
	u := (cb - pb) / dt * 100
	if u < 0 {
		return 0
	}
	if u > 100 {
		return 100
	}
	return u
}

// Collect gathers a snapshot. When detailed is true it also lists all disks
// and temperature sensors (used by System Center).
func (c *Collector) Collect(detailed bool) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	s := Snapshot{Time: time.Now(), Static: c.staticLocked()}

	if t, err := cpu.TimesWithContext(ctx, false); err == nil && len(t) > 0 {
		if len(c.prevTotal) > 0 {
			s.CPUPercent, s.CPUOK = Usage(c.prevTotal[0], t[0]), true
		} else if p, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false); err == nil && len(p) > 0 {
			s.CPUPercent, s.CPUOK = p[0], true
			t, _ = cpu.TimesWithContext(ctx, false)
		}
		c.prevTotal = t
	}
	if t, err := cpu.TimesWithContext(ctx, true); err == nil {
		if len(c.prevCores) == len(t) {
			s.PerCore = make([]float64, len(t))
			for i := range t {
				s.PerCore[i] = Usage(c.prevCores[i], t[i])
			}
		}
		c.prevCores = t
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.MemOK = true
		s.MemTotal, s.MemUsed, s.MemAvail, s.MemPercent = vm.Total, vm.Used, vm.Available, vm.UsedPercent
	}
	if sw, err := mem.SwapMemoryWithContext(ctx); err == nil {
		s.SwapOK = true
		s.SwapTotal, s.SwapUsed, s.SwapPercent = sw.Total, sw.Used, sw.UsedPercent
	}
	if du, err := disk.UsageWithContext(ctx, "/"); err == nil {
		s.RootDiskOK = true
		s.RootDisk = Disk{Mount: "/", FSType: du.Fstype, Total: du.Total, Used: du.Used, Free: du.Free, Percent: du.UsedPercent}
	}
	if up, err := host.UptimeWithContext(ctx); err == nil {
		s.UptimeOK, s.Uptime = true, time.Duration(up)*time.Second
	}
	if pids, err := process.PidsWithContext(ctx); err == nil {
		s.ProcsOK, s.Procs = true, len(pids)
	}
	if l, err := load.AvgWithContext(ctx); err == nil {
		s.LoadOK, s.Load1, s.Load5, s.Load15 = true, l.Load1, l.Load5, l.Load15
	}
	s.LocalIP = LocalIP()
	if detailed {
		s.Disks = listDisks(ctx)
		s.Temps = temps(ctx)
	}
	return s
}

func listDisks(ctx context.Context) []Disk {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Disk
	for _, p := range parts {
		// Read-only image mounts (snaps, squashfs bundles) are always 100% full
		// and only add noise, like `df -x squashfs`.
		if seen[p.Mountpoint] || strings.HasPrefix(p.Mountpoint, "/snap/") || p.Fstype == "squashfs" {
			continue
		}
		u, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || u.Total == 0 {
			continue
		}
		seen[p.Mountpoint] = true
		out = append(out, Disk{Mount: p.Mountpoint, Device: p.Device, FSType: p.Fstype,
			Total: u.Total, Used: u.Used, Free: u.Free, Percent: u.UsedPercent})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out
}

func temps(ctx context.Context) []Temp {
	ts, err := sensors.TemperaturesWithContext(ctx)
	if err != nil && len(ts) == 0 {
		return nil
	}
	var out []Temp
	for _, t := range ts {
		if t.Temperature <= 0 || t.Temperature > 150 {
			continue
		}
		out = append(out, Temp{Name: t.SensorKey, Value: t.Temperature})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// LocalIP returns the IPv4 address of the default-route interface, falling
// back to the first non-loopback IPv4 address. It sends no network traffic.
func LocalIP() string {
	ifaceName := defaultRouteIface()
	ifaces, err := net.Interfaces()
	if err != nil {
		return NA
	}
	pick := func(ifc net.Interface) string {
		addrs, err := ifc.Addrs()
		if err != nil {
			return ""
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
				return ipn.IP.String()
			}
		}
		return ""
	}
	if ifaceName != "" {
		for _, ifc := range ifaces {
			if ifc.Name == ifaceName {
				if ip := pick(ifc); ip != "" {
					return ip
				}
			}
		}
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if ip := pick(ifc); ip != "" {
			return ip
		}
	}
	return NA
}

func defaultRouteIface() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) > 2 && fields[1] == "00000000" {
			return fields[0]
		}
	}
	return ""
}

// HumanBytes formats a byte count using binary units.
func HumanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// HumanDuration formats an uptime like "3d 4h 12m".
func HumanDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
