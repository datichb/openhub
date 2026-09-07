// Package httplog provides a logging HTTP transport middleware.
// It wraps an existing http.Client to add structured slog logging for
// every outgoing HTTP request and response, without changing the caller's API.
//
// Usage:
//
//	client := httplog.Wrap(&http.Client{Timeout: 10*time.Second}, "tracker.gitlab")
//	// All requests through this client are now logged:
//	//   slog.Debug("tracker.gitlab.request",  "method", "GET", "url", "https://...")
//	//   slog.Debug("tracker.gitlab.response", "method", "GET", "url", "https://...", "status", 200, "duration", "142ms")
//
// For webhook URLs that contain secrets in the path, use WithMaskURL():
//
//	client := httplog.Wrap(&http.Client{}, "notify.mattermost", httplog.WithMaskURL())
//	// URL is masked: "https://mm.example.com/hooks/****3456"
package httplog

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Option configures the logging transport behaviour.
type Option func(*options)

type options struct {
	maskURL bool
}

// WithMaskURL enables URL masking for requests containing secrets in the path
// (e.g., webhook URLs). The last path segment is truncated to "****" + last 4 chars.
func WithMaskURL() Option {
	return func(o *options) { o.maskURL = true }
}

// Wrap returns a shallow copy of the given http.Client with a logging transport
// injected. The original client's Timeout, Jar, and other settings are preserved.
// namespace is the slog key prefix (e.g., "tracker.gitlab", "mcp.figma").
func Wrap(client *http.Client, namespace string, opts ...Option) *http.Client {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	inner := client.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}

	cp := *client // shallow copy — preserves Timeout, Jar, etc.
	cp.Transport = &loggingTransport{
		inner:     inner,
		namespace: namespace,
		maskURL:   o.maskURL,
	}
	return &cp
}

// loggingTransport wraps an http.RoundTripper with structured logging.
type loggingTransport struct {
	inner     http.RoundTripper
	namespace string
	maskURL   bool
}

// RoundTrip implements http.RoundTripper.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	displayURL := req.URL.String()
	if t.maskURL {
		displayURL = MaskURL(displayURL)
	}

	slog.Debug(t.namespace+".request",
		"method", req.Method,
		"url", displayURL,
	)

	start := time.Now()
	resp, err := t.inner.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		slog.Warn(t.namespace+".request.failed",
			"method", req.Method,
			"url", displayURL,
			"error", err,
			"duration", elapsed,
		)
		return resp, err
	}

	slog.Debug(t.namespace+".response",
		"method", req.Method,
		"url", displayURL,
		"status", resp.StatusCode,
		"duration", elapsed,
	)

	return resp, nil
}

// MaskURL hides potential secrets in webhook-style URLs.
// The last path segment is replaced with "****" + last 4 chars.
// Example: "https://mm.example.com/hooks/abcdef123456" → "https://mm.example.com/hooks/****3456"
func MaskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	path := u.Path
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		segment := path[idx+1:]
		if len(segment) > 4 {
			masked := "****" + segment[len(segment)-4:]
			u.Path = path[:idx+1] + masked
		}
	}
	// Strip user info (credentials in URL)
	u.User = nil
	return u.String()
}
