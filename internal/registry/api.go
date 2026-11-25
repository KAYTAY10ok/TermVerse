package registry

// This file defines the contract planned for community plugins (post-V1).
//
// Design goals:
//   - Plugins are opt-in and installed locally by the user; TermVerse never
//     fetches or runs remote code by itself.
//   - A plugin is described by a plugin.json manifest (see Manifest).
//   - The planned runtime is an out-of-process executable speaking JSON-RPC
//     over stdin/stdout, launched only after explicit user approval, so a
//     faulty plugin cannot crash or compromise the TUI process.
//
// The Go interface below is what built-in modules already satisfy through
// ui.Screen; community plugins will be adapted to it by a host bridge.

// Capability is a permission a plugin may request in its manifest.
type Capability string

// Planned capabilities.
const (
	CapNetwork Capability = "network"
	CapFSRead  Capability = "fs:read"
	CapFSWrite Capability = "fs:write"
	CapExec    Capability = "exec"
)

// Host is the API exposed to plugins by TermVerse (planned).
type Host interface {
	// Notify shows a message in the status bar.
	Notify(text string)
	// DataDir returns a private, per-plugin data directory.
	DataDir() string
	// Theme returns the active palette as hex colors keyed by role.
	Theme() map[string]string
}

// Plugin is implemented by plugin bridges (planned).
type Plugin interface {
	Manifest() Manifest
	// Render returns the plugin view for the given size.
	Render(width, height int) string
	// HandleKey processes a key press; it returns false to give control back.
	HandleKey(key string) bool
}
