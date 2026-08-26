// Package retry provides a generic retry utility with exponential backoff and jitter.
package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Config controls the retry behavior.
type Config struct {
	// MaxAttempts is the total number of attempts (including the first).
	// Default: 3.
	MaxAttempts int

	// BaseDelay is the initial delay before the first retry.
	// Default: 1s.
	BaseDelay time.Duration

	// MaxDelay caps the delay between retries.
	// Default: 30s.
	MaxDelay time.Duration

	// Jitter adds randomness to the delay (0.0 = none, 1.0 = full random up to computed delay).
	// Default: 0.2 (20%).
	Jitter float64
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}
}

// Do executes fn with exponential backoff.
//
// fn receives the current attempt number (1-based) and returns:
//   - retry: true if the operation should be retried on failure
//   - err: the error (nil means success, stops retrying)
//
// Do respects context cancellation: if ctx is cancelled during a sleep,
// it returns ctx.Err() immediately.
func Do(ctx context.Context, cfg Config, fn func(attempt int) (retry bool, err error)) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		shouldRetry, err := fn(attempt)
		if err == nil {
			return nil
		}
		lastErr = err

		if !shouldRetry || attempt == cfg.MaxAttempts {
			break
		}

		// Compute exponential delay: base * 2^(attempt-1)
		delay := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		// Apply jitter
		if cfg.Jitter > 0 {
			jitterRange := float64(delay) * cfg.Jitter
			delay += time.Duration(rand.Float64()*jitterRange - jitterRange/2)
			if delay < 0 {
				delay = cfg.BaseDelay
			}
		}

		// Sleep with context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// IsTransientHTTP returns true for HTTP status codes that are typically transient
// and worth retrying (429, 500, 502, 503, 504).
func IsTransientHTTP(statusCode int) bool {
	switch statusCode {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}
