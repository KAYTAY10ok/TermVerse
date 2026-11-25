package filemanager

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// Entry is a directory entry.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	IsLink  bool
	Target  string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	Err     error
}

// ReadDir lists dir, sorted with directories first.
func ReadDir(dir string, hidden bool, sortBy string) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		if !hidden && strings.HasPrefix(de.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, de.Name())
		e := Entry{Name: de.Name(), Path: p}
		info, err := os.Lstat(p)
		if err != nil {
			e.Err = err
			out = append(out, e)
			continue
		}
		e.Mode, e.Size, e.ModTime = info.Mode(), info.Size(), info.ModTime()
		e.IsDir = info.IsDir()
		if info.Mode()&fs.ModeSymlink != 0 {
			e.IsLink = true
			e.Target, _ = os.Readlink(p)
			if ti, err := os.Stat(p); err == nil && ti.IsDir() {
				e.IsDir = true
			}
		}
		out = append(out, e)
	}
	SortEntries(out, sortBy)
	return out, nil
}

// SortEntries sorts directories first, then by name, size or time.
func SortEntries(es []Entry, by string) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		switch by {
		case "size":
			if a.Size != b.Size {
				return a.Size > b.Size
			}
		case "time":
			if !a.ModTime.Equal(b.ModTime) {
				return a.ModTime.After(b.ModTime)
			}
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

// HumanSize formats a file size compactly.
func HumanSize(n int64) string {
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	for _, s := range []string{"K", "M", "G", "T"} {
		f /= u
		if f < u {
			return fmt.Sprintf("%.1f %s", f, s)
		}
	}
	return fmt.Sprintf("%.1f P", f/u)
}

// Owner returns "user:group" of a file (numeric if names are unknown).
func Owner(info fs.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "?"
	}
	uid, gid := strconv.Itoa(int(st.Uid)), strconv.Itoa(int(st.Gid))
	un, gn := uid, gid
	if u, err := user.LookupId(uid); err == nil {
		un = u.Username
	}
	if g, err := user.LookupGroupId(gid); err == nil {
		gn = g.Name
	}
	return un + ":" + gn
}

// Preview reads up to limit bytes of a file and reports whether it looks binary.
func Preview(path string, limit int64) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return "", false, err
	}
	if bytes.IndexByte(buf, 0) >= 0 || !utf8.Valid(trimPartialRune(buf)) {
		return "", true, nil
	}
	return string(buf), false, nil
}

func trimPartialRune(b []byte) []byte {
	for i := 0; i < 3 && len(b) > 0; i++ {
		r, _ := utf8.DecodeLastRune(b)
		if r != utf8.RuneError {
			return b
		}
		b = b[:len(b)-1]
	}
	return b
}

// HexDump returns a classic hex dump of the first bytes of a file.
func HexDump(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, n)
	m, _ := io.ReadFull(f, buf)
	buf = buf[:m]
	var b strings.Builder
	for off := 0; off < len(buf); off += 16 {
		end := min(off+16, len(buf))
		fmt.Fprintf(&b, "%08x  ", off)
		for i := off; i < off+16; i++ {
			if i < end {
				fmt.Fprintf(&b, "%02x ", buf[i])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" ")
		for _, c := range buf[off:end] {
			if c >= 32 && c < 127 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// DirSummary counts the entries of a directory.
func DirSummary(path string) (dirs, files int, err error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}
	for _, d := range des {
		if d.IsDir() {
			dirs++
		} else {
			files++
		}
	}
	return
}
