// Package filemanager implements a read-only file browser with preview.
// It never modifies, moves or deletes files.
package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// Screen is the file manager.
type Screen struct {
	ctx     *ui.Context
	dir     string
	entries []Entry
	list    ui.List
	err     error
	hidden  bool
	sortBy  string
	filter  string
	prompt  ui.Prompt
	details bool
	preview ui.Scroller
	focusPV bool
	lastSel string
	cursors map[string]int
}

// New creates the screen, starting in the current working directory.
func New(ctx *ui.Context) ui.Screen {
	dir, err := os.Getwd()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	s := &Screen{ctx: ctx, dir: dir, sortBy: "name", cursors: map[string]int{}}
	return s
}

func (s *Screen) ID() string      { return "files" }
func (s *Screen) Title() string   { return ui.T("File Manager") }
func (s *Screen) Capturing() bool { return s.prompt.Active }

func (s *Screen) Init() tea.Cmd {
	s.list.Vim, s.preview.Vim = s.ctx.Vim(), s.ctx.Vim()
	s.load()
	return nil
}

func (s *Screen) Keys() []ui.Key {
	if s.prompt.Active {
		return []ui.Key{{Keys: "enter", Desc: "go"}, {Keys: "esc", Desc: "cancel"}}
	}
	return []ui.Key{{Keys: "enter/→", Desc: "open"}, {Keys: "⌫/←", Desc: "parent"}, {Keys: ".", Desc: "hidden"},
		{Keys: "s", Desc: "sort"}, {Keys: "/", Desc: "filter"}, {Keys: "i", Desc: "info"}, {Keys: "tab", Desc: "preview focus"},
		{Keys: "g", Desc: "go to"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) load() {
	s.entries, s.err = ReadDir(s.dir, s.hidden, s.sortBy)
	s.rebuild()
}

func (s *Screen) rebuild() {
	var items []ui.Item
	if parent := filepath.Dir(s.dir); parent != s.dir {
		items = append(items, ui.Item{Title: "../", Right: ui.T("parent"), Value: Entry{Name: "..", Path: parent, IsDir: true}})
	}
	q := strings.ToLower(s.filter)
	for _, e := range s.entries {
		if q != "" && !strings.Contains(strings.ToLower(e.Name), q) {
			continue
		}
		name := ui.Sanitize(e.Name)
		right := ""
		switch {
		case e.Err != nil:
			right = "?"
		case e.IsDir:
			name += "/"
			right = e.Mode.Perm().String()
		default:
			right = fmt.Sprintf("%9s  %s", HumanSize(e.Size), e.Mode.Perm().String())
		}
		if e.IsLink {
			name += " → " + ui.Sanitize(e.Target)
		}
		items = append(items, ui.Item{Title: icon(e) + " " + name, Right: right, Value: e})
	}
	s.list.SetItems(items)
	if c, ok := s.cursors[s.dir]; ok {
		s.list.Cursor = c
		s.list.Clamp()
	}
}

func icon(e Entry) string {
	switch {
	case e.IsLink:
		return "↪"
	case e.IsDir:
		return "▸"
	case e.Mode&0o111 != 0:
		return "⚙"
	default:
		return "·"
	}
}

func (s *Screen) cd(dir string) {
	s.cursors[s.dir] = s.list.Cursor
	prev := s.dir
	s.dir = filepath.Clean(dir)
	s.filter = ""
	s.list.Cursor = 0
	s.load()
	if s.err != nil {
		bad := s.err
		s.dir = prev
		s.load()
		s.err = bad
		return
	}
	// When going up, select the directory we came from.
	if filepath.Dir(prev) == s.dir {
		for i, it := range s.list.Items {
			if it.Value.(Entry).Path == prev {
				s.list.Cursor = i
			}
		}
	}
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if s.focusPV {
			s.preview.HandleMouse(msg)
		} else {
			s.list.HandleMouse(msg)
		}
	case tea.KeyMsg:
		if s.prompt.Active {
			return s, s.prompt.Update(msg)
		}
		k := msg.String()
		vim := s.ctx.Vim()
		switch {
		case k == "esc" || k == "q":
			if s.filter != "" {
				s.filter = ""
				s.rebuild()
				return s, nil
			}
			if s.focusPV {
				s.focusPV = false
				return s, nil
			}
			return s, ui.BackCmd
		case k == "tab":
			s.focusPV = !s.focusPV
		case s.focusPV && s.preview.HandleKey(msg):
		case k == "enter" || k == "right" || vim && k == "l":
			if it, ok := s.list.Selected(); ok {
				e := it.Value.(Entry)
				if e.IsDir {
					s.cd(e.Path)
				} else {
					s.details = !s.details
				}
			}
		case k == "backspace" || k == "left" || vim && k == "h":
			s.cd(filepath.Dir(s.dir))
		case k == ".":
			s.hidden = !s.hidden
			s.load()
		case k == "s":
			s.sortBy = ui.CycleIndex([]string{"name", "size", "time"}, s.sortBy, 1)
			s.load()
		case k == "i":
			s.details = !s.details
		case k == "~":
			if h, err := os.UserHomeDir(); err == nil {
				s.cd(h)
			}
		case k == "/":
			return s, s.prompt.Open(s.ctx.S, "Filter entries by name", "", s.filter, func(v string) tea.Cmd {
				s.filter = strings.TrimSpace(v)
				s.list.Cursor = 0
				s.rebuild()
				return nil
			})
		case k == "g":
			return s, s.prompt.Open(s.ctx.S, "Go to directory", "/etc", s.dir, func(v string) tea.Cmd {
				v = strings.TrimSpace(v)
				if strings.HasPrefix(v, "~") {
					h, _ := os.UserHomeDir()
					v = filepath.Join(h, v[1:])
				}
				if !filepath.IsAbs(v) {
					v = filepath.Join(s.dir, v)
				}
				s.cd(v)
				return nil
			})
		default:
			s.list.HandleKey(msg)
		}
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	wide := w >= 100
	leftW := w
	if wide {
		leftW = w * 55 / 100
	}
	iw, ih := st.PanelInner(leftW, h)
	head := st.Accent.Render(ui.Truncate(ui.Tildify(s.dir), iw)) + "\n"
	info := ui.Tf("%d entries · sort: %s · hidden: %s · read-only", len(s.entries), ui.T(s.sortBy), ui.OnOff(s.hidden))
	if s.filter != "" {
		info += " · " + ui.T("filter: ") + s.filter
	}
	head += st.Muted.Render(ui.Truncate(info, iw)) + "\n"
	bottom := ""
	if s.prompt.Active {
		bottom = s.prompt.View(st, iw)
	} else if s.err != nil {
		bottom = st.Error.Render(ui.Truncate(s.err.Error(), iw))
	}
	bh := 0
	if bottom != "" {
		bh = lipgloss.Height(bottom) + 1
	}
	body := head + ui.Fit(s.list.View(st, iw, ih-2-bh), iw, ih-2-bh)
	if bottom != "" {
		body += "\n" + st.Sep(iw) + "\n" + bottom
	}
	left := st.Panel(s.Title(), body, leftW, h, !s.focusPV)
	if !wide {
		if s.details {
			return st.Panel(ui.T("Details"), s.detailView(iw), w, h, true)
		}
		return left
	}
	rw := w - leftW - 1
	riw, rih := st.PanelInner(rw, h)
	right := s.previewView(riw, rih)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", st.Panel(ui.T("Preview"), right, rw, h, s.focusPV))
}

func (s *Screen) detailView(width int) string {
	st := s.ctx.S
	it, ok := s.list.Selected()
	if !ok {
		return ""
	}
	e := it.Value.(Entry)
	info, err := os.Lstat(e.Path)
	if err != nil {
		return st.Error.Render(err.Error())
	}
	kind := ui.T("file")
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		kind = ui.T("symbolic link")
	case info.IsDir():
		kind = ui.T("directory")
	case info.Mode()&os.ModeNamedPipe != 0:
		kind = "FIFO"
	case info.Mode()&os.ModeSocket != 0:
		kind = "socket"
	case info.Mode()&os.ModeDevice != 0:
		kind = ui.T("device")
	}
	rows := [][2]string{
		{ui.T("Name"), ui.Sanitize(e.Name)},
		{ui.T("Path"), ui.Sanitize(e.Path)},
		{ui.T("Type"), kind},
		{ui.T("Size"), fmt.Sprintf("%s (%d B)", HumanSize(info.Size()), info.Size())},
		{ui.T("Mode"), fmt.Sprintf("%s (%04o)", info.Mode().String(), info.Mode().Perm())},
		{ui.T("Owner"), Owner(info)},
		{ui.T("Modified"), info.ModTime().Format("2006-01-02 15:04:05")},
	}
	if e.IsLink {
		rows = append(rows, [2]string{ui.T("Target"), ui.Sanitize(e.Target)})
	}
	if info.IsDir() || e.IsDir {
		if d, f, err := DirSummary(e.Path); err == nil {
			rows = append(rows, [2]string{ui.T("Contents"), ui.Tf("%d folders, %d files", d, f)})
		}
	}
	return st.KV(rows, 10)
}

func (s *Screen) previewView(width, height int) string {
	st := s.ctx.S
	it, ok := s.list.Selected()
	if !ok {
		return ""
	}
	e := it.Value.(Entry)
	meta := s.detailView(width)
	metaH := lipgloss.Height(meta) + 1
	var content string
	if e.Path != s.lastSel {
		s.lastSel = e.Path
		s.preview.Top()
	}
	switch {
	case e.IsDir:
		es, err := ReadDir(e.Path, s.hidden, "name")
		if err != nil {
			content = st.Error.Render(err.Error())
			break
		}
		var lines []string
		for _, c := range es {
			n := ui.Sanitize(c.Name)
			if c.IsDir {
				n = st.Accent.Render(n + "/")
			}
			lines = append(lines, icon(c)+" "+n)
		}
		if len(lines) == 0 {
			lines = []string{st.Muted.Render(ui.T("(empty)"))}
		}
		content = strings.Join(lines, "\n")
	case !e.Mode.IsRegular() && !e.IsLink:
		content = st.Muted.Render(ui.T("No preview for special files."))
	default:
		text, binary, err := Preview(e.Path, 64*1024)
		switch {
		case err != nil:
			content = st.Error.Render(err.Error())
		case binary:
			dump, _ := HexDump(e.Path, 256)
			content = st.Muted.Render(ui.T("Binary file — hex dump of the first 256 bytes:")) + "\n" + st.Text.Render(dump)
		default:
			text = ui.Sanitize(strings.ReplaceAll(text, "\t", "    "))
			lines := strings.Split(text, "\n")
			nw := len(fmt.Sprint(len(lines)))
			for i, l := range lines {
				lines[i] = st.Muted.Render(fmt.Sprintf("%*d ", nw, i+1)) + st.Text.Render(l)
			}
			content = strings.Join(lines, "\n")
		}
	}
	s.preview.SetContent(content)
	return meta + "\n" + st.Sep(width) + "\n" + s.preview.View(st, width, max(1, height-metaH-1))
}
