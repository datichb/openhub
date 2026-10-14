package files

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Open(root, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if b, _ := io.ReadAll(f); string(b) != "A" {
		t.Fatalf("read %q", b)
	}
}
