package teamstate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/filelock"
)

// Write lock of a team-state clone (N15): every git operation that changes
// the clone runs under
//   - an in-process lock per clone path, shared by every Repo of the path
//     (a process may hold several Repo values on the same clone), and
//   - the file lock .git/oh.lock, shared with the other oh processes (CLI,
//     TUI, daemon, MCP servers).
//
// Interactive commands wait up to 30 s, background writers (daemon, tracker
// synchronization) longer (WithLockWait), the periodic synchronizations of
// the TUI do not wait (WithTryLock): they skip a turn.

// ErrBusy is returned when the team-state clone stays locked by another
// operation for longer than the caller waits.
var ErrBusy = errors.New("team-state busy")

// BusyError names the holder of the lock, when it is known.
type BusyError struct {
	Path   string
	Holder string // "pid 123 (oh team sync) since 10:02:03", or git
}

func (e *BusyError) Error() string {
	if e.Holder == "" {
		return fmt.Sprintf("%v: %s", ErrBusy, e.Path)
	}
	return fmt.Sprintf("%v: %s (held by %s)", ErrBusy, e.Path, e.Holder)
}

func (e *BusyError) Unwrap() error { return ErrBusy }

// Waits of the lock.
const (
	// DefaultLockWait is the wait of an interactive command.
	DefaultLockWait = 30 * time.Second
	// BackgroundLockWait is the wait of the daemon and of the tracker
	// synchronization.
	BackgroundLockWait = 2 * time.Minute
	// staleIndexLock is the age after which a .git/index.lock left by a
	// killed git is removed (under our lock).
	staleIndexLock = 2 * time.Minute
)

type lockWaitKey struct{}

// WithLockWait sets how long the team-state operations of ctx wait for the
// lock of a clone.
func WithLockWait(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, lockWaitKey{}, d)
}

// WithTryLock makes the team-state operations of ctx give up at once when a
// clone is locked (ErrBusy).
func WithTryLock(ctx context.Context) context.Context { return WithLockWait(ctx, 0) }

func lockWait(ctx context.Context) time.Duration {
	if d, ok := ctx.Value(lockWaitKey{}).(time.Duration); ok {
		return d
	}
	return DefaultLockWait
}

// pathLocks holds the in-process lock of each clone (canonical path).
var pathLocks sync.Map // string -> chan struct{} (capacity 1)

func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	// Not created yet (clone): resolve the parent.
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(resolved, filepath.Base(abs))
	}
	return abs
}

func pathSem(p string) chan struct{} {
	v, _ := pathLocks.LoadOrStore(canonicalPath(p), make(chan struct{}, 1))
	return v.(chan struct{})
}

// lockFile is .git/oh.lock, or a file next to the clone while it does not
// exist yet (clone, init).
func (r *Repo) lockFile() string {
	if info, err := os.Stat(filepath.Join(r.path, ".git")); err == nil && info.IsDir() {
		return filepath.Join(r.path, ".git", "oh.lock")
	}
	return filepath.Join(filepath.Dir(r.path), "."+filepath.Base(r.path)+".oh.lock")
}

// lockGit takes the write lock of the clone and returns its release. The
// operations of the process queue up for the in-process lock (bounded by
// ctx only, except WithTryLock); the wait applies to the file lock, held by
// another process.
func (r *Repo) lockGit(ctx context.Context) (func(), error) {
	wait := lockWait(ctx)
	sem := pathSem(r.path)
	select {
	case sem <- struct{}{}:
	default:
		if wait <= 0 {
			return nil, &BusyError{Path: r.path, Holder: holderOf(r.lockFile())}
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	deadline := time.Now().Add(wait)
	releaseSem := func() { <-sem }

	file := r.lockFile()
	var unlock func()
	var err error
	if remaining := time.Until(deadline); remaining <= 0 {
		unlock, err = filelock.TryLock(file)
	} else {
		lctx, cancel := context.WithTimeout(ctx, remaining)
		unlock, err = filelock.LockContext(lctx, file)
		cancel()
		if err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			err = filelock.ErrLocked
		}
	}
	if err != nil {
		releaseSem()
		if errors.Is(err, filelock.ErrLocked) {
			return nil, &BusyError{Path: r.path, Holder: holderOf(file)}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("locking the team-state %s: %w", r.path, err)
	}
	writeHolder(file)
	release := func() {
		_ = os.Remove(file + ".info")
		unlock()
		releaseSem()
	}
	if err := r.clearIndexLock(ctx, deadline); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// writeHolder records who holds the lock, for the "busy" message (a separate
// file: the lock itself may not be written by another handle on Windows).
func writeHolder(file string) {
	cmd := filepath.Base(os.Args[0])
	if len(os.Args) > 1 {
		cmd += " " + strings.Join(os.Args[1:min(len(os.Args), 3)], " ")
	}
	info := fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), cmd, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(file+".info", []byte(info), 0o600)
}

func holderOf(file string) string {
	data, err := os.ReadFile(file + ".info")
	if err != nil {
		return ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(parts) < 3 {
		return ""
	}
	pid, _ := strconv.Atoi(parts[0])
	since := parts[2]
	if t, err := time.Parse(time.RFC3339, parts[2]); err == nil {
		since = t.Format("15:04:05")
	}
	return fmt.Sprintf("pid %d (%s) since %s", pid, parts[1], since)
}

// clearIndexLock handles a .git/index.lock: one older than staleIndexLock
// was left by a killed git and is removed; a recent one is a git running in
// the clone (outside oh): waited for until the deadline.
func (r *Repo) clearIndexLock(ctx context.Context, deadline time.Time) error {
	p := filepath.Join(r.path, ".git", "index.lock")
	for {
		info, err := os.Stat(p)
		if err != nil {
			return nil //nolint:nilerr // no index lock: nothing to wait for
		}
		if age := time.Since(info.ModTime()); age > staleIndexLock {
			slog.Warn("teamstate: removing a stale git index lock", "path", p, "age", age.Round(time.Second))
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if time.Now().After(deadline) {
			return &BusyError{Path: r.path, Holder: "git (.git/index.lock)"}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
