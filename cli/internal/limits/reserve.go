package limits

// Reservation of the dollar budgets (F-04, S-27). The cost of a session is
// what the tool reports (it knows the prices of its models, cache included),
// but it arrives after the requests. To keep concurrent requests from all
// being admitted on the same remaining budget, the daemon reserves an
// amount for each one while it runs. oh has no price list: the amount comes
// from rates deliberately above the usual prices, so that it errs high.

// Rates are prices in USD per million tokens.
type Rates struct {
	Input, Output, CacheRead, CacheWrite float64
}

// ReservationRates are the rates of the reservations: those of the dearest
// models, with the cache priced as Anthropic does (read at a tenth of the
// input, write at 1.25 times).
var ReservationRates = Rates{Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75}

// Cost is the price of the tokens at the rates.
func (r Rates) Cost(input, output, cacheRead, cacheWrite int64) float64 {
	return (float64(input)*r.Input + float64(output)*r.Output +
		float64(cacheRead)*r.CacheRead + float64(cacheWrite)*r.CacheWrite) / 1e6
}

// ReservationShare bounds one reservation to this share of the allowance, so
// that a request with a large output cap does not hold the whole budget.
const ReservationShare = 0.1

// Reserve is the amount held for a request: its input and the output it
// allowed itself, at the reservation rates, within a share of the allowance.
func Reserve(inputTokens, maxOutputTokens int64, allowance float64) float64 {
	est := ReservationRates.Cost(inputTokens, maxOutputTokens, 0, 0)
	if allowance > 0 {
		est = min(est, allowance*ReservationShare)
	}
	return est
}
