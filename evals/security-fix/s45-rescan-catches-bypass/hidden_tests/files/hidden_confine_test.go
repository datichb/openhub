package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHiddenOpenStaysUnderRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "uploads")
	for _, d := range []string{filepath.Join(root, "client-1"), filepath.Join(base, "uploads-private"), filepath.Join(base, "secrets")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string) {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "client-1", "facture.pdf"))
	write(filepath.Join(base, "uploads-private", "cle.txt"))
	write(filepath.Join(base, "secrets", "db.env"))
	if err := os.Symlink(filepath.Join(base, "secrets"), filepath.Join(root, "import")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../secrets/db.env", "client-1/../../secrets/db.env", "../uploads-private/cle.txt", "import/db.env"} {
		if f, err := Open(root, name); err == nil {
			f.Close()
			t.Errorf("Open(%q) opened a file outside the root", name)
		}
	}
	for _, name := range []string{"client-1/facture.pdf", "client-1/../client-1/facture.pdf"} {
		f, err := Open(root, name)
		if err != nil {
			t.Errorf("Open(%q) = %v, want the file", name, err)
			continue
		}
		f.Close()
	}
}
