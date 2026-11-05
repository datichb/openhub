package shell

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/filelock"
)

// readLines returns the lines of the history file as the notifications they
// hold; a line that is not a whole JSON object fails the test.
func readLines(t *testing.T, path string) []persistedNotif {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var out []persistedNotif
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var pn persistedNotif
		require.NoError(t, json.Unmarshal(sc.Bytes(), &pn), "a whole line, not cut: %q", sc.Text())
		out = append(out, pn)
	}
	require.NoError(t, sc.Err())
	return out
}

// Writers append while the file is rotated again and again: no line is lost
// and none is cut, whatever the interleaving.
func TestNotificationFileRotationWhileWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.jsonl")
	const writers, perWriter = 8, 120

	var wg sync.WaitGroup
	var failures atomic.Int32
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				if err := AppendToFile(path, ToastInfo, fmt.Sprintf("w%d-%03d", w, i)); err != nil {
					failures.Add(1)
				}
			}
		}()
	}
	stop := make(chan struct{})
	var rotations atomic.Int32
	rotated := make(chan struct{})
	go func() {
		defer close(rotated)
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Nothing is old or beyond the cap: the rotation rewrites the
			// file as it is, it must not drop a line that arrived meanwhile.
			if err := RotateFile(path, 100_000, 24*time.Hour); err != nil {
				failures.Add(1)
			}
			rotations.Add(1)
		}
	}()
	wg.Wait()
	close(stop)
	<-rotated

	require.Zero(t, failures.Load(), "no write nor rotation failed")
	require.Positive(t, rotations.Load(), "the file was rotated meanwhile")
	got := map[string]int{}
	for _, pn := range readLines(t, path) {
		got[pn.Message]++
	}
	assert.Len(t, got, writers*perWriter, "every line is there")
	for msg, n := range got {
		assert.Equal(t, 1, n, "%s written once", msg)
	}
}

// The rotation that really drops lines (the cap) still keeps the lines that
// are appended meanwhile: the file ends with the newest ones, in order.
func TestNotificationFileRotationCapKeepsTheNewest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.jsonl")
	const total, keep = 400, 50
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range total {
			_ = AppendToFile(path, ToastInfo, fmt.Sprintf("n%03d", i))
		}
	}()
	for finished := false; !finished; {
		select {
		case <-done:
			finished = true
		default:
			require.NoError(t, RotateFile(path, keep, 24*time.Hour))
		}
	}
	require.NoError(t, RotateFile(path, keep, 24*time.Hour))
	lines := readLines(t, path)
	require.Len(t, lines, keep)
	for i, pn := range lines {
		assert.Equal(t, fmt.Sprintf("n%03d", total-keep+i), pn.Message)
	}
}

// Another process (a second TUI, the daemon) holds the lock of the file: a
// write waits for it, and takes it only once released. The lock is the one of
// the filelock package, on a file next to the history.
func TestNotificationFileWritersWaitForTheLockOfAnotherProcess(t *testing.T) {
	for name, write := range map[string]func(path string) error{
		"append": func(path string) error { return AppendToFile(path, ToastInfo, "late") },
		"rotate": func(path string) error { return RotateFile(path, 10, time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notifications.jsonl")
			require.NoError(t, AppendToFile(path, ToastInfo, "first"))
			release, err := filelock.Lock(path + ".lock")
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() { done <- write(path) }()
			select {
			case err := <-done:
				release()
				t.Fatalf("%s did not wait for the lock (err=%v)", name, err)
			case <-time.After(300 * time.Millisecond):
			}
			release()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("the write never got the lock back")
			}
			assert.NotEmpty(t, readLines(t, path))
		})
	}
}

// A holder that never lets go does not block the writers for ever: they give
// up with an error after notificationsLockWait.
func TestNotificationFileLockWaitIsBounded(t *testing.T) {
	prev := notificationsLockWait
	notificationsLockWait = 150 * time.Millisecond
	t.Cleanup(func() { notificationsLockWait = prev })

	path := filepath.Join(t.TempDir(), "notifications.jsonl")
	require.NoError(t, AppendToFile(path, ToastInfo, "first"))
	release, err := filelock.Lock(path + ".lock")
	require.NoError(t, err)
	defer release()

	start := time.Now()
	require.Error(t, AppendToFile(path, ToastInfo, "never"))
	require.Error(t, RotateFile(path, 10, time.Hour))
	assert.Less(t, time.Since(start), 5*time.Second)

	release()
	require.NoError(t, AppendToFile(path, ToastInfo, "after"), "the in-process lock was given back too")
	assert.Len(t, readLines(t, path), 2)
}
