package parallel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlogWriter_WritesLines(t *testing.T) {
	sw := &slogWriter{taskID: "BD-42", level: 0}
	n, err := sw.Write([]byte("line one\nline two\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 18 {
		t.Errorf("expected n=18, got %d", n)
	}
}

func TestSlogWriter_EmptyInput(t *testing.T) {
	sw := &slogWriter{taskID: "BD-42", level: 0}
	n, err := sw.Write([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected n=0, got %d", n)
	}
}

func TestNewTaskLogger_CreatesFile(t *testing.T) {
	hubDir := t.TempDir()
	tl := NewTaskLogger(hubDir, "BD-42")
	defer tl.Close()

	if tl.file == nil {
		t.Fatal("expected log file to be created")
	}
	if tl.writer == nil {
		t.Fatal("expected writer to be set")
	}

	// Write something
	_, err := tl.Writer().Write([]byte("test log line\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Verify file was created in the right place
	logDir := filepath.Join(hubDir, logSubDir)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("cannot read log dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	if got := entries[0].Name(); got == "" {
		t.Fatal("expected non-empty filename")
	}
}

func TestNewTaskLogger_FallbackOnBadDir(t *testing.T) {
	// Use a path that cannot be created (file as parent)
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	tl := NewTaskLogger(tmpFile, "BD-42")
	defer tl.Close()

	// Should fall back to slog-only (no file)
	if tl.file != nil {
		t.Error("expected nil file on fallback")
	}
	// Writer should still work (slog only)
	if tl.writer == nil {
		t.Fatal("expected writer even on fallback")
	}
}

func TestCleanOldLogs(t *testing.T) {
	hubDir := t.TempDir()
	logDir := filepath.Join(hubDir, logSubDir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an old log file (10 days old)
	oldFile := filepath.Join(logDir, "old.log")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a recent log file
	recentFile := filepath.Join(logDir, "recent.log")
	if err := os.WriteFile(recentFile, []byte("recent"), 0644); err != nil {
		t.Fatal(err)
	}

	CleanOldLogs(hubDir)

	// Old file should be removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be removed")
	}
	// Recent file should remain
	if _, err := os.Stat(recentFile); err != nil {
		t.Error("expected recent file to remain")
	}
}

func TestSanitizeTaskID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"BD-42", "BD-42"},
		{"feat/my-branch", "feat_my-branch"},
		{"task:1", "task_1"},
		{"with spaces", "with_spaces"},
	}
	for _, tt := range tests {
		got := sanitizeTaskID(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeTaskID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
