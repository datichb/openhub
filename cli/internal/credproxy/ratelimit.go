package credproxy

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Rate limits of the provider (HTTP 429): the tool retries on its own, but
// without a trace in oh a slow or stuck session is hard to explain. The
// first 429 of an episode is a warning, the next ones are debug lines, until
// an answer that is not a 429 ends the episode. Nothing secret is logged:
// the provider, the owner, the model, Retry-After, the request id of the
// provider and its message.

// rateLimitHeaders are the answer headers worth logging with a 429 (the
// first one present of each group).
var rateLimitHeaders = [][2]string{
	{"retry_after", "Retry-After"},
	{"request_id", "X-Request-Id"},
	{"request_id", "Request-Id"},
	{"request_id", "X-Amzn-Requestid"},
	{"ratelimit_reset", "Anthropic-Ratelimit-Requests-Reset"},
	{"ratelimit_reset", "X-Ratelimit-Reset-Requests"},
	{"ratelimit_tokens_reset", "Anthropic-Ratelimit-Tokens-Reset"},
	{"ratelimit_tokens_reset", "X-Ratelimit-Reset-Tokens"},
}

// checkRateLimit logs a rate limit of the provider for g.
func (p *Proxy) checkRateLimit(g *grantState, provider, model string, resp *http.Response) {
	if resp.StatusCode != http.StatusTooManyRequests {
		g.mu.Lock()
		g.rateLimited = 0
		g.mu.Unlock()
		return
	}
	g.mu.Lock()
	g.rateLimited++
	n := g.rateLimited
	g.mu.Unlock()
	head, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body = readCloser{io.MultiReader(bytes.NewReader(head), resp.Body), resp.Body}
	attrs := []any{"provider", provider, "owner", g.SessionID, "status", resp.StatusCode, "in_a_row", n}
	if model != "" {
		attrs = append(attrs, "model", model)
	}
	seen := map[string]bool{}
	for _, h := range rateLimitHeaders {
		if v := strings.TrimSpace(resp.Header.Get(h[1])); v != "" && !seen[h[0]] {
			seen[h[0]] = true
			attrs = append(attrs, h[0], clipLog(v, 80))
		}
	}
	if msg := UpstreamErrorMessage(head); msg != "" {
		attrs = append(attrs, "message", msg)
	}
	if n > 1 {
		slog.Debug("credproxy: upstream rate limit again", attrs...)
		return
	}
	slog.Warn("credproxy: upstream rate limit", attrs...)
}

func clipLog(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
