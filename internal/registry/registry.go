// Package registry describes the built-in TermVerse modules and discovers
// local plugin manifests. Plugin code is never downloaded or executed: V1
// only lists manifests so users can see what is installed.
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// Module describes a built-in module.
type Module struct {
	ID          string
	Number      int // menu number 0-10, -1 for sub-components
	Name        string
	Description string
	Version     string
	Optional    bool   // can be disabled from the Plugin Hub
	Parent      string // for sub-components
}

// Sections are the 11 main menu entries, in order.
var Sections = []Module{
	{ID: "updates", Number: 0, Name: "Updates & News", Description: "Changelog, release notes and update check"},
	{ID: "devtools", Number: 1, Name: "Developer Tools", Description: "Git, JSON, UUID, Base64, hashes, regex, HTTP, ports", Optional: true},
	{ID: "arcade", Number: 2, Name: "Terminal Arcade", Description: "Snake, Sky Hopper, 2048, Mines, typing test, online room", Optional: true},
	{ID: "system", Number: 3, Name: "System Center", Description: "Live CPU, memory, disks, sensors and processes", Optional: true},
	{ID: "network", Number: 4, Name: "Network Tools", Description: "Interfaces, DNS lookup, TCP test, connections", Optional: true},
	{ID: "files", Number: 5, Name: "File Manager", Description: "Read-only file browser with preview", Optional: true},
	{ID: "productivity", Number: 6, Name: "Productivity", Description: "Tasks, quick notes and Pomodoro timer", Optional: true},
	{ID: "automation", Number: 7, Name: "Automation Lab", Description: "Saved commands run only after confirmation", Optional: true},
	{ID: "customize", Number: 8, Name: "Customization", Description: "Themes, colors, borders, ASCII art, widgets"},
	{ID: "plugins", Number: 9, Name: "Plugin Hub", Description: "Built-in modules and local plugins"},
	{ID: "settings", Number: 10, Name: "Settings & About", Description: "Preferences, updates, reset, license"},
}

// Components are built-in sub-modules shown in the Plugin Hub.
var Components = []Module{
	{ID: "devtools.git", Parent: "devtools", Name: "Git Inspector", Description: "Branch, status, remotes and recent commits"},
	{ID: "devtools.json", Parent: "devtools", Name: "JSON Formatter", Description: "Validate, pretty-print and minify JSON"},
	{ID: "devtools.uuid", Parent: "devtools", Name: "UUID Generator", Description: "Random v4 and time-ordered v7 UUIDs"},
	{ID: "devtools.base64", Parent: "devtools", Name: "Base64", Description: "Encode and decode Base64 / URL-safe Base64"},
	{ID: "devtools.hash", Parent: "devtools", Name: "Hash Calculator", Description: "MD5, SHA-1, SHA-256, SHA-512 of text"},
	{ID: "devtools.time", Parent: "devtools", Name: "Timestamp Converter", Description: "Unix time <-> human dates"},
	{ID: "devtools.password", Parent: "devtools", Name: "Password Generator", Description: "Cryptographically secure passwords"},
	{ID: "devtools.regex", Parent: "devtools", Name: "Regex Tester", Description: "Test Go regular expressions live"},
	{ID: "devtools.http", Parent: "devtools", Name: "HTTP Client", Description: "Send GET/HEAD requests and inspect responses"},
	{ID: "devtools.ports", Parent: "devtools", Name: "Listening Ports", Description: "Sockets in LISTEN state"},
	{ID: "arcade.snake", Parent: "arcade", Name: "Snake", Description: "Classic snake with speed levels"},
	{ID: "arcade.hopper", Parent: "arcade", Name: "Sky Hopper", Description: "Flap through the gaps"},
	{ID: "arcade.guess", Parent: "arcade", Name: "Number Guess", Description: "Find the secret number"},
	{ID: "arcade.merge", Parent: "arcade", Name: "2048", Description: "Slide and merge tiles"},
	{ID: "arcade.mines", Parent: "arcade", Name: "Mines", Description: "Clear the field without hitting a mine"},
	{ID: "arcade.typing", Parent: "arcade", Name: "Typing Test", Description: "Measure your words per minute"},
	{ID: "arcade.online", Parent: "arcade", Name: "Online Room", Description: "LAN/Internet chat and Four in a Row"},
	{ID: "system.procs", Parent: "system", Name: "Process Manager", Description: "Sortable process list with signals"},
}

func init() {
	for i := range Sections {
		Sections[i].Version = version.Version
	}
	for i := range Components {
		Components[i].Version = version.Version
		Components[i].Number = -1
	}
}

// Section returns the section with id.
func Section(id string) (Module, bool) {
	for _, m := range Sections {
		if m.ID == id {
			return m, true
		}
	}
	return Module{}, false
}

// Count returns the number of built-in modules (sections + components).
func Count() int { return len(Sections) + len(Components) }

// Manifest is the plugin.json file of a local plugin.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Homepage    string `json:"homepage"`
	// Dir is where the manifest was found (not serialized).
	Dir   string `json:"-"`
	Error string `json:"-"`
}

// Discover lists plugin manifests in dir/<plugin>/plugin.json. Nothing is
// executed. Invalid manifests are returned with Error set.
func Discover(dir string) ([]Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "plugin.json")
		m := Manifest{Name: e.Name(), Dir: filepath.Dir(p)}
		data, err := os.ReadFile(p)
		if err != nil {
			m.Error = "missing plugin.json"
			out = append(out, m)
			continue
		}
		if len(data) > 64*1024 {
			m.Error = "plugin.json too large"
			out = append(out, m)
			continue
		}
		if err := json.Unmarshal(data, &m); err != nil {
			m.Error = fmt.Sprintf("invalid plugin.json: %v", err)
		} else if strings.TrimSpace(m.Name) == "" {
			m.Name, m.Error = e.Name(), "manifest has no name"
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
