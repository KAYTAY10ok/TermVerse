package devtools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatJSON(t *testing.T) {
	out, err := FormatJSON(`{"a":[1,2],"b":{"c":true}}`, "  ")
	if err != nil || !strings.Contains(out, "\n  \"a\": [") {
		t.Fatalf("%q %v", out, err)
	}
	min, _ := FormatJSON("{ \"a\" : 1 }", "")
	if min != `{"a":1}` {
		t.Errorf("minify %q", min)
	}
	_, err = FormatJSON("{\n  \"a\": 1,\n  \"b\": }", "  ")
	je, ok := err.(*JSONError)
	if !ok || je.Line != 3 {
		t.Errorf("expected line 3 error, got %v", err)
	}
	if _, err := FormatJSON(`{"a":1} {"b":2}`, "  "); err == nil {
		t.Error("trailing data accepted")
	}
	if s := JSONStats(`{"a":[1,{"b":2}]}`); !strings.Contains(s, "2 objects") {
		t.Error(s)
	}
}

func TestBase64(t *testing.T) {
	enc := Base64Encode("héllo?>", false)
	b, kind, err := Base64Decode(enc)
	if err != nil || string(b) != "héllo?>" || kind != "standard" {
		t.Fatal(string(b), kind, err)
	}
	u := Base64Encode("??>>", true)
	if _, kind, _ := Base64Decode(u); kind != "URL-safe" {
		t.Error(kind)
	}
	if _, _, err := Base64Decode("@@@"); err == nil {
		t.Error("invalid accepted")
	}
	if _, kind, _ := Base64Decode("aGk"); kind != "standard (no padding)" {
		t.Error(kind)
	}
}

func TestHash(t *testing.T) {
	h := HashString("abc")
	want := map[string]string{
		"MD5":     "900150983cd24fb0d6963f7d28e17f72",
		"SHA-256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"CRC32":   "352441c2",
	}
	for _, r := range h {
		if w, ok := want[r.Name]; ok && w != r.Hex {
			t.Errorf("%s = %s", r.Name, r.Hex)
		}
	}
}

func TestParseTime(t *testing.T) {
	tm, kind, err := ParseTime("1700000000", time.UTC)
	if err != nil || tm.Unix() != 1700000000 || kind != "unix seconds" {
		t.Fatal(tm, kind, err)
	}
	if tm, kind, _ := ParseTime("1700000000123", time.UTC); kind != "unix milliseconds" || tm.UnixMilli() != 1700000000123 {
		t.Error(kind)
	}
	if tm, _, err := ParseTime("2024-02-29 12:00", time.UTC); err != nil || tm.Day() != 29 {
		t.Error(err)
	}
	if _, _, err := ParseTime("yesterday-ish", time.UTC); err == nil {
		t.Error("garbage accepted")
	}
	now := time.Now()
	if r := Relative(now.Add(-3*time.Hour), now); r != "3h ago" {
		t.Error(r)
	}
}

func TestPassword(t *testing.T) {
	p, bits, err := GeneratePassword(PasswordOptions{Length: 24, Upper: true, Lower: true, Digits: true, Symbols: true, NoAmbiguous: true})
	if err != nil || len(p) != 24 || bits < 100 {
		t.Fatal(p, bits, err)
	}
	if strings.ContainsAny(p, ambiguous) {
		t.Error("ambiguous char present")
	}
	classes := []string{lowerSet, upperSet, digitSet, symbolSet}
	for _, c := range classes {
		if !strings.ContainsAny(p, c) {
			t.Errorf("missing class %q in %s", c, p)
		}
	}
	if _, _, err := GeneratePassword(PasswordOptions{Length: 10}); err == nil {
		t.Error("no class accepted")
	}
}

func TestReadGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git missing")
	}
	d := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = d
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("a"), 0o644)
	run("add", "a.txt")
	run("commit", "-q", "-m", "first")
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(d, "new.txt"), []byte("n"), 0o644)
	gi := ReadGit(d)
	if gi.Err != nil {
		t.Fatal(gi.Err)
	}
	if gi.Branch != "main" || gi.Modified != 1 || gi.Untracked != 1 || len(gi.Log) != 1 {
		t.Fatalf("%+v", gi)
	}
	if gi := ReadGit(t.TempDir()); gi.Err == nil {
		t.Error("expected not-a-repo error")
	}
}

func TestParseBranchLine(t *testing.T) {
	var gi GitInfo
	parseBranchLine(&gi, "main...origin/main [ahead 2, behind 1]")
	if gi.Branch != "main" || gi.Upstream != "origin/main" || gi.Ahead != 2 || gi.Behind != 1 {
		t.Fatalf("%+v", gi)
	}
}
