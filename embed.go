// Package termverse exposes the static assets bundled into the TermVerse
// binary: the changelog, the built-in themes and the default ASCII art.
package termverse

import "embed"

// Assets contains the files embedded at build time.
//
//go:embed CHANGELOG.md assets/themes/*.json assets/ascii/*.txt
var Assets embed.FS
