// Package fsutil provides small, safe filesystem helpers.
package fsutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path atomically: it writes a temporary file
// in the same directory, syncs it and renames it over the destination. A
// crash mid-write therefore never leaves a truncated file behind.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// WriteJSONAtomic marshals v with indentation and writes it atomically.
func WriteJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, 0o644)
}

// ReadJSON decodes the JSON file at path into v. It returns (false, nil) when
// the file does not exist.
func ReadJSON(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return true, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return true, nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteIfMissing writes data to path only when the file does not exist yet.
// It never overwrites user files. It reports whether a file was written.
func WriteIfMissing(path string, data []byte, perm os.FileMode) (bool, error) {
	if Exists(path) {
		return false, nil
	}
	if err := WriteFileAtomic(path, data, perm); err != nil {
		return false, err
	}
	return true, nil
}

// BackupCorrupt renames a broken file to "<name>.invalid-<suffix>" so the
// user's data is preserved for manual recovery. It returns the new path.
func BackupCorrupt(path, suffix string) (string, error) {
	dst := path + ".invalid-" + suffix
	if err := os.Rename(path, dst); err != nil {
		return "", err
	}
	return dst, nil
}
