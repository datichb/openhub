package platform

import (
	"math"
	"testing"
)

func TestCacheSavings(t *testing.T) {
	saved, pct := AggregateStats{TotalTokensIn: 1000, CacheReadTokens: 400, TotalCost: 8}.CacheSavings()
	if math.Abs(saved-2.4) > 1e-9 || math.Abs(pct-30) > 1e-9 {
		t.Errorf("CacheSavings() = %v, %v; want 2.4, 30", saved, pct)
	}
	if saved, pct := (AggregateStats{TotalCost: 8, CacheReadTokens: 5}).CacheSavings(); saved != 0 || pct != 0 {
		t.Errorf("without input tokens: %v, %v", saved, pct)
	}
}
