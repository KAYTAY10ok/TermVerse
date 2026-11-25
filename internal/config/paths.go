package config

import (
	"errors"
	"os"
	"path/filepath"
)

// Paths lists every location TermVerse reads or writes.
type Paths struct {
	ConfigDir  string // ~/.config/termverse
	ConfigFile string // ~/.config/termverse/config.json
	ThemesDir  string // ~/.config/termverse/themes
	ASCIIDir   string // ~/.config/termverse/ascii
	PluginsDir string // ~/.config/termverse/plugins
	DataDir    string // ~/.local/share/termverse
}

// ResolvePaths computes the paths following the XDG Base Directory spec:
// $XDG_CONFIG_HOME (default ~/.config) and $XDG_DATA_HOME (default
// ~/.local/share). Relative XDG values are ignored, as the spec requires.
func ResolvePaths() (Paths, error) {
	home, herr := os.UserHomeDir()
	xdg := func(env, fallback string) (string, error) {
		if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
			return v, nil
		}
		if herr != nil || home == "" {
			return "", errors.New("cannot determine home directory and " + env + " is not set")
		}
		return filepath.Join(home, fallback), nil
	}
	cfgHome, err := xdg("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return Paths{}, err
	}
	dataHome, err := xdg("XDG_DATA_HOME", filepath.Join(".local", "share"))
	if err != nil {
		return Paths{}, err
	}
	cd := filepath.Join(cfgHome, "termverse")
	return Paths{
		ConfigDir:  cd,
		ConfigFile: filepath.Join(cd, "config.json"),
		ThemesDir:  filepath.Join(cd, "themes"),
		ASCIIDir:   filepath.Join(cd, "ascii"),
		PluginsDir: filepath.Join(cd, "plugins"),
		DataDir:    filepath.Join(dataHome, "termverse"),
	}, nil
}

// DataFile returns the path of a file in the data directory.
func (p Paths) DataFile(name string) string { return filepath.Join(p.DataDir, name) }
