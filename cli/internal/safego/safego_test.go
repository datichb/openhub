package safego

import (
	"bytes"
	"context"
	"log/slog"
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
