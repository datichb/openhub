package credproxy

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// Request headers and query parameters relayed to the provider (audit v5,
// S-28). The tool is an agent that may be misled: relaying everything it
// sends would let it pick another account or project (OpenAI-Organization,
// OpenAI-Project), pass the keys of other providers (X-Provider-API-Keys),
// opt into betas, tiers or guardrails that change the price, or reach options
// the provider reads in the query. Only the headers the real clients send are
// kept; the others are dropped, and logged by name (never by value) at debug
// level. The credentials of the tool (strippedHeaders) are dropped first and
// replaced by the real ones.
//
// The lists come from what opencode sends to each provider (its own Messages,
// Converse, Chat Completions and Responses clients, and the bundled
// Anthropic SDK). To relay one more header, add it to the provider here and
// to TestRelayHeadersOfEachProvider: a header that changes who pays, where
// the request runs or which model answers does not belong here.

// commonRelayHeaders are relayed on every provider. Accept-Encoding is not
// among them: it is dropped (strippedHeaders) so that the transport
// negotiates compression and the usage accounting reads a decoded body.
var commonRelayHeaders = []string{"Content-Type", "Accept", "User-Agent"}

// providerRelayHeaders are the headers relayed on one provider only.
var providerRelayHeaders = map[string][]string{
	// anthropic-beta is chosen by the tool for the features of its model
	// (interleaved thinking, 1M context…); the X-Stainless-* ones are the
	// metadata of the Anthropic SDK.
	ProviderAnthropic: {
		"Anthropic-Version", "Anthropic-Beta",
		"X-Stainless-Lang", "X-Stainless-Package-Version", "X-Stainless-OS", "X-Stainless-Arch",
		"X-Stainless-Runtime", "X-Stainless-Runtime-Version", "X-Stainless-Retry-Count",
		"X-Stainless-Timeout", "X-Stainless-Helper-Method", "X-Stainless-Helper",
	},
	// Only the response format of InvokeModel. The signing headers (x-amz-date,
	// x-amz-security-token…) are the tool's and are replaced by the proxy's
	// own signature; the trace, guardrail, performance and service tier
	// headers change the cost or the checks of a request and stay with oh.
	ProviderBedrock: {"X-Amzn-Bedrock-Accept"},
	// The session headers let the provider keep the cache of a session on
	// the same backend. OpenAI-Organization, OpenAI-Project and OpenAI-Beta
	// pick the account, the project and beta features: never relayed.
	ProviderOpenAI: {"X-Session-Id", "X-Session-Affinity"},
	// HTTP-Referer and X-Title are the attribution of the application.
	// X-Provider-API-Keys carries other keys (bring your own key): never.
	ProviderOpenRouter: {
		"HTTP-Referer", "X-Title", "X-OpenRouter-Title", "X-OpenRouter-Categories",
		"X-Session-Id", "X-Session-Affinity",
	},
}

// RelayHeaders returns the request headers relayed to a provider, in their
// canonical form (sorted): the allow-list of the proxy.
func RelayHeaders(provider string) []string {
	out := slices.Concat(commonRelayHeaders, providerRelayHeaders[provider])
	for i, h := range out {
		out[i] = http.CanonicalHeaderKey(h)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// relaySets are the allow-lists by provider, as sets of canonical names.
var relaySets = func() map[string]map[string]bool {
	sets := map[string]map[string]bool{}
	for p := range providerRelayHeaders {
		sets[p] = map[string]bool{}
		for _, h := range RelayHeaders(p) {
			sets[p][h] = true
		}
	}
	return sets
}()

// maxLoggedDropped bounds the names listed by one debug line, and
// maxDroppedName the length of one name: a tool may send many, long ones.
const (
	maxLoggedDropped = 20
	maxDroppedName   = 64
)

// filterRelay removes from the outgoing request the headers the provider does
// not take and keeps, in its query, the parameters of the route and nothing
// else. It returns the names dropped (headers, query parameters).
func filterRelay(provider string, rt route, out *http.Request, inRawQuery string) (headers, params []string) {
	out.Header.Del("Content-Length") // the transport sets it from the body it sends
	allowed := relaySets[provider]   // none for an unknown provider: nothing is relayed
	for name := range out.Header {
		if !allowed[http.CanonicalHeaderKey(name)] {
			out.Header.Del(name)
			headers = append(headers, name)
		}
	}
	out.URL.RawQuery, params = filterQuery(rt.query, inRawQuery)
	slices.Sort(headers)
	slices.Sort(params)
	return headers, params
}

// filterQuery keeps the parameters of a raw query that the route takes, with
// an accepted value, as they were written; a parameter repeated is kept once,
// since the proxy and the provider could read different ones.
func filterQuery(allowed map[string][]string, raw string) (kept string, dropped []string) {
	var pairs []string
	seen := map[string]bool{}
	for pair := range strings.SplitSeq(raw, "&") {
		if pair == "" {
			continue
		}
		rawKey, rawVal, _ := strings.Cut(pair, "=")
		key, err1 := url.QueryUnescape(rawKey)
		val, err2 := url.QueryUnescape(rawVal)
		values, known := allowed[key]
		switch {
		case err1 != nil || err2 != nil || !known || seen[key]:
			dropped = append(dropped, rawKey)
		case len(values) > 0 && !slices.Contains(values, val):
			dropped = append(dropped, rawKey)
		default:
			seen[key] = true
			pairs = append(pairs, pair)
		}
	}
	return strings.Join(pairs, "&"), dropped
}

// logDropped journals, at debug level, what filterRelay removed: names only,
// never values.
func logDropped(provider string, headers, params []string) {
	if len(headers) == 0 && len(params) == 0 {
		return
	}
	slog.Debug("credproxy: request headers and parameters not relayed",
		"provider", provider, "headers", cap20(headers), "query", cap20(params))
}

func cap20(names []string) []string {
	out := make([]string, 0, min(len(names), maxLoggedDropped+1))
	for i, n := range names {
		if i == maxLoggedDropped {
			out = append(out, "…")
			break
		}
		if len(n) > maxDroppedName {
			n = n[:maxDroppedName] + "…"
		}
		out = append(out, n)
	}
	return out
}
