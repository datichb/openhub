package daemon

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/logrotate"
)

// runLogRotation keeps the logs written by live processes bounded: ohd.log
// (the daemon's own output) and the *.log files of the data directories of
// the server groups (N16). It never keeps the daemon awake.
func (d *Daemon) runLogRotation(ctx context.Context) {
	every := d.opts.LogRotateEvery
	if every <= 0 {
		every = 10 * time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		d.rotateLogs()
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case <-t.C:
		}
	}
}

// rotateLogs copies then truncates the logs larger than the limit.
func (d *Daemon) rotateLogs() {
	maxBytes, keep := d.opts.LogMaxBytes, d.opts.LogKeep
	if maxBytes <= 0 {
		maxBytes = logrotate.DefaultMaxBytes
	}
	if keep <= 0 {
		keep = logrotate.DefaultKeep
	}
	files := []string{d.opts.Paths.Log()}
	if d.opts.ServersDir != "" {
		if m, err := filepath.Glob(filepath.Join(d.opts.ServersDir, "*", "data", "*.log")); err == nil {
			files = append(files, m...)
		}
	}
	for _, f := range files {
		if rotated, err := logrotate.CopyTruncate(f, maxBytes, keep); err != nil {
			slog.Warn("rotating a log", "path", f, "error", err)
		} else if rotated {
			slog.Info("log rotated", "path", f)
		}
	}
}

// runTask runs a task every Task.Every, the first time once the grants are
// restored.
func (d *Daemon) runTask(ctx context.Context, t Task) {
	every := t.Every
	if every <= 0 {
		every = time.Hour
	}
	select {
	case <-d.restored:
	case <-ctx.Done():
		return
	case <-d.stop:
		return
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		safeRun(ctx, t)
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case <-tick.C:
		}
	}
}

// safeRun runs a task; a panic is logged, the daemon goes on.
func safeRun(ctx context.Context, t Task) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("daemon task panicked", "task", t.Name, "panic", r)
		}
	}()
	t.Run(ctx)
}
