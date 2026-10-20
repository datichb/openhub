package credproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The weights are the price ratios of the Anthropic and Bedrock lists.
func TestWeightedTokens(t *testing.T) {
	assert.InDelta(t, 10.0, WeightedTokens(10, 0, 0, 0), 1e-9, "uncached input ×1")
	assert.InDelta(t, 50.0, WeightedTokens(0, 10, 0, 0), 1e-9, "output ×5")
	assert.InDelta(t, 9.8, WeightedTokens(0, 0, 98, 0), 1e-9, "cache read ×0.1")
	assert.InDelta(t, 12.5, WeightedTokens(0, 0, 0, 10), 1e-9, "cache write ×1.25")
	assert.InDelta(t, 0.0, WeightedTokens(0, 0, 0, 0), 1e-9)

	// A pass of Opus 5.5 (EU) that read 100k tokens of cache, wrote 20k and
	// produced 3k of output cost about $0.20; at the input price of $5.5/M,
	// its 50k equivalents are $0.275, an upper bound.
	u := Usage{CacheReadTokens: 100_000, CacheWriteTokens: 20_000, OutputTokens: 3_000}
	assert.InDelta(t, 50_000, u.Weighted(), 1e-9)
	assert.Equal(t, int64(123_000), u.Tokens(), "raw count kept apart")
}
