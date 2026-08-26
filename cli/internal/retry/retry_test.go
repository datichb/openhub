package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultConfig(), func(attempt int) (bool, error) {
		calls++
		return false, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Jitter:      0,
	}, func(attempt int) (bool, error) {
		calls++
		if attempt < 3 {
			return true, errors.New("transient")
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoExhausted(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Jitter:      0,
	}, func(attempt int) (bool, error) {
		calls++
		return true, errors.New("always fails")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := Do(ctx, Config{
		MaxAttempts: 5,
		BaseDelay:   time.Second,
		MaxDelay:    time.Second,
		Jitter:      0,
	}, func(attempt int) (bool, error) {
		if attempt == 1 {
			return true, errors.New("first fail")
		}
		t.Fatal("should not reach attempt 2")
		return false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoPermanentError(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}, func(attempt int) (bool, error) {
		calls++
		return false, errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on permanent), got %d", calls)
	}
}

func TestIsTransientHTTP(t *testing.T) {
	transient := []int{429, 500, 502, 503, 504}
	for _, code := range transient {
		if !IsTransientHTTP(code) {
			t.Errorf("expected %d to be transient", code)
		}
	}
	permanent := []int{200, 301, 400, 401, 403, 404}
	for _, code := range permanent {
		if IsTransientHTTP(code) {
			t.Errorf("expected %d to NOT be transient", code)
		}
	}
}
