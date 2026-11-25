// Package dashboard renders the TermVerse home screen.
package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// MenuWidth is the preferred width of the main menu panel.
const MenuWidth = 46

// Layout reports where the menu items were drawn (for mouse clicks).
type Layout struct {
	MenuX, MenuY, MenuW int // top-left of the first menu row
	MenuRows            int
}

var tips = []string{
	"Press Ctrl+P to open the command palette.",
	"Press ? anywhere to see keyboard shortcuts.",
	"Digits 0-9 open a section directly; use arrows + Enter for [10].",
	"Drop your own theme JSON in ~/.config/termverse/themes.",
	"Change the ASCII art from Customization [8].",
	"Tasks and notes are saved in ~/.local/share/termverse.",
	"Automation Lab always asks before running a command.",
	"Host an Online Room in the Arcade to chat with friends on your LAN.",
	"Disable optional modules you do not use in the Plugin Hub [9].",
}

// Tip returns the tip of the moment (changes every minute).
func Tip(now time.Time) string { return ui.T(tips[(now.Unix()/60)%int64(len(tips))]) }

// MenuLabel formats a section label.
func MenuLabel(m registry.Module) string {
	return fmt.Sprintf("%-5s %s", fmt.Sprintf("[%d]", m.Number), ui.T(m.Name))
}

// Render draws the dashboard body (without the global header/footer).
func Render(ctx *ui.Context, menu *ui.List, width, height int, now time.Time) (string, Layout) {
	s := ctx.S
	w := ctx.Cfg.Widgets
	var lay Layout

	art := ""
	if w.ASCII && ctx.ASCII != "" {
		art = ctx.ASCII
		if ctx.Cfg.Appearance.ASCIIGradient {
			phase := 0.0
			if ctx.Cfg.Appearance.Animations {
				phase = float64(now.Unix()%20) / 20
			}
			art = ui.Gradient(art, s.C.Primary, s.C.Secondary, phase)
		} else {
			art = s.Primary.Render(art)
		}
	}
	artW, artH := lipgloss.Width(art), lipgloss.Height(art)
	if art == "" {
		artH = 0
	}

	menuH := len(menu.Items) + 4
	menuW := min(MenuWidth, width)

	// Narrow layout: menu only.
	if width < 80 {
		body := ""
		used := 0
		if art != "" && artW <= width && height >= menuH+artH+2 {
			body = ui.Center(art, width) + "\n"
			// Center works per line only; re-center each line.
			lines := strings.Split(art, "\n")
			for i, l := range lines {
				lines[i] = strings.Repeat(" ", max(0, (width-artW)/2)) + l
			}
			body = strings.Join(lines, "\n") + "\n"
			used = artH + 1
		}
		mh := min(menuH, height-used)
		body += menuPanel(ctx, menu, menuW, mh)
		lay = Layout{MenuX: 2, MenuY: used + 1, MenuW: menuW - 4, MenuRows: mh - 4}
		if w.SystemInfo && height-used-mh >= 3 {
			body += "\n" + compactInfo(ctx, width)
		}
		return ui.Fit(body, width, height), lay
	}

	// Wide layout: left column (art + system info), right column (menu + info).
	rightW := menuW
	leftW := width - rightW - 1
	var left []string
	leftUsed := 0
	if art != "" && artW <= leftW && height-artH >= 12 {
		pad := strings.Repeat(" ", max(0, (leftW-artW)/2))
		lines := strings.Split(art, "\n")
		for i := range lines {
			lines[i] = pad + lines[i]
		}
		left = append(left, "", strings.Join(lines, "\n"), "")
		leftUsed = artH + 2
	}
	if w.SystemInfo {
		h := height - leftUsed
		if w.Tips && h > 18 {
			h -= 3
		}
		left = append(left, s.Panel(ui.T("System"), sysInfo(ctx, leftW-4), leftW, h, false))
		leftUsed += h
	}
	if w.Tips && height-leftUsed >= 3 {
		left = append(left, s.Panel(ui.T("Tip"), s.Muted.Render("✦ ")+s.Text.Render(Tip(now)), leftW, 3, false))
	}
	leftBlock := ui.Fit(strings.Join(left, "\n"), leftW, height)

	var right []string
	mh := min(menuH, height)
	right = append(right, menuPanel(ctx, menu, rightW, mh))
	lay = Layout{MenuX: leftW + 1 + 2, MenuY: 1, MenuW: rightW - 4, MenuRows: mh - 4}
	if w.TermVerseInfo && height-mh >= 5 {
		right = append(right, s.Panel("TermVerse", tvInfo(ctx, now), rightW, min(height-mh, 9), false))
	}
	rightBlock := ui.Fit(strings.Join(right, "\n"), rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, " ", rightBlock), lay
}

func menuPanel(ctx *ui.Context, menu *ui.List, w, h int) string {
	s := ctx.S
	iw, ih := s.PanelInner(w, h)
	body := menu.View(s, iw, ih-2)
	if it, ok := menu.Selected(); ok {
		m := it.Value.(registry.Module)
		desc := ui.T(m.Description)
		if it.Disabled {
			desc = ui.T("Disabled — enable it in the Plugin Hub [9]")
		}
		body = ui.Fit(body, iw, ih-2) + "\n" + s.Sep(iw) + "\n" + s.Muted.Render(ui.Truncate(desc, iw))
	}
	return s.Panel(ui.T("Main menu"), body, w, h, true)
}

func na(ok bool, v string) string {
	if !ok {
		return system.NA
	}
	return v
}

func sysInfo(ctx *ui.Context, width int) string {
	s := ctx.S
	if !ctx.SysOK {
		return s.Muted.Render(ui.T("Collecting system information…"))
	}
	sn := ctx.Sys
	barW := max(10, min(30, width-14))
	rows := [][2]string{
		{ui.T("User"), sn.User},
		{ui.T("Host"), sn.Hostname},
		{ui.T("OS"), sn.OS},
		{ui.T("Kernel"), sn.Kernel},
		{ui.T("Arch"), sn.Arch},
		{ui.T("CPU"), fmt.Sprintf("%s (%d)", sn.CPUModel, sn.Threads)},
		{ui.T("Uptime"), na(sn.UptimeOK, system.HumanDuration(sn.Uptime))},
		{ui.T("Processes"), na(sn.ProcsOK, fmt.Sprint(sn.Procs))},
		{ui.T("Local IP"), sn.LocalIP},
	}
	out := s.KV(rows, 11) + "\n"
	bar := func(label string, ok bool, pct float64, extra string) string {
		l := s.Muted.Render(ui.PadRight(label, 11)) + " "
		if !ok {
			return l + s.Text.Render(system.NA)
		}
		return l + s.Bar(pct, barW) + " " + s.Muted.Render(extra)
	}
	out += "\n" + bar(ui.T("CPU"), sn.CPUOK, sn.CPUPercent, "")
	out += "\n" + bar(ui.T("Memory"), sn.MemOK, sn.MemPercent,
		fmt.Sprintf("%s / %s", system.HumanBytes(sn.MemUsed), system.HumanBytes(sn.MemTotal)))
	out += "\n" + bar(ui.T("Disk /"), sn.RootDiskOK, sn.RootDisk.Percent,
		fmt.Sprintf("%s %s", system.HumanBytes(sn.RootDisk.Free), ui.T("free")))
	return out
}

func compactInfo(ctx *ui.Context, width int) string {
	s := ctx.S
	if !ctx.SysOK {
		return ""
	}
	sn := ctx.Sys
	line := fmt.Sprintf("%s@%s · CPU %s · RAM %s · %s",
		sn.User, sn.Hostname,
		na(sn.CPUOK, fmt.Sprintf("%.0f%%", sn.CPUPercent)),
		na(sn.MemOK, fmt.Sprintf("%.0f%%", sn.MemPercent)),
		na(sn.UptimeOK, system.HumanDuration(sn.Uptime)))
	return s.Muted.Render(ui.Truncate(line, width))
}

func tvInfo(ctx *ui.Context, now time.Time) string {
	s := ctx.S
	last := ui.T("never")
	if !ctx.LastUpdateCheck.IsZero() {
		last = ctx.LastUpdateCheck.Local().Format("2006-01-02 15:04")
		if ctx.LatestVersion != "" {
			last += " (v" + ctx.LatestVersion + ")"
		}
	}
	rows := [][2]string{
		{ui.T("Version"), "v" + version.Version},
		{ui.T("Theme"), ctx.Theme.Name},
		{ui.T("Config"), ui.Tildify(ctx.Paths.ConfigFile)},
		{ui.T("Modules"), fmt.Sprintf("%d", ctx.ModuleCount)},
		{ui.T("Update check"), last},
	}
	out := s.KV(rows, 12)
	if n := len(ctx.Warnings); n > 0 {
		out += "\n" + s.Warning.Render(ui.Tf("⚠ %d warning(s) — see Settings [10]", n))
	}
	return out
}
