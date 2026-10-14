package store

import (
	"path/filepath"
	"testing"
)

func TestAddList(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "todo.txt"))
	if err := s.Add("pain"); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil || len(got) != 1 || got[0] != "pain" {
		t.Fatalf("List() = %v, %v ; attendu [pain]", got, err)
	}
}
