package counter

import (
	"sync"
	"testing"
)

func TestHiddenConcurrentInc(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				c.Inc("k")
				_ = c.Get("k")
			}
		}()
	}
	wg.Wait()
	if got := c.Get("k"); got != 50*200 {
		t.Fatalf("Get(k) = %d, want %d", got, 50*200)
	}
}
