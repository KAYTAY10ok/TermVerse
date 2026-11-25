// Package theme loads, validates and resolves TermVerse color themes.
//
// Themes are JSON files. The five built-in themes are embedded in the binary
// and copied to ~/.config/termverse/themes on first launch; any *.json file in
// that directory is loaded, so users can create their own themes without
// touching the source code.
package theme

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Colors is the palette of a theme. All values are hex colors (#RGB or #RRGGBB).
type Colors struct {
	Primary       string `json:"primary"`
	Secondary     string `json:"secondary"`
	Accent        string `json:"accent"`
	Text          string `json:"text"`
	Muted         string `json:"muted"`
	Border        string `json:"border"`
	Highlight     string `json:"highlight"`
	HighlightText string `json:"highlight_text"`
	Success       string `json:"success"`
	Warning       string `json:"warning"`
	Error         string `json:"error"`
}

// Theme is a named palette.
type Theme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	Colors      Colors `json:"colors"`
	// Source is "builtin" or the path of the file the theme was loaded from.
	Source string `json:"-"`
}

// BuiltinNames lists the built-in themes in display order.
var BuiltinNames = []string{"cyberpunk", "matrix", "dracula", "nord", "minimal"}

// DefaultName is the theme used when nothing else is configured.
const DefaultName = "cyberpunk"

var hexRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ValidHex reports whether s is a #RGB or #RRGGBB color.
func ValidHex(s string) bool { return hexRe.MatchString(s) }

// NormalizeHex upper-cases a valid hex color and expands #RGB to #RRGGBB.
func NormalizeHex(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	if !ValidHex(s) {
		return "", false
	}
	s = strings.ToUpper(s)
	if len(s) == 4 {
		s = "#" + string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})
	}
	return s, true
}

// Parse decodes and validates a theme. Missing optional colors are filled
// from fallback (usually the default theme), so partial themes are accepted.
func Parse(data []byte, fallback Colors) (Theme, error) {
	var t Theme
	if err := json.Unmarshal(data, &t); err != nil {
		return Theme{}, fmt.Errorf("invalid JSON: %w", err)
	}
	t.Name = strings.ToLower(strings.TrimSpace(t.Name))
	if t.Name == "" {
		return Theme{}, fmt.Errorf("missing \"name\"")
	}
	if !validName(t.Name) {
		return Theme{}, fmt.Errorf("invalid name %q (use letters, digits, '-' or '_')", t.Name)
	}
	if err := t.Colors.fill(fallback); err != nil {
		return Theme{}, err
	}
	return t, nil
}

func validName(n string) bool {
	for _, r := range n {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return n != ""
}

func (c *Colors) fields() []*string {
	return []*string{&c.Primary, &c.Secondary, &c.Accent, &c.Text, &c.Muted, &c.Border,
		&c.Highlight, &c.HighlightText, &c.Success, &c.Warning, &c.Error}
}

var fieldNames = []string{"primary", "secondary", "accent", "text", "muted", "border",
	"highlight", "highlight_text", "success", "warning", "error"}

func (c *Colors) fill(fb Colors) error {
	own, def := c.fields(), fb.fields()
	for i, p := range own {
		if strings.TrimSpace(*p) == "" {
			*p = *def[i]
			continue
		}
		n, ok := NormalizeHex(*p)
		if !ok {
			return fmt.Errorf("color %q has invalid value %q (expected #RRGGBB)", fieldNames[i], *p)
		}
		*p = n
	}
	return nil
}

// Registry holds all known themes.
type Registry struct {
	themes map[string]Theme
}

// NewRegistry builds a registry from the embedded built-in theme files.
func NewRegistry(builtin fs.FS) (*Registry, error) {
	r := &Registry{themes: map[string]Theme{}}
	for _, name := range BuiltinNames {
		data, err := fs.ReadFile(builtin, name+".json")
		if err != nil {
			return nil, fmt.Errorf("built-in theme %s: %w", name, err)
		}
		t, err := Parse(data, Colors{})
		if err != nil {
			return nil, fmt.Errorf("built-in theme %s: %w", name, err)
		}
		t.Source = "builtin"
		r.themes[t.Name] = t
	}
	return r, nil
}

// LoadDir loads every *.json theme from dir. User files override built-in
// themes with the same name. Invalid files are skipped and reported.
func (r *Registry) LoadDir(dir string) []string {
	var warnings []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("cannot read themes directory: %v", err))
		}
		return warnings
	}
	fallback := r.themes[DefaultName].Colors
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("theme %s: %v", e.Name(), err))
			continue
		}
		t, err := Parse(data, fallback)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("theme %s skipped: %v", e.Name(), err))
			continue
		}
		t.Source = path
		r.themes[t.Name] = t
	}
	return warnings
}

// Get returns the theme with the given name.
func (r *Registry) Get(name string) (Theme, bool) {
	t, ok := r.themes[strings.ToLower(name)]
	return t, ok
}

// Names returns the theme names: built-ins first (fixed order), then custom
// themes alphabetically.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.themes))
	seen := map[string]bool{}
	for _, n := range BuiltinNames {
		if _, ok := r.themes[n]; ok {
			names = append(names, n)
			seen[n] = true
		}
	}
	var custom []string
	for n := range r.themes {
		if !seen[n] {
			custom = append(custom, n)
		}
	}
	sort.Strings(custom)
	return append(names, custom...)
}

// Len returns the number of themes.
func (r *Registry) Len() int { return len(r.themes) }

// Overrides are user color overrides applied on top of a theme.
type Overrides struct {
	Primary, Secondary, Accent string
}

// Apply returns a copy of t with valid, non-empty overrides applied.
func (t Theme) Apply(o Overrides) Theme {
	set := func(dst *string, v string) {
		if n, ok := NormalizeHex(v); ok && v != "" {
			*dst = n
		}
	}
	set(&t.Colors.Primary, o.Primary)
	set(&t.Colors.Secondary, o.Secondary)
	set(&t.Colors.Accent, o.Accent)
	return t
}

// Marshal encodes the theme as indented JSON.
func (t Theme) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
