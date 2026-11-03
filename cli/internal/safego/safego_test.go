package safego

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGo_NoPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	var ran bool
	Go(func() {
		defer wg.Done()
		ran = true
	})
	wg.Wait()
	if !ran {
		t.Fatal("function did not run")
	}
}

func TestGo_RecoversPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go(func() {
		defer wg.Done()
		panic("test panic")
	})
	// If Go does not recover, this test process crashes.
	wg.Wait()
}

func TestGo_RecoversPanicNilValue(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go(func() {
		defer wg.Done()
		var s *string
		_ = *s // nil pointer dereference
	})
	wg.Wait()
}

func TestSupervise_ReturnsWhenFnReturns(t *testing.T) {
	runs := 0
	SuperviseBackoff(context.Background(), "once", time.Millisecond, time.Millisecond, func(context.Context) { runs++ })
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

func TestSupervise_RestartsAfterPanic(t *testing.T) {
	runs := 0
	SuperviseBackoff(context.Background(), "flaky", time.Millisecond, time.Millisecond, func(context.Context) {
		runs++
		if runs < 3 {
			panic("boom")
		}
	})
	if runs != 3 {
		t.Fatalf("runs = %d, want 3 (two panics, then a normal return)", runs)
	}
}

func TestSupervise_StopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runs := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		// A pause that would never end: only the context stops the loop.
		SuperviseBackoff(ctx, "cancelled", time.Hour, time.Hour, func(context.Context) {
			runs++
			cancel()
			panic("boom")
		})
	}()
	<-done
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

func TestSupervise_LogsThePanic(t *testing.T) {
	var logs syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	n := 0
	SuperviseBackoff(context.Background(), "watcher g1", time.Millisecond, time.Millisecond, func(context.Context) {
		if n++; n == 1 {
			panic("event handler exploded")
		}
	})
	out := logs.String()
	for _, want := range []string{"recovered panic in supervised loop", "watcher g1", "event handler exploded", "goroutine"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestAfterFunc_RunsAndRecoversAPanic(t *testing.T) {
	done := make(chan struct{})
	AfterFunc(time.Millisecond, func() {
		defer close(done)
		panic("test panic")
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("function did not run")
	}
}

func TestAfterFunc_StopCancelsTheCall(t *testing.T) {
	ran := make(chan struct{}, 1)
	timer := AfterFunc(time.Hour, func() { ran <- struct{}{} })
	if !timer.Stop() {
		t.Fatal("the timer had already fired")
	}
	select {
	case <-ran:
		t.Fatal("a stopped timer ran")
	case <-time.After(20 * time.Millisecond):
	}
}

// captureLogs gives the default logger a buffer and the hub a folder of its own.
func captureLogs(t *testing.T) (*syncBuffer, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return logs, filepath.Join(home, "run")
}

func crashFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestGo_WritesACrashFileAndLogsItsPath(t *testing.T) {
	logs, dir := captureLogs(t)
	done := make(chan struct{})
	Go(func() {
		defer close(done)
		panic("goroutine exploded")
	})
	<-done
	var files []string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if files = crashFiles(t, dir); len(files) > 0 {
			break
		}
	}
	if len(files) != 1 {
		t.Fatalf("crash files = %v, want 1", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"panic: goroutine exploded", "TestGo_WritesACrashFileAndLogsItsPath"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("crash file lacks %q:\n%s", want, data)
		}
	}
	var out string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if out = logs.String(); strings.Contains(out, "recovered panic") {
			break
		}
	}
	if !strings.Contains(out, "crash_file="+files[0]) {
		t.Errorf("the log does not give the crash file %s:\n%s", files[0], out)
	}
}

func TestSupervise_WritesACrashFile(t *testing.T) {
	_, dir := captureLogs(t)
	n := 0
	SuperviseBackoff(context.Background(), "loop", time.Millisecond, time.Millisecond, func(context.Context) {
		if n++; n == 1 {
			panic("loop exploded")
		}
	})
	if files := crashFiles(t, dir); len(files) != 1 {
		t.Fatalf("crash files = %v, want 1", files)
	}
}

func TestWrapErr_ReturnsThePanicAsAnErrorWithItsStack(t *testing.T) {
	_, dir := captureLogs(t)
	cause := errors.New("sentinel")
	err := WrapErr(func() error { panic(cause) })()
	var perr *PanicError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %v, want a *PanicError", err)
	}
	if !errors.Is(err, cause) {
		t.Error("the error given to panic is not reachable with errors.Is")
	}
	if !strings.Contains(string(perr.Stack), "TestWrapErr_ReturnsThePanicAsAnErrorWithItsStack") {
		t.Errorf("stack lacks the panicking function:\n%s", perr.Stack)
	}
	files := crashFiles(t, dir)
	if len(files) != 1 || perr.CrashFile != files[0] {
		t.Errorf("CrashFile = %q, files = %v", perr.CrashFile, files)
	}
	if msg := err.Error(); !strings.Contains(msg, "sentinel") || !strings.Contains(msg, files[0]) || !strings.Contains(msg, "goroutine") {
		t.Errorf("message lacks the value, the file or the stack:\n%s", msg)
	}
}

func TestWrapErr_PassesTheErrorThrough(t *testing.T) {
	want := errors.New("plain")
	if got := WrapErr(func() error { return want })(); got != want {
		t.Errorf("err = %v, want %v", got, want)
	}
	if got := WrapErr(func() error { return nil })(); got != nil {
		t.Errorf("err = %v, want nil", got)
	}
}

func TestWrap_RecoversForAWaitGroup(t *testing.T) {
	captureLogs(t)
	var wg sync.WaitGroup
	wg.Go(Wrap(func() { panic("in a wait group") }))
	wg.Wait()
}
