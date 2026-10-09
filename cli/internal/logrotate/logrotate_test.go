package logrotate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), n), 0o600); err != nil {
		t.Fatal(err)
	}
}

func size(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return fi.Size()
}

func TestRotateKeepsTheLastCopies(t *testing.T) {
	log := filepath.Join(t.TempDir(), "ohd.log")
	if ok, err := Rotate(log, 10, 3); ok || err != nil {
		t.Fatalf("missing file: %v %v", ok, err)
	}
	write(t, log, 5)
	if ok, _ := Rotate(log, 10, 3); ok {
		t.Fatal("small file rotated")
	}
	for i := 1; i <= 5; i++ {
		write(t, log, 10+i)
		ok, err := Rotate(log, 10, 3)
		if !ok || err != nil {
			t.Fatalf("round %d: %v %v", i, ok, err)
		}
	}
	if size(t, log) != -1 {
		t.Fatal("the log stays after a rename")
	}
	for n, want := range map[int]int64{1: 15, 2: 14, 3: 13} {
		if got := size(t, log+"."+string(rune('0'+n))); got != want {
			t.Errorf(".%d: size %d, want %d", n, got, want)
		}
	}
	if size(t, log+".4") != -1 {
		t.Error("more copies than keep")
	}
}

// A live process writes with O_APPEND: after the truncation its writes go
// at the new end (no hole), the copy holds what was there.
func TestCopyTruncateWithAnAppendWriter(t *testing.T) {
	log := filepath.Join(t.TempDir(), "oh-server.log")
	w, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.WriteString(strings.Repeat("before\n", 10)); err != nil {
		t.Fatal(err)
	}
	ok, err := CopyTruncate(log, 20, 2)
	if !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if _, err := w.WriteString("after\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(log)
	if string(got) != "after\n" {
		t.Fatalf("log = %q", got)
	}
	old, _ := os.ReadFile(log + ".1")
	if string(old) != strings.Repeat("before\n", 10) {
		t.Fatalf("copy = %q", old)
	}
	if fi, _ := os.Stat(log + ".1"); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
}

func TestCopyTruncateWithoutCopies(t *testing.T) {
	log := filepath.Join(t.TempDir(), "x.log")
	write(t, log, 100)
	if ok, err := CopyTruncate(log, 10, 0); !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if size(t, log) != 0 || size(t, log+".1") != -1 {
		t.Fatal("keep 0: truncated, no copy")
	}
}
