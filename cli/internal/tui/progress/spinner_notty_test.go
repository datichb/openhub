package progress

import (
	"io"
	"os"
	"testing"
)

// A14: no animation when the output is not a terminal (pipe, --no-tui).
func TestSpinnerSilentWithoutTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	s := NewSpinner("working")
	s.Start()
	running := s.running
	s.Stop()
	os.Stdout, os.Stderr = prevOut, prevErr
	w.Close()
	out, _ := io.ReadAll(r)
	if running || len(out) != 0 {
		t.Fatalf("spinner drawn without terminal: running=%v output=%q", running, out)
	}
}
