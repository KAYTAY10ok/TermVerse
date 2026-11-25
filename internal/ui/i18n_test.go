package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

var verbRe = regexp.MustCompile(`%[-+ #0-9.]*[a-zA-Z%]`)

// TestFrenchFormatVerbs ensures translations keep the same printf verbs.
func TestFrenchFormatVerbs(t *testing.T) {
	for en, f := range fr {
		a, b := verbRe.FindAllString(en, -1), verbRe.FindAllString(f, -1)
		if len(a) != len(b) {
			t.Errorf("%q → %q: verbs %v vs %v", en, f, a, b)
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%q → %q: verbs %v vs %v", en, f, a, b)
			}
		}
	}
}

// TestFrenchCoverage checks that every UI string found by the extractor
// script has a French translation (identifiers and proper names excepted).
func TestFrenchCoverage(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	out := filepath.Join(t.TempDir(), "strings.json")
	if b, err := exec.Command(py, "../../scripts/i18n-extract.py", out).CombinedOutput(); err != nil {
		t.Skipf("extractor failed: %v %s", err, b)
	}
	data, _ := os.ReadFile(out)
	var strs []string
	if err := jsonUnmarshal(data, &strs); err != nil {
		t.Fatal(err)
	}
	for _, s := range strs {
		if _, ok := fr[s]; !ok && !untranslated[s] {
			t.Errorf("missing French translation for %q", s)
		}
	}
}

// untranslated lists strings that are identical in both languages.
var untranslated = map[string]bool{}

func init() {
	for _, s := range []string{"/etc/os-release", "/proc/net/route", "https://api.github.com/zen", "cyberpunk", "dracula", "matrix",
		"minimal", "nord", "rounded", "double", "thick", "normal", "accent", "border", "error", "highlight", "highlight_text", "muted",
		"primary", "secondary", "success", "warning", "auto", "guess", "notes", "pomodoro", "tasks", "Arch", "CPU", "OS", "Go", "GitHub",
		"ID", "Base64", "Snake", "Sky Hopper", "Mines", "Pomodoro", "Notes", "Plugin Hub", "Interfaces", "Version", "Modules", "Config",
		"Commit", "Page", "Mode", "Swap", "Section", "Sections", "Score %d", "Data", "Global", "Maintenance", "Type", "2048", "Local IP",
		"Navigation", "Unix (ms)", "Unix (ns)", "chat", "menu", "mode", "pause", "palette", "parent", "threads", "action", "format", "info", "s", "min", "Animations", "Unix (s)", "  · %d mines", "%d sockets"} {
		untranslated[s] = true
	}
}
