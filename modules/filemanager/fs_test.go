package filemanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDirSortsDirsFirstAndHidesDotfiles(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "b.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(d, ".hidden"), nil, 0o644)
	os.Mkdir(filepath.Join(d, "zdir"), 0o755)
	es, err := ReadDir(d, false, "name")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 || es[0].Name != "zdir" || es[1].Name != "b.txt" {
		t.Fatalf("unexpected %+v", es)
	}
	es, _ = ReadDir(d, true, "name")
	if len(es) != 3 {
		t.Fatalf("hidden not shown: %d", len(es))
	}
}

func TestPreviewBinaryDetection(t *testing.T) {
	d := t.TempDir()
	txt := filepath.Join(d, "a.txt")
	bin := filepath.Join(d, "a.bin")
	os.WriteFile(txt, []byte("héllo\nworld"), 0o644)
	os.WriteFile(bin, []byte{0, 1, 2, 3, 0xff}, 0o644)
	if s, b, err := Preview(txt, 1024); err != nil || b || s != "héllo\nworld" {
		t.Errorf("text preview: %q %v %v", s, b, err)
	}
	if _, b, _ := Preview(bin, 1024); !b {
		t.Error("binary not detected")
	}
	// Truncating in the middle of a multi-byte rune must not look binary.
	if _, b, _ := Preview(txt, 2); b {
		t.Error("partial rune flagged as binary")
	}
	dump, err := HexDump(bin, 16)
	if err != nil || len(dump) == 0 {
		t.Error("hexdump failed")
	}
}

func TestHumanSize(t *testing.T) {
	if HumanSize(512) != "512 B" || HumanSize(2048) != "2.0 K" {
		t.Error(HumanSize(512), HumanSize(2048))
	}
}
