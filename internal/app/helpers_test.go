package app_test

import (
	"os"
	"testing"
	"time"
)

func waitShort() <-chan time.Time { return time.After(60 * time.Millisecond) }

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
