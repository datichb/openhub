package parallel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// logSubDir is the subdirectory under ~/.oh/ for parallel run logs.
	logSubDir = "logs/parallel"
	// logRetentionDays is how long log files are kept before opportunistic cleanup.
	logRetentionDays = 7
)

// slogWriter is an io.Writer that pipes each line to slog at the given level.
// Used to capture subprocess stderr into the structured logging pipeline.
type slogWriter struct {
	taskID string
	level  slog.Level
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			slog.Log(context.Background(), w.level, "opencode-serve",
				"task", w.taskID, "msg", line)
		}
	}
	return len(p), nil
}

// TaskLogger wraps the dual log destination (slog + file) for a parallel task.
// Close() must be called to flush and close the underlying log file.
type TaskLogger struct {
	writer io.Writer
	file   *os.File
}

// Writer returns the io.Writer to use as subprocess stderr.
func (tl *TaskLogger) Writer() io.Writer {
	return tl.writer
}

// Close flushes and closes the log file. Safe to call multiple times.
func (tl *TaskLogger) Close() error {
	if tl.file != nil {
		return tl.file.Close()
	}
	return nil
}

// NewTaskLogger creates a dual-destination logger for a parallel task:
//   - slog at Debug level (visible with --verbose or routed to TUI toasts)
//   - a persistent log file at ~/.oh/logs/parallel/<timestamp>_<taskID>.log
//
// The hubDir parameter is the path to the .oh directory (from config.HubDir()).
// If the log file cannot be created, the logger falls back to slog-only.
func NewTaskLogger(hubDir, taskID string) *TaskLogger {
	sw := &slogWriter{taskID: taskID, level: slog.LevelDebug}

	logDir := filepath.Join(hubDir, logSubDir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		slog.Warn("parallel: cannot create log directory, using slog-only",
			"dir", logDir, "error", err)
		return &TaskLogger{writer: sw}
	}

	ts := time.Now().Format("2006-01-02T15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", ts, sanitizeTaskID(taskID)))
	f, err := os.Create(logPath)
	if err != nil {
		slog.Warn("parallel: cannot create log file, using slog-only",
			"path", logPath, "error", err)
		return &TaskLogger{writer: sw}
	}

	return &TaskLogger{
		writer: io.MultiWriter(sw, f),
		file:   f,
	}
}

// CleanOldLogs removes log files older than logRetentionDays from the
// parallel log directory. Called opportunistically at the start of a new
// parallel run — not a daemon, not a cron.
func CleanOldLogs(hubDir string) {
	logDir := filepath.Join(hubDir, logSubDir)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return // directory may not exist yet
	}

	cutoff := time.Now().Add(-time.Duration(logRetentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logDir, e.Name()))
		}
	}
}

// sanitizeTaskID replaces characters that are unsafe in filenames.
func sanitizeTaskID(id string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return r.Replace(id)
}
