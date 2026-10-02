package safego

import (
	"sync"
	"testing"
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
