// Command termverse starts the TermVerse terminal environment.
package main

import (
	"fmt"
	"os"

	"github.com/KAYTAY10ok/TermVerse/internal/app"
	"github.com/KAYTAY10ok/TermVerse/internal/cli"
	"github.com/KAYTAY10ok/TermVerse/modules/arcade"
	"github.com/KAYTAY10ok/TermVerse/modules/automation"
	"github.com/KAYTAY10ok/TermVerse/modules/customization"
	"github.com/KAYTAY10ok/TermVerse/modules/devtools"
	"github.com/KAYTAY10ok/TermVerse/modules/filemanager"
	"github.com/KAYTAY10ok/TermVerse/modules/network"
	"github.com/KAYTAY10ok/TermVerse/modules/plugins"
	"github.com/KAYTAY10ok/TermVerse/modules/productivity"
	"github.com/KAYTAY10ok/TermVerse/modules/settings"
	"github.com/KAYTAY10ok/TermVerse/modules/systemcenter"
	"github.com/KAYTAY10ok/TermVerse/modules/updates"
)

func factories() map[string]app.Factory {
	return map[string]app.Factory{
		"updates":      updates.New,
		"devtools":     devtools.New,
		"arcade":       arcade.New,
		"system":       systemcenter.New,
		"network":      network.New,
		"files":        filemanager.New,
		"productivity": productivity.New,
		"automation":   automation.New,
		"customize":    customization.New,
		"plugins":      plugins.New,
		"settings":     settings.New,
	}
}

func main() { os.Exit(run()) }

func run() int {
	paths, reg, warnings, err := app.Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "termverse:", err)
		return 1
	}
	code, interactive := cli.Run(os.Args[1:], cli.Env{
		Paths: paths, Themes: reg, Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
	})
	if !interactive {
		return code
	}
	ctx, err := app.Context(paths, reg, warnings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "termverse:", err)
		return 1
	}
	if err := app.Run(ctx, factories()); err != nil {
		fmt.Fprintln(os.Stderr, "termverse:", err)
		return 1
	}
	return 0
}
