package devtools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/KAYTAY10ok/TermVerse/internal/netinfo"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// ---- Git ---------------------------------------------------------------------

type gitMsg GitInfo

type gitTool struct {
	ctx    *ui.Context
	info   *GitInfo
	dir    string
	busy   bool
	scroll ui.Scroller
	prompt ui.Prompt
}

func newGitTool(ctx *ui.Context) *gitTool {
	wd, _ := os.Getwd()
	return &gitTool{ctx: ctx, dir: wd}
}
func (t *gitTool) id() string   { return "git" }
func (t *gitTool) name() string { return "Git Inspector" }
func (t *gitTool) desc() string {
	return "Branch, status, remotes and recent commits of a local repository"
}
func (t *gitTool) capturing() bool { return t.prompt.Active }
func (t *gitTool) keys() []ui.Key {
	return []ui.Key{{Keys: "r", Desc: "refresh"}, {Keys: "o", Desc: "open directory"}, {Keys: "↑↓", Desc: "scroll"}}
}
func (t *gitTool) load() tea.Cmd {
	t.busy = true
	dir := t.dir
	return func() tea.Msg { return gitMsg(ReadGit(dir)) }
}
func (t *gitTool) open() tea.Cmd { t.scroll.Vim = t.ctx.Vim(); return t.load() }
func (t *gitTool) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gitMsg:
		gi := GitInfo(m)
		t.info, t.busy = &gi, false
	case tea.KeyMsg:
		if t.prompt.Active {
			return t.prompt.Update(msg)
		}
		switch m.String() {
		case "r":
			return t.load()
		case "o":
			return t.prompt.Open(t.ctx.S, "Repository directory", "~/projects/app", t.dir, func(v string) tea.Cmd {
				v = strings.TrimSpace(v)
				if strings.HasPrefix(v, "~") {
					h, _ := os.UserHomeDir()
					v = h + v[1:]
				}
				t.dir = v
				return t.load()
			})
		default:
			t.scroll.HandleKey(m)
		}
	}
	return nil
}
func (t *gitTool) view(w, h int) string {
	st := t.ctx.S
	var b strings.Builder
	switch {
	case t.info == nil || t.busy && t.info == nil:
		b.WriteString(st.Muted.Render(ui.T("Reading repository…")))
	case t.info.Err != nil:
		b.WriteString(st.Muted.Render(t.info.Dir) + "\n\n" + st.Warning.Render(t.info.Err.Error()) + "\n\n" +
			st.Muted.Render(ui.T("Press o to choose another directory.")))
	default:
		gi := t.info
		clean := st.Success.Render("✔ " + ui.T("clean"))
		if len(gi.Changes) > 0 {
			clean = st.Warning.Render(ui.Tf("● %d change(s)", len(gi.Changes)))
		}
		track := ui.T("no upstream")
		if gi.Upstream != "" {
			track = fmt.Sprintf("%s  ↑%d ↓%d", gi.Upstream, gi.Ahead, gi.Behind)
		}
		b.WriteString(st.KV([][2]string{
			{ui.T("Repository"), gi.Root}, {ui.T("Branch"), st.Accent.Render(gi.Branch)}, {ui.T("Upstream"), track},
			{ui.T("Last tag"), orDash(gi.LastTag)}, {ui.T("Working tree"), clean},
			{ui.T("Details"), ui.Tf("%d staged · %d modified · %d untracked · %d conflicts · %d stashes", gi.Staged, gi.Modified, gi.Untracked, gi.Conflict, gi.Stashes)},
		}, 13) + "\n\n")
		if len(gi.Changes) > 0 {
			b.WriteString(st.Title.Render(ui.T("Changes")) + "\n")
			for i, c := range gi.Changes {
				if i == 30 {
					b.WriteString(st.Muted.Render(ui.Tf("… and %d more", len(gi.Changes)-30)) + "\n")
					break
				}
				b.WriteString(st.Warning.Render(c[:2]) + st.Text.Render(ui.Sanitize(c[2:])) + "\n")
			}
			b.WriteString("\n")
		}
		if len(gi.Remotes) > 0 {
			b.WriteString(st.Title.Render(ui.T("Remotes")) + "\n")
			for _, r := range gi.Remotes {
				b.WriteString(st.Text.Render(ui.Sanitize(r)) + "\n")
			}
			b.WriteString("\n")
		}
		b.WriteString(st.Title.Render(ui.T("Recent commits")) + "\n")
		for _, l := range gi.Log {
			hash, rest, _ := strings.Cut(l, "  ")
			b.WriteString(st.Accent.Render(hash) + "  " + st.Text.Render(ui.Sanitize(rest)) + "\n")
		}
	}
	bottom := ""
	if t.prompt.Active {
		bottom = t.prompt.View(st, w)
	}
	return withBottom(st, &t.scroll, b.String(), bottom, w, h)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// withBottom renders scrollable content above an optional bottom area.
func withBottom(st ui.Styles, sc *ui.Scroller, content, bottom string, w, h int) string {
	sc.SetContent(content)
	if bottom == "" {
		return sc.View(st, w, h)
	}
	bh := strings.Count(bottom, "\n") + 2
	return sc.View(st, w, h-bh) + "\n" + st.Sep(w) + "\n" + bottom
}

// ---- JSON --------------------------------------------------------------------

type jsonTool struct {
	ctx    *ui.Context
	ta     textarea.Model
	result string
	err    error
	scroll ui.Scroller
}

func newJSONTool(ctx *ui.Context) *jsonTool { return &jsonTool{ctx: ctx} }
func (t *jsonTool) id() string              { return "json" }
func (t *jsonTool) name() string            { return "JSON Formatter" }
func (t *jsonTool) desc() string {
	return "Validate, pretty-print and minify JSON with precise error location"
}
func (t *jsonTool) capturing() bool { return true }
func (t *jsonTool) keys() []ui.Key {
	return []ui.Key{{Keys: "ctrl+f", Desc: "format"}, {Keys: "ctrl+t", Desc: "minify"}, {Keys: "ctrl+l", Desc: "clear"}, {Keys: "pgup/pgdn", Desc: "scroll result"}}
}
func (t *jsonTool) open() tea.Cmd {
	if t.ta.Value() == "" && t.ta.Placeholder == "" {
		t.ta = textarea.New()
		t.ta.Placeholder = ui.T("Paste JSON here…")
		t.ta.CharLimit = 1 << 20
		t.ta.ShowLineNumbers = true
	}
	return t.ta.Focus()
}
func (t *jsonTool) validate() {
	t.result, t.err = FormatJSON(t.ta.Value(), "  ")
}
func (t *jsonTool) update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch km.String() {
	case "ctrl+f":
		if out, err := FormatJSON(t.ta.Value(), "  "); err == nil {
			t.ta.SetValue(out)
		}
		t.validate()
		return nil
	case "ctrl+t":
		if out, err := FormatJSON(t.ta.Value(), ""); err == nil {
			t.ta.SetValue(out)
		}
		t.validate()
		return nil
	case "ctrl+l":
		t.ta.Reset()
		t.result, t.err = "", nil
		return nil
	case "pgup", "pgdown":
		t.scroll.HandleKey(km)
		return nil
	}
	var cmd tea.Cmd
	t.ta, cmd = t.ta.Update(msg)
	t.validate()
	return cmd
}
func (t *jsonTool) view(w, h int) string {
	st := t.ctx.S
	half := w / 2
	t.ta.SetWidth(half - 1)
	t.ta.SetHeight(h - 2)
	var status, res string
	switch {
	case strings.TrimSpace(t.ta.Value()) == "":
		status = st.Muted.Render(ui.T("Waiting for input"))
	case t.err != nil:
		status = st.Error.Render("✗ " + t.err.Error())
	default:
		status = st.Success.Render("✔ "+ui.T("valid JSON")) + "  " + st.Muted.Render(JSONStats(t.ta.Value()))
		res = st.Text.Render(t.result)
	}
	t.scroll.SetContent(res)
	left := st.Accent.Render(ui.T("Input")) + "\n" + t.ta.View()
	right := st.Accent.Render(ui.T("Result")) + "\n" + ui.Fit(t.scroll.View(st, w-half-2, h-2), w-half-2, h-2)
	return joinCols(left, right, half, w-half-2, h-1) + "\n" + ui.Truncate(status, w)
}

func joinCols(a, b string, wa, wb, h int) string {
	al := strings.Split(ui.Fit(a, wa, h), "\n")
	bl := strings.Split(ui.Fit(b, wb, h), "\n")
	out := make([]string, h)
	for i := range out {
		out[i] = al[i] + "  " + bl[i]
	}
	return strings.Join(out, "\n")
}

// ---- UUID --------------------------------------------------------------------

type uuidTool struct {
	ctx   *ui.Context
	ids   []string
	v7    bool
	upper bool
}

func newUUIDTool(ctx *ui.Context) *uuidTool { return &uuidTool{ctx: ctx} }
func (t *uuidTool) id() string              { return "uuid" }
func (t *uuidTool) name() string            { return "UUID Generator" }
func (t *uuidTool) desc() string            { return "Random v4 and time-ordered v7 UUIDs" }
func (t *uuidTool) capturing() bool         { return false }
func (t *uuidTool) keys() []ui.Key {
	return []ui.Key{{Keys: "enter/g", Desc: "generate"}, {Keys: "n", Desc: "generate 10"}, {Keys: "v", Desc: "v4/v7"}, {Keys: "u", Desc: "upper case"}, {Keys: "c", Desc: "clear"}}
}
func (t *uuidTool) open() tea.Cmd {
	if len(t.ids) == 0 {
		t.gen(1)
	}
	return nil
}
func (t *uuidTool) gen(n int) {
	for i := 0; i < n; i++ {
		var u uuid.UUID
		var err error
		if t.v7 {
			u, err = uuid.NewV7()
		} else {
			u, err = uuid.NewRandom()
		}
		if err == nil {
			t.ids = append([]string{u.String()}, t.ids...)
		}
	}
	if len(t.ids) > 200 {
		t.ids = t.ids[:200]
	}
}
func (t *uuidTool) update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter", "g", " ":
			t.gen(1)
		case "n":
			t.gen(10)
		case "v":
			t.v7 = !t.v7
		case "u":
			t.upper = !t.upper
		case "c":
			t.ids = nil
		}
	}
	return nil
}
func (t *uuidTool) view(w, h int) string {
	st := t.ctx.S
	ver := "v4 (random)"
	if t.v7 {
		ver = "v7 (time-ordered)"
	}
	var b strings.Builder
	b.WriteString(st.Muted.Render(ui.T("Version: ")) + st.Accent.Render(ver) + st.Muted.Render("   "+ui.T("upper case: ")+ui.OnOff(t.upper)) + "\n\n")
	for i, id := range t.ids {
		if i >= h-4 {
			break
		}
		if t.upper {
			id = strings.ToUpper(id)
		}
		if i == 0 {
			b.WriteString(st.Selected.Render(" "+id+" ") + "  " + st.Muted.Render(strings.ReplaceAll(id, "-", "")) + "\n")
		} else {
			b.WriteString(" " + st.Text.Render(id) + "\n")
		}
	}
	b.WriteString("\n" + st.Muted.Render(ui.T("Select the text with your mouse to copy it.")))
	return b.String()
}

// ---- Base64 ------------------------------------------------------------------

type b64Tool struct {
	ctx  *ui.Context
	in   textinput.Model
	mode int // 0 encode, 1 encode URL-safe, 2 decode
}

var b64Modes = []string{"Encode", "Encode (URL-safe)", "Decode"}

func newBase64Tool(ctx *ui.Context) *b64Tool { return &b64Tool{ctx: ctx} }
func (t *b64Tool) id() string                { return "base64" }
func (t *b64Tool) name() string              { return "Base64" }
func (t *b64Tool) desc() string {
	return "Encode and decode Base64 (standard, URL-safe, padded or raw)"
}
func (t *b64Tool) capturing() bool { return true }
func (t *b64Tool) keys() []ui.Key {
	return []ui.Key{{Keys: "tab", Desc: "mode"}, {Keys: "ctrl+l", Desc: "clear"}}
}
func (t *b64Tool) open() tea.Cmd {
	if t.in.Prompt == "" {
		t.in = t.ctx.S.NewInput("Text to encode or Base64 to decode", 1<<16)
	}
	return t.in.Focus()
}
func (t *b64Tool) update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab":
			t.mode = (t.mode + 1) % 3
			return nil
		case "shift+tab":
			t.mode = (t.mode + 2) % 3
			return nil
		case "ctrl+l":
			t.in.SetValue("")
			return nil
		}
	}
	var cmd tea.Cmd
	t.in, cmd = t.in.Update(msg)
	return cmd
}
func (t *b64Tool) view(w, h int) string {
	st := t.ctx.S
	t.in.Width = w - 4
	out := st.Tabs(b64Modes, t.mode) + "\n\n" + t.in.View() + "\n\n" + st.Accent.Render(ui.T("Result")) + "\n"
	v := t.in.Value()
	if v == "" {
		return out + st.Muted.Render(ui.T("Type or paste some text."))
	}
	switch t.mode {
	case 0, 1:
		out += st.Text.Render(ui.Wrap(Base64Encode(v, t.mode == 1), w))
	case 2:
		b, kind, err := Base64Decode(v)
		if err != nil {
			return out + st.Error.Render(ui.T(err.Error()))
		}
		text, isText := Printable(b)
		label := ui.Tf("%s · %d bytes · text", kind, len(b))
		if !isText {
			label = ui.Tf("%s · %d bytes · binary (hex)", kind, len(b))
		}
		out += st.Muted.Render(label) + "\n" + st.Text.Render(ui.Wrap(ui.Sanitize(text), w))
	}
	return out
}

// ---- Hash --------------------------------------------------------------------

type hashTool struct {
	ctx    *ui.Context
	in     textinput.Model
	file   bool
	res    []HashResult
	size   int64
	err    error
	hashed string
}

type hashMsg struct {
	path string
	res  []HashResult
	n    int64
	err  error
}

func newHashTool(ctx *ui.Context) *hashTool { return &hashTool{ctx: ctx} }
func (t *hashTool) id() string              { return "hash" }
func (t *hashTool) name() string            { return "Hash Calculator" }
func (t *hashTool) desc() string            { return "MD5, SHA-1, SHA-256, SHA-512 and CRC32 of text or files" }
func (t *hashTool) capturing() bool         { return true }
func (t *hashTool) keys() []ui.Key {
	return []ui.Key{{Keys: "tab", Desc: "text/file"}, {Keys: "enter", Desc: "hash file"}, {Keys: "ctrl+l", Desc: "clear"}}
}
func (t *hashTool) open() tea.Cmd {
	if t.in.Prompt == "" {
		t.in = t.ctx.S.NewInput("Text to hash", 1<<16)
	}
	return t.in.Focus()
}
func (t *hashTool) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case hashMsg:
		t.res, t.size, t.err, t.hashed = m.res, m.n, m.err, m.path
		return nil
	case tea.KeyMsg:
		switch m.String() {
		case "tab":
			t.file = !t.file
			t.res, t.err = nil, nil
			if t.file {
				t.in.Placeholder = ui.T("Path of the file to hash")
			} else {
				t.in.Placeholder = ui.T("Text to hash")
			}
			return nil
		case "ctrl+l":
			t.in.SetValue("")
			t.res, t.err = nil, nil
			return nil
		case "enter":
			if t.file {
				p := strings.TrimSpace(t.in.Value())
				if strings.HasPrefix(p, "~") {
					h, _ := os.UserHomeDir()
					p = h + p[1:]
				}
				return func() tea.Msg { r, n, err := HashFile(p); return hashMsg{p, r, n, err} }
			}
		}
	}
	var cmd tea.Cmd
	t.in, cmd = t.in.Update(msg)
	if !t.file {
		t.res, t.err = HashString(t.in.Value()), nil
		t.size = int64(len(t.in.Value()))
	}
	return cmd
}
func (t *hashTool) view(w, h int) string {
	st := t.ctx.S
	t.in.Width = w - 4
	out := st.Tabs([]string{"Text", "File"}, map[bool]int{false: 0, true: 1}[t.file]) + "\n\n" + t.in.View() + "\n\n"
	if t.err != nil {
		return out + st.Error.Render(t.err.Error())
	}
	if t.res == nil {
		if t.file {
			return out + st.Muted.Render(ui.T("Enter a path and press Enter."))
		}
		t.res = HashString("")
	}
	out += st.Muted.Render(ui.Tf("%d bytes", t.size)) + "\n\n"
	for _, r := range t.res {
		out += st.Accent.Render(ui.PadRight(r.Name, 9)) + st.Text.Render(ui.Truncate(r.Hex, w-10)) + "\n"
	}
	return out
}

// ---- Timestamp ---------------------------------------------------------------

type timeTool struct {
	ctx *ui.Context
	in  textinput.Model
}

func newTimeTool(ctx *ui.Context) *timeTool { return &timeTool{ctx: ctx} }
func (t *timeTool) id() string              { return "time" }
func (t *timeTool) name() string            { return "Timestamp Converter" }
func (t *timeTool) desc() string {
	return "Convert Unix timestamps (s, ms, µs, ns) and dates both ways"
}
func (t *timeTool) capturing() bool { return true }
func (t *timeTool) keys() []ui.Key  { return []ui.Key{{Keys: "ctrl+l", Desc: "clear"}} }
func (t *timeTool) open() tea.Cmd {
	if t.in.Prompt == "" {
		t.in = t.ctx.S.NewInput("1700000000, 2024-05-01 14:30 or empty for now", 64)
	}
	return t.in.Focus()
}
func (t *timeTool) update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "ctrl+l" {
		t.in.SetValue("")
		return nil
	}
	var cmd tea.Cmd
	t.in, cmd = t.in.Update(msg)
	return cmd
}
func (t *timeTool) view(w, h int) string {
	st := t.ctx.S
	t.in.Width = w - 4
	out := t.in.View() + "\n\n"
	tm, kind, err := ParseTime(t.in.Value(), time.Local)
	if err != nil {
		return out + st.Error.Render(ui.T(err.Error())) + "\n\n" + st.Muted.Render(ui.T("Accepted: Unix time, RFC3339, YYYY-MM-DD [HH:MM[:SS]], DD/MM/YYYY, RFC1123"))
	}
	now := time.Now()
	return out + st.Muted.Render(ui.T("Detected: ")+ui.T(kind)) + "\n\n" + st.KV([][2]string{
		{ui.T("Local"), tm.Local().Format("Mon 2006-01-02 15:04:05 MST")},
		{"UTC", tm.UTC().Format("Mon 2006-01-02 15:04:05")},
		{"RFC 3339", tm.Format(time.RFC3339)},
		{"ISO week", fmt.Sprintf("%d-W%02d", isoYear(tm), isoWeek(tm))},
		{ui.T("Unix (s)"), fmt.Sprint(tm.Unix())},
		{ui.T("Unix (ms)"), fmt.Sprint(tm.UnixMilli())},
		{ui.T("Unix (ns)"), fmt.Sprint(tm.UnixNano())},
		{ui.T("Relative"), Relative(tm, now)},
		{ui.T("Day of year"), fmt.Sprint(tm.YearDay())},
	}, 12)
}

func isoWeek(t time.Time) int { _, w := t.ISOWeek(); return w }
func isoYear(t time.Time) int { y, _ := t.ISOWeek(); return y }

// ---- Password ----------------------------------------------------------------

type pwTool struct {
	ctx  *ui.Context
	opts PasswordOptions
	opt  ui.Options
	pw   string
	bits float64
	err  error
}

func newPasswordTool(ctx *ui.Context) *pwTool {
	t := &pwTool{ctx: ctx, opts: PasswordOptions{Length: 20, Upper: true, Lower: true, Digits: true, Symbols: true}}
	toggle := func(label string, p *bool) ui.Option {
		return ui.Option{Label: label, Value: func() string { return ui.OnOff(*p) }, Cycle: func(int) tea.Cmd { *p = !*p; t.gen(); return nil }}
	}
	t.opt.Items = []ui.Option{
		{Label: "Length", Value: func() string { return fmt.Sprint(t.opts.Length) }, Cycle: func(d int) tea.Cmd {
			t.opts.Length = min(128, max(4, t.opts.Length+d))
			t.gen()
			return nil
		}},
		toggle("Upper case (A-Z)", &t.opts.Upper),
		toggle("Lower case (a-z)", &t.opts.Lower),
		toggle("Digits (0-9)", &t.opts.Digits),
		toggle("Symbols (!@#…)", &t.opts.Symbols),
		toggle("Exclude ambiguous (Il1O0)", &t.opts.NoAmbiguous),
		{Label: "Generate", Enter: func() tea.Cmd { t.gen(); return nil }},
	}
	return t
}
func (t *pwTool) id() string      { return "password" }
func (t *pwTool) name() string    { return "Password Generator" }
func (t *pwTool) desc() string    { return "Cryptographically secure passwords with entropy estimate" }
func (t *pwTool) capturing() bool { return false }
func (t *pwTool) keys() []ui.Key {
	return []ui.Key{{Keys: "←→", Desc: "change"}, {Keys: "g", Desc: "generate"}}
}
func (t *pwTool) gen()          { t.pw, t.bits, t.err = GeneratePassword(t.opts) }
func (t *pwTool) open() tea.Cmd { t.opt.Vim = t.ctx.Vim(); t.gen(); return nil }
func (t *pwTool) update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "g" {
			t.gen()
			return nil
		}
		_, cmd := t.opt.HandleKey(km)
		return cmd
	}
	return nil
}
func (t *pwTool) view(w, h int) string {
	st := t.ctx.S
	out := ui.Fit(t.opt.View(st, w, len(t.opt.Items)), w, len(t.opt.Items)) + "\n\n"
	if t.err != nil {
		return out + st.Error.Render(ui.T(t.err.Error()))
	}
	strength := Strength(t.bits)
	style := map[string]func(...string) string{
		"weak": st.Error.Render, "fair": st.Warning.Render, "strong": st.Success.Render, "very strong": st.Success.Bold(true).Render,
	}[strength]
	return out + st.Selected.Render(" "+t.pw+" ") + "\n\n" +
		st.Muted.Render(ui.Tf("Entropy: %.0f bits · ", t.bits)) + style(ui.T(strength))
}

// ---- Regex -------------------------------------------------------------------

type regexTool struct {
	ctx     *ui.Context
	pattern textinput.Model
	text    textarea.Model
	focus   int
	scroll  ui.Scroller
	inited  bool
}

func newRegexTool(ctx *ui.Context) *regexTool { return &regexTool{ctx: ctx} }
func (t *regexTool) id() string               { return "regex" }
func (t *regexTool) name() string             { return "Regex Tester" }
func (t *regexTool) desc() string {
	return "Test Go (RE2) regular expressions with live highlighting and groups"
}
func (t *regexTool) capturing() bool { return true }
func (t *regexTool) keys() []ui.Key {
	return []ui.Key{{Keys: "tab", Desc: "pattern/text"}, {Keys: "pgup/pgdn", Desc: "scroll matches"}}
}
func (t *regexTool) open() tea.Cmd {
	if !t.inited {
		t.inited = true
		t.pattern = t.ctx.S.NewInput(`(\w+)@(\w+)\.com`, 1024)
		t.text = textarea.New()
		t.text.Placeholder = ui.T("Test text…")
		t.text.ShowLineNumbers = false
		t.text.CharLimit = 1 << 16
	}
	t.focus = 0
	t.text.Blur()
	return t.pattern.Focus()
}
func (t *regexTool) update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "shift+tab":
			t.focus = 1 - t.focus
			if t.focus == 0 {
				t.text.Blur()
				return t.pattern.Focus()
			}
			t.pattern.Blur()
			return t.text.Focus()
		case "pgup", "pgdown":
			t.scroll.HandleKey(km)
			return nil
		}
	} else {
		return nil
	}
	var cmd tea.Cmd
	if t.focus == 0 {
		t.pattern, cmd = t.pattern.Update(msg)
	} else {
		t.text, cmd = t.text.Update(msg)
	}
	return cmd
}
func (t *regexTool) view(w, h int) string {
	st := t.ctx.S
	t.pattern.Width = w - 4
	textH := max(3, h/3)
	t.text.SetWidth(w)
	t.text.SetHeight(textH)
	out := st.Accent.Render(ui.T("Pattern")) + "\n" + t.pattern.View() + "\n" + st.Accent.Render(ui.T("Text")) + "\n" + t.text.View() + "\n"
	used := 4 + textH
	pat := t.pattern.Value()
	var res strings.Builder
	if pat == "" {
		res.WriteString(st.Muted.Render(ui.T("Enter a pattern.")))
	} else if re, err := regexp.Compile(pat); err != nil {
		res.WriteString(st.Error.Render(err.Error()))
	} else {
		txt := t.text.Value()
		ms := re.FindAllStringSubmatchIndex(txt, 500)
		res.WriteString(st.Success.Render(ui.Tf("%d match(es)", len(ms))) + "\n")
		// highlighted text
		var hl strings.Builder
		last := 0
		for _, m := range ms {
			hl.WriteString(st.Text.Render(txt[last:m[0]]))
			hl.WriteString(st.Selected.Render(txt[m[0]:m[1]]))
			last = m[1]
		}
		hl.WriteString(st.Text.Render(txt[last:]))
		res.WriteString(ui.Sanitize(hl.String()) + "\n\n")
		names := re.SubexpNames()
		for i, m := range ms {
			if i >= 50 {
				break
			}
			res.WriteString(st.Accent.Render(fmt.Sprintf("#%d", i+1)) + " " + st.Text.Render(fmt.Sprintf("%q", txt[m[0]:m[1]])) + st.Muted.Render(fmt.Sprintf(" [%d:%d]", m[0], m[1])) + "\n")
			for g := 1; g*2 < len(m); g++ {
				if m[2*g] < 0 {
					continue
				}
				label := fmt.Sprintf("  $%d", g)
				if names[g] != "" {
					label += " (" + names[g] + ")"
				}
				res.WriteString(st.Muted.Render(label+" ") + st.Text.Render(fmt.Sprintf("%q", txt[m[2*g]:m[2*g+1]])) + "\n")
			}
		}
	}
	t.scroll.SetContent(res.String())
	return out + st.Sep(w) + "\n" + t.scroll.View(st, w, max(1, h-used-1))
}

// ---- HTTP --------------------------------------------------------------------

type httpResult struct {
	Method, URL, Status, Proto string
	Code                       int
	Headers                    http.Header
	Body                       string
	Size                       int64
	Duration                   time.Duration
	Err                        error
}

type httpMsg httpResult

type httpTool struct {
	ctx    *ui.Context
	in     textinput.Model
	method int
	busy   bool
	res    *httpResult
	scroll ui.Scroller
}

var httpMethods = []string{"GET", "HEAD"}

func newHTTPTool(ctx *ui.Context) *httpTool { return &httpTool{ctx: ctx} }
func (t *httpTool) id() string              { return "http" }
func (t *httpTool) name() string            { return "HTTP Client" }
func (t *httpTool) desc() string {
	return "Send GET/HEAD requests and inspect status, headers and body"
}
func (t *httpTool) capturing() bool { return true }
func (t *httpTool) keys() []ui.Key {
	return []ui.Key{{Keys: "enter", Desc: "send"}, {Keys: "tab", Desc: "method"}, {Keys: "pgup/pgdn ↑↓", Desc: "scroll"}}
}
func (t *httpTool) open() tea.Cmd {
	if t.in.Prompt == "" {
		t.in = t.ctx.S.NewInput("https://api.github.com/zen", 2048)
		t.scroll.Vim = false
	}
	return t.in.Focus()
}

// Request performs an HTTP request with a 15s timeout (body capped at 256 KiB).
func Request(method, rawURL string) httpResult {
	r := httpResult{Method: method, URL: rawURL}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
		r.URL = rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		r.Err = fmt.Errorf("invalid URL (only http and https are supported)")
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		r.Err = err
		return r
	}
	req.Header.Set("User-Agent", "TermVerse-HTTP-Client")
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.Err, r.Duration = err, time.Since(start)
		return r
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	r.Duration = time.Since(start)
	r.Status, r.Code, r.Proto, r.Headers, r.Size = resp.Status, resp.StatusCode, resp.Proto, resp.Header, int64(len(body))
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		if pretty, err := FormatJSON(string(body), "  "); err == nil {
			body = []byte(pretty)
		}
	}
	text, ok := Printable(body)
	if !ok {
		text = fmt.Sprintf("(binary body, %d bytes)", len(body))
	}
	r.Body = text
	return r
}

func (t *httpTool) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case httpMsg:
		r := httpResult(m)
		t.res, t.busy = &r, false
		t.scroll.Top()
		return nil
	case tea.KeyMsg:
		switch m.String() {
		case "tab":
			t.method = (t.method + 1) % len(httpMethods)
			return nil
		case "enter":
			if t.busy || strings.TrimSpace(t.in.Value()) == "" {
				return nil
			}
			t.busy = true
			method, u := httpMethods[t.method], strings.TrimSpace(t.in.Value())
			return func() tea.Msg { return httpMsg(Request(method, u)) }
		case "pgup", "pgdown", "up", "down":
			t.scroll.HandleKey(m)
			return nil
		}
	}
	var cmd tea.Cmd
	t.in, cmd = t.in.Update(msg)
	return cmd
}
func (t *httpTool) view(w, h int) string {
	st := t.ctx.S
	t.in.Width = w - 12
	out := st.Selected.Render(" "+httpMethods[t.method]+" ") + " " + t.in.View() + "\n" + st.Sep(w) + "\n"
	var b strings.Builder
	switch {
	case t.busy:
		b.WriteString(st.Secondary.Render(ui.T("Sending request… (timeout 15s)")))
	case t.res == nil:
		b.WriteString(st.Muted.Render(ui.T("Type a URL and press Enter. Requests are only sent when you press Enter.")))
	case t.res.Err != nil:
		b.WriteString(st.Error.Render(t.res.Err.Error()))
	default:
		r := t.res
		cs := st.Success
		if r.Code >= 400 {
			cs = st.Error
		} else if r.Code >= 300 {
			cs = st.Warning
		}
		b.WriteString(cs.Bold(true).Render(r.Status) + st.Muted.Render(fmt.Sprintf("  %s · %s · %d B", r.Proto, r.Duration.Round(time.Millisecond), r.Size)) + "\n\n")
		keys := make([]string, 0, len(r.Headers))
		for k := range r.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(st.Accent.Render(k+": ") + st.Text.Render(ui.Sanitize(strings.Join(r.Headers[k], ", "))) + "\n")
		}
		if r.Body != "" {
			b.WriteString("\n" + st.Text.Render(ui.Sanitize(r.Body)))
		}
	}
	t.scroll.SetContent(b.String())
	return out + t.scroll.View(st, w, h-2)
}

// ---- Ports -------------------------------------------------------------------

type portsMsg struct {
	conns []netinfo.Conn
	err   error
}

type portsTool struct {
	ctx    *ui.Context
	res    *portsMsg
	scroll ui.Scroller
}

func newPortsTool(ctx *ui.Context) *portsTool { return &portsTool{ctx: ctx} }
func (t *portsTool) id() string               { return "ports" }
func (t *portsTool) name() string             { return "Listening Ports" }
func (t *portsTool) desc() string             { return "TCP sockets in LISTEN state and bound UDP sockets" }
func (t *portsTool) capturing() bool          { return false }
func (t *portsTool) keys() []ui.Key {
	return []ui.Key{{Keys: "r", Desc: "refresh"}, {Keys: "↑↓", Desc: "scroll"}}
}
func (t *portsTool) load() tea.Cmd {
	return func() tea.Msg { c, err := netinfo.Listening(); return portsMsg{c, err} }
}
func (t *portsTool) open() tea.Cmd { t.scroll.Vim = t.ctx.Vim(); return t.load() }
func (t *portsTool) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case portsMsg:
		t.res = &m
	case tea.KeyMsg:
		if m.String() == "r" {
			return t.load()
		}
		t.scroll.HandleKey(m)
	}
	return nil
}
func (t *portsTool) view(w, h int) string {
	st := t.ctx.S
	var b strings.Builder
	switch {
	case t.res == nil:
		b.WriteString(st.Muted.Render(ui.T("Loading…")))
	case t.res.err != nil:
		b.WriteString(st.Error.Render(t.res.err.Error()))
	default:
		b.WriteString(st.Title.Render(ui.Tf("%d listening sockets", len(t.res.conns))) + "\n\n")
		b.WriteString(st.Bold.Render(fmt.Sprintf("%-6s %-32s %s", "PROTO", ui.T("ADDRESS"), ui.T("PROCESS"))) + "\n")
		for _, c := range t.res.conns {
			proc := st.Muted.Render(ui.T("(unknown — other user)"))
			if c.PID > 0 {
				proc = st.Text.Render(fmt.Sprintf("%s (%d)", c.Process, c.PID))
			}
			b.WriteString(st.Accent.Render(fmt.Sprintf("%-6s ", c.Proto)) + st.Text.Render(fmt.Sprintf("%-32s ", c.Local)) + proc + "\n")
		}
	}
	t.scroll.SetContent(b.String())
	return t.scroll.View(st, w, h)
}
