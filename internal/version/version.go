// Package version holds build metadata. Values are overridden at link time:
//
//	go build -ldflags "-X github.com/KAYTAY10ok/TermVerse/internal/version.Version=0.1.0"
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of the build (without leading "v").
	Version = "0.1.0"
	// Commit is the git commit the binary was built from.
	Commit = "unknown"
	// Date is the build date (RFC3339).
	Date = "unknown"
)

const (
	// Name is the application name.
	Name = "TermVerse"
	// RepoOwner and RepoName identify the upstream GitHub repository.
	RepoOwner = "KAYTAY10ok"
	RepoName  = "TermVerse"
	// License is the SPDX identifier of the project license.
	License = "MIT"
)

// RepoURL returns the GitHub URL of the project.
func RepoURL() string { return fmt.Sprintf("https://github.com/%s/%s", RepoOwner, RepoName) }

// String returns a human readable version line.
func String() string {
	return fmt.Sprintf("%s %s (commit %s, built %s, %s %s/%s)",
		Name, Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
