package devtools

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ---- JSON -------------------------------------------------------------------

// JSONError locates a syntax error.
type JSONError struct {
	Line, Col int
	Msg       string
}

func (e *JSONError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Col, e.Msg)
}

func lineCol(data string, off int64) (int, int) {
	if off > int64(len(data)) {
		off = int64(len(data))
	}
	before := data[:off]
	line := strings.Count(before, "\n") + 1
	col := int(off) - strings.LastIndex(before, "\n")
	return line, col
}

// FormatJSON validates and re-indents JSON (indent "" = minify).
func FormatJSON(in, indent string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", errors.New("empty input")
	}
	var buf bytes.Buffer
	var err error
	if indent == "" {
		err = json.Compact(&buf, []byte(in))
	} else {
		err = json.Indent(&buf, []byte(in), "", indent)
	}
	if err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			l, c := lineCol(in, se.Offset)
			return "", &JSONError{l, c, se.Error()}
		}
		return "", err
	}
	// Reject trailing data after the first value (json.Indent accepts it in some cases).
	dec := json.NewDecoder(strings.NewReader(in))
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	if dec.More() {
		return "", errors.New("unexpected data after the JSON value")
	}
	return buf.String(), nil
}

// JSONStats describes a parsed JSON document.
func JSONStats(in string) string {
	var v any
	if json.Unmarshal([]byte(in), &v) != nil {
		return ""
	}
	var objs, arrs, leaves, depth int
	var walk func(any, int)
	walk = func(x any, d int) {
		depth = max(depth, d)
		switch t := x.(type) {
		case map[string]any:
			objs++
			for _, c := range t {
				walk(c, d+1)
			}
		case []any:
			arrs++
			for _, c := range t {
				walk(c, d+1)
			}
		default:
			leaves++
		}
	}
	walk(v, 1)
	return fmt.Sprintf("%d objects · %d arrays · %d values · depth %d", objs, arrs, leaves, depth)
}

// ---- Base64 -----------------------------------------------------------------

// Base64Encode encodes with the standard or URL-safe alphabet.
func Base64Encode(s string, url bool) string {
	if url {
		return base64.URLEncoding.EncodeToString([]byte(s))
	}
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// Base64Decode tries standard, URL-safe, padded and raw encodings.
func Base64Decode(s string) ([]byte, string, error) {
	s = strings.Join(strings.Fields(s), "")
	if s == "" {
		return nil, "", errors.New("empty input")
	}
	encs := []struct {
		name string
		enc  *base64.Encoding
	}{
		{"standard", base64.StdEncoding}, {"URL-safe", base64.URLEncoding},
		{"standard (no padding)", base64.RawStdEncoding}, {"URL-safe (no padding)", base64.RawURLEncoding},
	}
	for _, e := range encs {
		if b, err := e.enc.DecodeString(s); err == nil {
			return b, e.name, nil
		}
	}
	return nil, "", errors.New("not valid Base64")
}

// Printable renders bytes as text if valid UTF-8, otherwise as hex.
func Printable(b []byte) (string, bool) {
	if utf8.Valid(b) && !bytes.ContainsRune(b, 0) {
		return string(b), true
	}
	return hex.EncodeToString(b), false
}

// ---- Hashes -----------------------------------------------------------------

// HashResult is a named digest.
type HashResult struct{ Name, Hex string }

func hashers() []struct {
	name string
	h    hash.Hash
} {
	return []struct {
		name string
		h    hash.Hash
	}{
		{"MD5", md5.New()}, {"SHA-1", sha1.New()}, {"SHA-256", sha256.New()},
		{"SHA-512", sha512.New()}, {"CRC32", crc32.NewIEEE()},
	}
}

// HashReader computes all digests of r in one pass.
func HashReader(r io.Reader) ([]HashResult, int64, error) {
	hs := hashers()
	ws := make([]io.Writer, len(hs))
	for i := range hs {
		ws[i] = hs[i].h
	}
	n, err := io.Copy(io.MultiWriter(ws...), r)
	if err != nil {
		return nil, n, err
	}
	out := make([]HashResult, len(hs))
	for i, h := range hs {
		out[i] = HashResult{h.name, hex.EncodeToString(h.h.Sum(nil))}
	}
	return out, n, nil
}

// HashString hashes a string.
func HashString(s string) []HashResult {
	r, _, _ := HashReader(strings.NewReader(s))
	return r
}

// HashFile hashes a file.
func HashFile(path string) ([]HashResult, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	return HashReader(f)
}

// ---- Timestamps -------------------------------------------------------------

var layouts = []string{
	time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04",
	"2006-01-02", "02/01/2006 15:04", "02/01/2006", time.RFC1123, time.RFC1123Z, time.RFC822, time.ANSIC,
}

// ParseTime interprets s as a Unix timestamp (s, ms, µs, ns) or a date.
func ParseTime(s string, loc *time.Location) (time.Time, string, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "now") {
		return time.Now(), "now", nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		abs := n
		if abs < 0 {
			abs = -abs
		}
		switch {
		case abs < 1e11:
			return time.Unix(n, 0), "unix seconds", nil
		case abs < 1e14:
			return time.UnixMilli(n), "unix milliseconds", nil
		case abs < 1e17:
			return time.UnixMicro(n), "unix microseconds", nil
		default:
			return time.Unix(0, n), "unix nanoseconds", nil
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		sec, frac := math.Modf(f)
		return time.Unix(int64(sec), int64(frac*1e9)), "unix seconds (fractional)", nil
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, "date (" + l + ")", nil
		}
	}
	return time.Time{}, "", errors.New("unrecognized timestamp or date")
}

// Relative describes t relative to now ("3h ago", "in 2d").
func Relative(t, now time.Time) string {
	d := now.Sub(t)
	suffix := "ago"
	if d < 0 {
		d, suffix = -d, "from now"
	}
	var v string
	switch {
	case d < time.Minute:
		v = fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		v = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		v = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 730*24*time.Hour:
		v = fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		v = fmt.Sprintf("%.1fy", d.Hours()/24/365.25)
	}
	return v + " " + suffix
}

// ---- Passwords --------------------------------------------------------------

// PasswordOptions configures generation.
type PasswordOptions struct {
	Length                        int
	Upper, Lower, Digits, Symbols bool
	NoAmbiguous                   bool
}

const (
	lowerSet  = "abcdefghijklmnopqrstuvwxyz"
	upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet  = "0123456789"
	symbolSet = "!@#$%^&*()-_=+[]{};:,.<>?/~"
	ambiguous = "Il1O0o|`'\""
)

func filterSet(set string, noAmb bool) string {
	if !noAmb {
		return set
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(ambiguous, r) {
			return -1
		}
		return r
	}, set)
}

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(v.Int64())
}

// GeneratePassword uses crypto/rand and guarantees one char of each class.
func GeneratePassword(o PasswordOptions) (string, float64, error) {
	var sets []string
	for _, c := range []struct {
		on  bool
		set string
	}{{o.Lower, lowerSet}, {o.Upper, upperSet}, {o.Digits, digitSet}, {o.Symbols, symbolSet}} {
		if c.on {
			sets = append(sets, filterSet(c.set, o.NoAmbiguous))
		}
	}
	if len(sets) == 0 {
		return "", 0, errors.New("select at least one character class")
	}
	if o.Length < len(sets) || o.Length > 256 {
		return "", 0, fmt.Errorf("length must be between %d and 256", len(sets))
	}
	all := strings.Join(sets, "")
	out := make([]byte, 0, o.Length)
	for _, s := range sets {
		out = append(out, s[randInt(len(s))])
	}
	for len(out) < o.Length {
		out = append(out, all[randInt(len(all))])
	}
	for i := len(out) - 1; i > 0; i-- { // Fisher–Yates
		j := randInt(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	entropy := float64(o.Length) * math.Log2(float64(len(all)))
	return string(out), entropy, nil
}

// Strength labels an entropy in bits.
func Strength(bits float64) string {
	switch {
	case bits < 40:
		return "weak"
	case bits < 64:
		return "fair"
	case bits < 100:
		return "strong"
	default:
		return "very strong"
	}
}

// ---- Git --------------------------------------------------------------------

// GitInfo is a read-only summary of a repository.
type GitInfo struct {
	Dir, Root, Branch, Upstream, LastTag  string
	Ahead, Behind                         int
	Staged, Modified, Untracked, Conflict int
	Changes                               []string
	Remotes                               []string
	Log                                   []string
	Stashes                               int
	Err                                   error
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", errors.New(strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// ReadGit inspects the repository containing dir using read-only git commands.
func ReadGit(dir string) GitInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gi := GitInfo{Dir: dir}
	if _, err := exec.LookPath("git"); err != nil {
		gi.Err = errors.New("git is not installed")
		return gi
	}
	root, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		gi.Err = fmt.Errorf("not a Git repository (%v)", err)
		return gi
	}
	gi.Root = root
	status, _ := git(ctx, dir, "status", "--porcelain=v1", "--branch")
	for i, l := range strings.Split(status, "\n") {
		if l == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(l, "## ") {
			parseBranchLine(&gi, l[3:])
			continue
		}
		gi.Changes = append(gi.Changes, l)
		if len(l) < 2 {
			continue
		}
		x, y := l[0], l[1]
		switch {
		case x == '?' && y == '?':
			gi.Untracked++
		case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
			gi.Conflict++
		default:
			if x != ' ' {
				gi.Staged++
			}
			if y != ' ' {
				gi.Modified++
			}
		}
	}
	if b, err := git(ctx, dir, "branch", "--show-current"); err == nil && b != "" {
		gi.Branch = b
	}
	if gi.Branch == "" {
		if h, err := git(ctx, dir, "rev-parse", "--short", "HEAD"); err == nil {
			gi.Branch = "(detached " + h + ")"
		}
	}
	gi.LastTag, _ = git(ctx, dir, "describe", "--tags", "--abbrev=0")
	if r, err := git(ctx, dir, "remote", "-v"); err == nil && r != "" {
		seen := map[string]bool{}
		for _, l := range strings.Split(r, "\n") {
			f := strings.Fields(l)
			if len(f) >= 2 && !seen[f[0]+f[1]] {
				seen[f[0]+f[1]] = true
				gi.Remotes = append(gi.Remotes, f[0]+"  "+f[1])
			}
		}
	}
	if lg, err := git(ctx, dir, "log", "-n", "12", "--pretty=format:%h  %ad  %an  %s", "--date=short"); err == nil && lg != "" {
		gi.Log = strings.Split(lg, "\n")
	}
	if st, err := git(ctx, dir, "stash", "list"); err == nil && st != "" {
		gi.Stashes = len(strings.Split(st, "\n"))
	}
	return gi
}

func parseBranchLine(gi *GitInfo, l string) {
	// "main...origin/main [ahead 1, behind 2]" or "No commits yet on main"
	head, rest, _ := strings.Cut(l, " [")
	if b, up, ok := strings.Cut(head, "..."); ok {
		gi.Branch, gi.Upstream = b, up
	} else {
		gi.Branch = head
	}
	rest = strings.TrimSuffix(rest, "]")
	for _, part := range strings.Split(rest, ", ") {
		var n int
		if _, err := fmt.Sscanf(part, "ahead %d", &n); err == nil {
			gi.Ahead = n
		}
		if _, err := fmt.Sscanf(part, "behind %d", &n); err == nil {
			gi.Behind = n
		}
	}
}
