package limits

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRatesCostCountsTheCache(t *testing.T) {
	r := Rates{Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12}
	assert.InDelta(t, 10+50+1+12, r.Cost(1_000_000, 1_000_000, 1_000_000, 1_000_000), 1e-9)
	assert.InDelta(t, 0.0, r.Cost(0, 0, 0, 0), 1e-9)
	assert.Greater(t, ReservationRates.Output, ReservationRates.Input)
	assert.Less(t, ReservationRates.CacheRead, ReservationRates.Input, "reading the cache is cheaper than the input")
}

func TestReserveErrsHighWithinAShareOfTheAllowance(t *testing.T) {
	// 16k tokens of output at 75 $/M = 1.2 $: capped by a tenth of 5 $.
	assert.InDelta(t, 0.5, Reserve(1000, 16000, 5), 1e-9)
	// Small request: the estimate itself (not capped), above the real price of a cheap model.
	got := Reserve(1000, 100, 100)
	assert.InDelta(t, (1000*15+100*75)/1e6, got, 1e-12)
	// No allowance known: no cap.
	assert.InDelta(t, 1.2+0.015, Reserve(1000, 16000, 0), 1e-9)
}
