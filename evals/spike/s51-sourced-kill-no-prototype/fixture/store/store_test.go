package store

import (
	"os"
	"testing"
)

func TestSave(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := Save([]byte("[]")); err != nil {
		t.Fatal(err)
	}
}
