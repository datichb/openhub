package credproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Budget reservation (F-04, S-27). Concurrent requests (a step may delegate
// to a dozen subagents) are all admitted before any of them is counted. So
// before a model request is relayed, an amount that errs high is reserved
// for it: the token budget of the grant and the Gate of the daemon (dollar
// budgets) refuse it when the spent and the reserved amounts together pass
// the budget. The reservation is released when the request is over, and the
// real usage is accounted instead.
//
// A request alone is never refused for its estimate: a budget stays a soft
// cap, and a request is only turned away when the budget is already spent,
// or when requests in flight hold what is left.

const (
	// DefaultMaxOutputTokens is the output assumed for a request that names
	// none (OpenAI-style APIs default to the model maximum).
	DefaultMaxOutputTokens = 16384
	// MaxReservedOutputTokens bounds the output of an estimate.
	MaxReservedOutputTokens = 128000
	// reserveRetry is the delay announced to a request refused for the
	// reservations of the others (Retry-After).
	reserveRetry = 2 * time.Second
)

// RetryAfterHeader is set on the momentary refusals (429).
const RetryAfterHeader = "Retry-After"

// modelCall reports whether a route generates text (it costs tokens):
// not the model lists, not the token counters.
func modelCall(provider, method, escapedRest string) bool {
	if method != http.MethodPost {
		return false
	}
	switch provider {
	case ProviderBedrock:
		for _, op := range []string{"/converse", "/converse-stream", "/invoke", "/invoke-with-response-stream"} {
			if strings.HasSuffix(escapedRest, op) {
				return true
			}
		}
	case ProviderAnthropic:
		return escapedRest == "messages"
	case ProviderOpenAI, ProviderOpenRouter:
		return escapedRest == "chat/completions" || escapedRest == "completions" || escapedRest == "responses"
	}
	return false
}

// isEventStream reports whether a response is an event stream (SSE, AWS
// event stream): it shows its own end.
func isEventStream(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "eventstream")
}

// maxOutputTokens reads the output cap of a request body (0 = none): the
// Anthropic and OpenAI "max_tokens", "max_completion_tokens" and
// "max_output_tokens", and the Bedrock "inferenceConfig.maxTokens".
func maxOutputTokens(body []byte) int64 {
	var b struct {
		MaxTokens           int64 `json:"max_tokens"`
		MaxCompletionTokens int64 `json:"max_completion_tokens"`
		MaxOutputTokens     int64 `json:"max_output_tokens"`
		InferenceConfig     struct {
			MaxTokens int64 `json:"maxTokens"`
		} `json:"inferenceConfig"`
	}
	if json.Unmarshal(body, &b) != nil {
		return 0
	}
	return min(max(b.MaxTokens, b.MaxCompletionTokens, b.MaxOutputTokens, b.InferenceConfig.MaxTokens), MaxReservedOutputTokens)
}

// admission estimates a request: a token is at least three bytes of body.
func admission(model string, body []byte) Admission {
	out := maxOutputTokens(body)
	if out <= 0 {
		out = DefaultMaxOutputTokens
	}
	return Admission{Model: model, InputTokens: int64(len(body))/3 + 1, MaxOutputTokens: out}
}

// admit reserves the estimate of a request on the token budget of the grant
// and asks the Gate. The returned function releases the reservations (once).
func (p *Proxy) admit(ctx context.Context, g *grantState, a Admission) (func(Usage), error) {
	est := WeightedTokens(a.InputTokens, a.MaxOutputTokens, 0, 0)
	if g.MaxTokens > 0 {
		g.mu.Lock()
		if used := g.usage.Weighted(); g.reserved > 0 && used+g.reserved+est > float64(g.MaxTokens) {
			g.mu.Unlock()
			return nil, &Refusal{Message: "credproxy: session token budget reserved by the requests in flight", RetryAfter: reserveRetry}
		}
		g.reserved += est
		g.mu.Unlock()
	}
	unreserve := func() {
		if g.MaxTokens > 0 {
			g.mu.Lock()
			g.reserved = max(0, g.reserved-est)
			g.mu.Unlock()
		}
	}
	p.mu.RLock()
	gate := p.Gate
	p.mu.RUnlock()
	gateRelease := func(Usage) {}
	if gate != nil {
		rel, err := gate(ctx, g.SessionID, a)
		if err != nil {
			unreserve()
			return nil, err
		}
		if rel != nil {
			gateRelease = rel
		}
	}
	var once sync.Once
	return func(u Usage) {
		once.Do(func() {
			unreserve()
			gateRelease(u)
		})
	}, nil
}

// refuse answers a refused request.
func refuse(w http.ResponseWriter, err error) {
	var r *Refusal
	if !errors.As(err, &r) {
		http.Error(w, "credproxy: request not admitted", http.StatusServiceUnavailable)
		return
	}
	if r.RetryAfter > 0 {
		w.Header().Set(RetryAfterHeader, strconv.Itoa(int((r.RetryAfter+time.Second-1)/time.Second)))
		http.Error(w, r.Message, http.StatusTooManyRequests)
		return
	}
	http.Error(w, r.Message, http.StatusPaymentRequired)
}
