// Package productivity implements tasks, quick notes and a Pomodoro timer.
package productivity

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

var tabs = []string{"Tasks", "Notes", "Pomodoro"}

type phase int

const (
	idle phase = iota
	focus
	pause
)

type pomoTick struct{ gen int }

// Screen is the productivity screen.
type Screen struct {
	ctx     *ui.Context
	store   *Store
	tab     int
	tasks   ui.List
	notes   ui.List
	prompt  ui.Prompt
	confirm ui.Confirm
	filter  int // 0 all, 1 todo, 2 done

	// note editor
	editing  bool
	editID   int64
	editor   textarea.Model
	noteView ui.Scroller

	// pomodoro
	phase     phase
	onBreak   bool
	endAt     time.Time
	remaining time.Duration
	gen       int
}

// New creates the screen and loads persisted data.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx, store: &Store{
		TasksPath:    ctx.Paths.DataFile("tasks.json"),
		NotesPath:    ctx.Paths.DataFile("notes.json"),
		PomodoroPath: ctx.Paths.DataFile("pomodoro.json"),
	}}
	ctx.Warnings = append(ctx.Warnings, s.store.Load()...)
	return s
}

func (s *Screen) ID() string      { return "productivity" }
func (s *Screen) Title() string   { return ui.T("Productivity") }
func (s *Screen) Capturing() bool { return s.prompt.Active || s.editing }

func (s *Screen) Init() tea.Cmd {
	vim := s.ctx.Vim()
	s.tasks.Vim, s.notes.Vim, s.noteView.Vim = vim, vim, vim
	s.refresh()
	return nil
}

// OpenSub opens a tab: tasks, notes, pomodoro.
func (s *Screen) OpenSub(sub string) tea.Cmd {
	for i, t := range []string{"tasks", "notes", "pomodoro"} {
		if t == sub {
			s.tab = i
		}
	}
	return nil
}

func (s *Screen) Keys() []ui.Key {
	switch {
	case s.prompt.Active:
		return []ui.Key{{Keys: "enter", Desc: "save"}, {Keys: "esc", Desc: "cancel"}}
	case s.editing:
		return []ui.Key{{Keys: "ctrl+s", Desc: "save"}, {Keys: "esc", Desc: "cancel"}}
	}
	base := []ui.Key{{Keys: "tab", Desc: "switch tab"}}
	switch s.tab {
	case 0:
		base = append(base, ui.Key{Keys: "a", Desc: "add"}, ui.Key{Keys: "space", Desc: "done"}, ui.Key{Keys: "e", Desc: "edit"},
			ui.Key{Keys: "p", Desc: "priority"}, ui.Key{Keys: "f", Desc: "filter"}, ui.Key{Keys: "d", Desc: "delete"}, ui.Key{Keys: "c", Desc: "clear done"})
	case 1:
		base = append(base, ui.Key{Keys: "a", Desc: "new"}, ui.Key{Keys: "enter", Desc: "edit"}, ui.Key{Keys: "r", Desc: "rename"}, ui.Key{Keys: "d", Desc: "delete"})
	case 2:
		base = append(base, ui.Key{Keys: "space", Desc: "start/pause"}, ui.Key{Keys: "r", Desc: "reset"}, ui.Key{Keys: "s", Desc: "skip"})
	}
	return append(base, ui.Key{Keys: "esc", Desc: "back"})
}

var prioMark = []string{"", "!", "‼"}

func (s *Screen) refresh() {
	var ti []ui.Item
	open := 0
	for _, t := range s.store.Tasks {
		if !t.Done {
			open++
		}
		if s.filter == 1 && t.Done || s.filter == 2 && !t.Done {
			continue
		}
		box := "[ ]"
		if t.Done {
			box = "[✔]"
		}
		title := box + " " + t.Title
		if p := prioMark[min(t.Priority, 2)]; p != "" {
			title = box + " " + p + " " + t.Title
		}
		ti = append(ti, ui.Item{Title: title, Right: t.Created.Format("02 Jan"), Value: t.ID, Disabled: t.Done})
	}
	s.tasks.SetItems(ti)
	var ni []ui.Item
	for i := len(s.store.Notes) - 1; i >= 0; i-- {
		n := s.store.Notes[i]
		ni = append(ni, ui.Item{Title: n.Title, Right: n.Updated.Format("02 Jan 15:04"), Value: n.ID})
	}
	s.notes.SetItems(ni)
}

func (s *Screen) result(err error, ok string) tea.Cmd {
	s.refresh()
	if err != nil {
		return ui.Notify(ui.Error, err.Error())
	}
	if ok == "" {
		return nil
	}
	return ui.Notify(ui.Success, ui.T(ok))
}

func (s *Screen) openEditor(id int64, body string) tea.Cmd {
	s.editing, s.editID = true, id
	ta := textarea.New()
	ta.Placeholder = ui.T("Write your note… (first line becomes the title)")
	ta.ShowLineNumbers = false
	ta.CharLimit = 20000
	ta.SetValue(body)
	s.editor = ta
	return s.editor.Focus()
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case pomoTick:
		return s, s.tick(msg)
	case tea.MouseMsg:
		switch s.tab {
		case 0:
			s.tasks.HandleMouse(msg)
		case 1:
			s.notes.HandleMouse(msg)
		}
		return s, nil
	case tea.KeyMsg:
		return s, s.key(msg)
	}
	if s.editing {
		var cmd tea.Cmd
		s.editor, cmd = s.editor.Update(msg)
		return s, cmd
	}
	if s.prompt.Active {
		return s, s.prompt.Update(msg)
	}
	return s, nil
}

func (s *Screen) key(msg tea.KeyMsg) tea.Cmd {
	if s.prompt.Active {
		return s.prompt.Update(msg)
	}
	if s.editing {
		switch msg.String() {
		case "esc":
			s.editing = false
			return ui.Notify(ui.Info, ui.T("Edit cancelled"))
		case "ctrl+s":
			s.editing = false
			title := ""
			for _, n := range s.store.Notes {
				if n.ID == s.editID {
					title = n.Title
				}
			}
			_, err := s.store.SaveNote(s.editID, title, s.editor.Value())
			return s.result(err, "Note saved")
		}
		var cmd tea.Cmd
		s.editor, cmd = s.editor.Update(msg)
		return cmd
	}
	if s.confirm.Active {
		return s.confirm.Update(msg)
	}
	k := msg.String()
	switch k {
	case "esc", "q":
		return ui.BackCmd
	case "tab", "right":
		s.tab = (s.tab + 1) % len(tabs)
		return nil
	case "shift+tab", "left":
		s.tab = (s.tab + len(tabs) - 1) % len(tabs)
		return nil
	}
	switch s.tab {
	case 0:
		return s.taskKey(msg)
	case 1:
		return s.noteKey(msg)
	default:
		return s.pomoKey(k)
	}
}

func (s *Screen) selectedTask() (int64, bool) {
	it, ok := s.tasks.Selected()
	if !ok {
		return 0, false
	}
	return it.Value.(int64), true
}

func (s *Screen) taskKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "a", "n":
		return s.prompt.Open(s.ctx.S, "New task", "Buy coffee", "", func(v string) tea.Cmd {
			_, err := s.store.AddTask(v)
			s.tasks.Cursor = len(s.tasks.Items)
			return s.result(err, "Task added")
		})
	case " ", "x", "enter":
		if id, ok := s.selectedTask(); ok {
			return s.result(s.store.ToggleTask(id), "")
		}
	case "e":
		if id, ok := s.selectedTask(); ok {
			cur := ""
			for _, t := range s.store.Tasks {
				if t.ID == id {
					cur = t.Title
				}
			}
			return s.prompt.Open(s.ctx.S, "Edit task", "", cur, func(v string) tea.Cmd {
				v = strings.TrimSpace(v)
				if v == "" {
					return nil
				}
				return s.result(s.store.UpdateTask(id, func(t *Task) { t.Title = v }), "Task updated")
			})
		}
	case "p":
		if id, ok := s.selectedTask(); ok {
			return s.result(s.store.UpdateTask(id, func(t *Task) { t.Priority = (t.Priority + 1) % 3 }), "")
		}
	case "f":
		s.filter = (s.filter + 1) % 3
		s.refresh()
	case "d", "delete":
		if id, ok := s.selectedTask(); ok {
			s.confirm.Ask(ui.T("Delete this task?"), func() tea.Cmd { return s.result(s.store.DeleteTask(id), "Task deleted") })
		}
	case "c":
		s.confirm.Ask(ui.T("Remove all completed tasks?"), func() tea.Cmd {
			n, err := s.store.ClearDone()
			return s.result(err, ui.Tf("%d task(s) removed", n))
		})
	default:
		s.tasks.HandleKey(msg)
	}
	return nil
}

func (s *Screen) selectedNote() (Note, bool) {
	it, ok := s.notes.Selected()
	if !ok {
		return Note{}, false
	}
	for _, n := range s.store.Notes {
		if n.ID == it.Value.(int64) {
			return n, true
		}
	}
	return Note{}, false
}

func (s *Screen) noteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "a", "n":
		return s.openEditor(0, "")
	case "enter", "e":
		if n, ok := s.selectedNote(); ok {
			return s.openEditor(n.ID, n.Body)
		}
	case "r":
		if n, ok := s.selectedNote(); ok {
			return s.prompt.Open(s.ctx.S, "Rename note", "", n.Title, func(v string) tea.Cmd {
				_, err := s.store.SaveNote(n.ID, v, n.Body)
				return s.result(err, "Note renamed")
			})
		}
	case "d", "delete":
		if n, ok := s.selectedNote(); ok {
			s.confirm.Ask(ui.Tf("Delete note %q?", n.Title), func() tea.Cmd { return s.result(s.store.DeleteNote(n.ID), "Note deleted") })
		}
	case "pgdown", "ctrl+d":
		s.noteView.HandleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	case "pgup", "ctrl+u":
		s.noteView.HandleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	default:
		if s.notes.HandleKey(msg) {
			s.noteView.Top()
		}
	}
	return nil
}

// --- Pomodoro -------------------------------------------------------------

func (s *Screen) phaseLen() time.Duration {
	p := s.ctx.Cfg.Productivity
	if s.onBreak {
		return time.Duration(p.PomodoroBreak) * time.Minute
	}
	return time.Duration(p.PomodoroWork) * time.Minute
}

func (s *Screen) schedule() tea.Cmd {
	gen := s.gen
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return pomoTick{gen} })
}

func (s *Screen) pomoKey(k string) tea.Cmd {
	switch k {
	case " ", "enter":
		switch s.phase {
		case idle:
			s.phase, s.endAt = focus, time.Now().Add(s.phaseLen())
			s.gen++
			return s.schedule()
		case focus:
			s.phase, s.remaining = pause, time.Until(s.endAt)
			s.gen++
		case pause:
			s.phase, s.endAt = focus, time.Now().Add(s.remaining)
			s.gen++
			return s.schedule()
		}
	case "r":
		s.phase, s.onBreak = idle, false
		s.gen++
	case "s":
		return s.finishPhase(false)
	}
	return nil
}

func (s *Screen) tick(m pomoTick) tea.Cmd {
	if m.gen != s.gen || s.phase != focus {
		return nil
	}
	if time.Now().Before(s.endAt) {
		return s.schedule()
	}
	return s.finishPhase(true)
}

func (s *Screen) finishPhase(completed bool) tea.Cmd {
	var note tea.Cmd
	if !s.onBreak && completed {
		_ = s.store.RecordPomodoro(s.ctx.Cfg.Productivity.PomodoroWork, time.Now())
		note = ui.Notify(ui.Success, ui.T("🍅 Focus session complete — time for a break!"))
	} else if s.onBreak && completed {
		note = ui.Notify(ui.Info, ui.T("Break is over — ready for the next focus session?"))
	}
	s.onBreak = !s.onBreak
	s.gen++
	if s.phase == idle {
		return note
	}
	s.phase, s.endAt = focus, time.Now().Add(s.phaseLen())
	return tea.Batch(note, s.schedule())
}

// Status is shown elsewhere (e.g. dashboard) — remaining time if running.
func (s *Screen) pomoRemaining() time.Duration {
	switch s.phase {
	case focus:
		return max(0, time.Until(s.endAt))
	case pause:
		return s.remaining
	default:
		return s.phaseLen()
	}
}

// --- Views ------------------------------------------------------------------

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	iw, ih := st.PanelInner(w, h)
	head := st.Tabs(tabs, s.tab) + "\n" + st.Sep(iw) + "\n"
	bottom := ""
	switch {
	case s.prompt.Active:
		bottom = s.prompt.View(st, iw)
	case s.confirm.Active:
		bottom = s.confirm.View(st)
	}
	bh := 0
	if bottom != "" {
		bh = lipgloss.Height(bottom) + 1
	}
	area := ih - 2 - bh
	var body string
	switch s.tab {
	case 0:
		body = s.taskView(iw, area)
	case 1:
		body = s.noteViewRender(iw, area)
	case 2:
		body = s.pomoView(iw, area)
	}
	out := head + ui.Fit(body, iw, area)
	if bottom != "" {
		out += "\n" + st.Sep(iw) + "\n" + bottom
	}
	return st.Panel(s.Title(), out, w, h, true)
}

func (s *Screen) taskView(w, h int) string {
	st := s.ctx.S
	done := 0
	for _, t := range s.store.Tasks {
		if t.Done {
			done++
		}
	}
	total := len(s.store.Tasks)
	pct := 0.0
	if total > 0 {
		pct = float64(done) / float64(total) * 100
	}
	filters := []string{"all", "to do", "done"}
	head := st.Text.Render(ui.Tf("%d/%d done", done, total)) + "  " + st.Bar(pct, min(30, w/3)) +
		st.Muted.Render("   "+ui.T("view: ")+ui.T(filters[s.filter]))
	if total == 0 {
		return head + "\n\n" + st.Muted.Render(ui.T("No tasks yet — press a to add one."))
	}
	return head + "\n\n" + s.tasks.View(st, w, h-2)
}

func (s *Screen) noteViewRender(w, h int) string {
	st := s.ctx.S
	if s.editing {
		s.editor.SetWidth(w)
		s.editor.SetHeight(h - 2)
		return st.Accent.Render(ui.T("Editing note — Ctrl+S to save, Esc to cancel")) + "\n\n" + s.editor.View()
	}
	if len(s.store.Notes) == 0 {
		return st.Muted.Render(ui.T("No notes yet — press a to write one."))
	}
	lw := min(36, w/3)
	left := ui.Fit(s.notes.View(st, lw, h), lw, h)
	rw := w - lw - 3
	right := ""
	if n, ok := s.selectedNote(); ok {
		s.noteView.SetContent(st.Title.Render(n.Title) + "\n" + st.Muted.Render(ui.Tf("updated %s", n.Updated.Format("2006-01-02 15:04"))) +
			"\n\n" + st.Text.Render(ui.Wrap(n.Body, rw)))
		right = s.noteView.View(st, rw, h)
	}
	sep := strings.TrimRight(strings.Repeat(st.Muted.Render(" │ ")+"\n", h), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

var bigDigits = map[rune][3]string{
	'0': {"█▀█", "█ █", "▀▀▀"}, '1': {" ▀█", "  █", "  ▀"}, '2': {"▀▀█", "█▀▀", "▀▀▀"},
	'3': {"▀▀█", " ▀█", "▀▀▀"}, '4': {"█ █", "▀▀█", "  ▀"}, '5': {"█▀▀", "▀▀█", "▀▀▀"},
	'6': {"█▀▀", "█▀█", "▀▀▀"}, '7': {"▀▀█", "  █", "  ▀"}, '8': {"█▀█", "█▀█", "▀▀▀"},
	'9': {"█▀█", "▀▀█", "▀▀▀"}, ':': {" ", "▪", "▪"},
}

// BigClock renders mm:ss with block digits.
func BigClock(text string) string {
	var rows [3]string
	for _, r := range text {
		g, ok := bigDigits[r]
		if !ok {
			continue
		}
		for i := range rows {
			rows[i] += g[i] + " "
		}
	}
	return strings.Join(rows[:], "\n")
}

func (s *Screen) pomoView(w, h int) string {
	st := s.ctx.S
	rem := s.pomoRemaining().Round(time.Second)
	clock := fmt.Sprintf("%02d:%02d", int(rem.Minutes()), int(rem.Seconds())%60)
	label := ui.T("Focus")
	style := st.Primary
	if s.onBreak {
		label, style = ui.T("Break"), st.Success
	}
	state := map[phase]string{idle: ui.T("ready"), focus: ui.T("running"), pause: ui.T("paused")}[s.phase]
	total := s.phaseLen()
	pct := 0.0
	if total > 0 {
		pct = (1 - float64(rem)/float64(total)) * 100
	}
	center := func(x string) string {
		var out []string
		for _, l := range strings.Split(x, "\n") {
			out = append(out, ui.Center(l, w))
		}
		return strings.Join(out, "\n")
	}
	ps := s.store.Pomodoro
	today := ps.Today
	if ps.Day != time.Now().Format("2006-01-02") {
		today = 0
	}
	body := "\n" + center(style.Bold(true).Render(strings.ToUpper(label))+"  "+st.Muted.Render("("+state+")")) + "\n\n" +
		center(style.Render(BigClock(clock))) + "\n\n" +
		center(st.Bar(pct, min(50, w-4))) + "\n\n" +
		center(st.Muted.Render(ui.Tf("Focus %d min · break %d min (change in Settings)", s.ctx.Cfg.Productivity.PomodoroWork, s.ctx.Cfg.Productivity.PomodoroBreak))) + "\n" +
		center(st.Text.Render(ui.Tf("Today: %d 🍅   Total: %d   Focus time: %d h %02d min", today, ps.Total, ps.FocusMins/60, ps.FocusMins%60))) + "\n\n" +
		center(st.Key.Render("space")+st.Muted.Render(" "+ui.T("start/pause")+"  ")+st.Key.Render("r")+st.Muted.Render(" "+ui.T("reset")+"  ")+st.Key.Render("s")+st.Muted.Render(" "+ui.T("skip")))
	return body
}
