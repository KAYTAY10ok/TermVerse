// Package update checks GitHub for new TermVerse releases. It only reads
// release metadata: nothing is downloaded or installed automatically.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
)

// Release is the subset of the GitHub release API we use.
type Release struct {
	Tag         string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	URL         string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

// Version returns the tag without a leading "v".
func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// ErrNoRelease is returned when the repository has no published release.
var ErrNoRelease = errors.New("no release has been published yet")

// APIBase can be overridden in tests.
var APIBase = "https://api.github.com"

// Latest fetches the latest release of repo ("owner/name") with a timeout.
func Latest(ctx context.Context, repo string) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/repos/%s/releases/latest", APIBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TermVerse-update-check")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return Release{}, errors.New("request timed out after 10s")
		}
		return Release{}, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Release{}, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Release{}, ErrNoRelease
	case http.StatusForbidden, http.StatusTooManyRequests:
		return Release{}, errors.New("GitHub API rate limit reached, try again later")
	default:
		return Release{}, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var r Release
	if err := json.Unmarshal(body, &r); err != nil {
		return Release{}, fmt.Errorf("invalid response: %w", err)
	}
	return r, nil
}

// Compare compares two semantic versions (with or without "v").
// It returns -1, 0 or 1. Pre-release suffixes sort before the release.
func Compare(a, b string) int {
	pa, preA := parse(a)
	pb, preB := parse(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case preA == preB:
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	case preA < preB:
		return -1
	default:
		return 1
	}
}

func parse(v string) ([3]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "+")
	core, pre, _ := strings.Cut(v, "-")
	var out [3]int
	for i, p := range strings.SplitN(core, ".", 3) {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out, pre
}

// State is persisted between runs.
type State struct {
	LastCheck time.Time `json:"last_check"`
	Latest    string    `json:"latest_version"`
}

// LoadState reads the state file (missing file → zero state).
func LoadState(path string) State {
	var s State
	_, _ = fsutil.ReadJSON(path, &s)
	return s
}

// SaveState writes the state file atomically.
func SaveState(path string, s State) error { return fsutil.WriteJSONAtomic(path, s) }

// ChangelogSection extracts the section for version v from a Keep-a-Changelog file.
func ChangelogSection(changelog, v string) string {
	lines := strings.Split(changelog, "\n")
	var out []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = strings.Contains(l, "["+v+"]") || strings.Contains(l, " "+v)
		}
		if in {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
