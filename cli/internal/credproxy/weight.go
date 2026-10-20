package credproxy

// The token budget of a grant (Grant.MaxTokens) is counted in input-
// equivalent tokens: each kind of token weighs its price relative to an
// uncached input token. The ratios are those of the Anthropic and Bedrock
// price lists, the same for Opus, Sonnet and Haiku 5.x in every region
// (eu., us., global.): output 5×, cache read 0.1×, cache write 1.25×. A raw
// count would let the output through at a fifth of its cost, or, once the
// cache counts, stop a session on the cache reads that make most of its
// tokens and a tenth of its cost.
const (
	WeightInput      = 1.0
	WeightOutput     = 5.0
	WeightCacheRead  = 0.1
	WeightCacheWrite = 1.25
)

// WeightedTokens is the number of input-equivalent tokens of a usage: the
// only place the weights are applied.
func WeightedTokens(input, output, cacheRead, cacheWrite int64) float64 {
	return float64(input)*WeightInput + float64(output)*WeightOutput +
		float64(cacheRead)*WeightCacheRead + float64(cacheWrite)*WeightCacheWrite
}

// Weighted is the usage in input-equivalent tokens, which Grant.MaxTokens
// bounds. Tokens is the raw count.
func (u Usage) Weighted() float64 {
	return WeightedTokens(u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens)
}
