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

func TestCutCostPricesTheTokensAtTheReservationRates(t *testing.T) {
	// 1000 input, 2000 output, 3000 cache read, 400 cache write.
	want := (1000*15 + 2000*75 + 3000*1.5 + 400*18.75) / 1e6
	assert.InDelta(t, want, CutCost(1000, 2000, 3000, 400, 0), 1e-12, "no allowance: not bounded")
	assert.InDelta(t, want, CutCost(1000, 2000, 3000, 400, 100), 1e-12, "within a tenth of the allowance")
	assert.InDelta(t, 0.003, CutCost(1000, 2000, 3000, 400, 0.03), 1e-12, "bounded by a tenth of the allowance, like a reservation")
	assert.Zero(t, CutCost(0, 0, 0, 0, 5))
	assert.Greater(t, CutCost(1000, 0, 0, 0, 0), 0.0)
}
