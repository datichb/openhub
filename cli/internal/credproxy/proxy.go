// Package credproxy is the oh credential proxy: the agentic tool talks to the
// LLM provider through it with a per-session token, and the proxy swaps that
// token for the real credential (kept by oh, never exposed to the tool or the
// agent's shell). It also enforces per-session model allow-lists and token
// budgets, and counts usage.
package credproxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Auth applies the real credential to an upstream request.
type Auth interface {
	Apply(req *http.Request, body []byte) error
}

// BearerAuth sets "Authorization: Bearer <token>" (Bedrock API keys, OpenRouter, OpenAI).
type BearerAuth struct{ Token string }

// Apply implements Auth.
func (a BearerAuth) Apply(req *http.Request, _ []byte) error {
	req.Header.Set("Authorization", "Bearer "+a.Token)
	return nil
}

// HeaderAuth sets a custom header (Anthropic "x-api-key").
type HeaderAuth struct{ Name, Value string }

// Apply implements Auth.
func (a HeaderAuth) Apply(req *http.Request, _ []byte) error {
	req.Header.Set(a.Name, a.Value)
	return nil
}

// Upstream is the real provider endpoint.
type Upstream struct {
	BaseURL string // e.g. https://bedrock-runtime.eu-west-1.amazonaws.com
	Auth    Auth
}

// Grant authorizes one session to use one provider through the proxy.
type Grant struct {
	SessionID     string
	Provider      string // opencode provider id: amazon-bedrock | anthropic | openrouter | openai
	Upstream      Upstream
	AllowedModels []string // wildcard patterns on the model id; empty = any
	MaxTokens     int64    // input+output token budget; 0 = unlimited
}

// Usage is the traffic accounted to a grant.
type Usage struct {
	Requests     int64
	InputTokens  int64
	OutputTokens int64
}

type grantState struct {
	Grant
	token string
	mu    sync.Mutex
	usage Usage
}

// Proxy is a local HTTP proxy. Routes: /<provider>/... → upstream /... .
type Proxy struct {
	mu       sync.RWMutex
	byToken  map[string]*grantState
	listener net.Listener
	server   *http.Server
	url      string
	client   *http.Transport
	extra    map[string]*http.Server // additional listeners by host (containers)
	extraURL map[string]string
	// Hooks serves HooksPrefix routes for the tools holding a valid session
	// token (oh plugin → oh daemon); the grant owner is in the request
	// context (HookOwner).
	Hooks http.Handler
}

// HooksPrefix is the path prefix of the oh hook routes on the proxy listeners.
const HooksPrefix = "/oh/v1/hooks/"

type ownerKey struct{}

// HookOwner returns the owner (server group) of the token of a hook request.
func HookOwner(r *http.Request) string {
	s, _ := r.Context().Value(ownerKey{}).(string)
	return s
}

// New returns an unstarted proxy.
func New() *Proxy {
	return &Proxy{
		byToken: map[string]*grantState{},
		client: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        32,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// Start listens on addr ("127.0.0.1:0" for a free local port).
func (p *Proxy) Start(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("credproxy: listen %s: %w", addr, err)
	}
	p.listener = l
	p.url = "http://" + l.Addr().String()
	p.server = &http.Server{Handler: p, ReadHeaderTimeout: 30 * time.Second}
	go func() {
		if err := p.server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("credproxy stopped", "error", err)
		}
	}()
	return nil
}

// URL is the proxy base URL (http://127.0.0.1:port).
func (p *Proxy) URL() string { return p.url }

// BaseURL returns the provider base URL to configure in the tool.
func (p *Proxy) BaseURL(provider string) string { return p.url + "/" + provider }

// Listen adds a listener on host (an address reachable from containers, e.g.
// a bridge gateway), on the main port when free, and returns its base URL.
// The same tokens are accepted on every listener. Idempotent per host.
func (p *Proxy) Listen(host string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if u, ok := p.extraURL[host]; ok {
		return u, nil
	}
	if p.listener == nil {
		return "", errors.New("credproxy: not started")
	}
	port := p.listener.Addr().(*net.TCPAddr).Port
	l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		if l, err = net.Listen("tcp", net.JoinHostPort(host, "0")); err != nil {
			return "", fmt.Errorf("credproxy: listen %s: %w", host, err)
		}
	}
	srv := &http.Server{Handler: p, ReadHeaderTimeout: 30 * time.Second}
	go func() {
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("credproxy listener stopped", "addr", l.Addr().String(), "error", err)
		}
	}()
	if p.extra == nil {
		p.extra, p.extraURL = map[string]*http.Server{}, map[string]string{}
	}
	u := "http://" + l.Addr().String()
	p.extra[host], p.extraURL[host] = srv, u
	return u, nil
}

// Listeners returns the base URLs of the additional listeners by host.
func (p *Proxy) Listeners() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]string, len(p.extraURL))
	for k, v := range p.extraURL {
		out[k] = v
	}
	return out
}

// Close stops the proxy.
func (p *Proxy) Close(ctx context.Context) error {
	p.mu.Lock()
	extra := p.extra
	p.extra, p.extraURL = nil, nil
	p.mu.Unlock()
	for _, s := range extra {
		_ = s.Shutdown(ctx)
	}
	if p.server == nil {
		return nil
	}
	return p.server.Shutdown(ctx)
}

// Issue registers a grant and returns the session token to hand to the tool.
func (p *Proxy) Issue(g Grant) (string, error) {
	if _, err := url.Parse(g.Upstream.BaseURL); err != nil {
		return "", fmt.Errorf("credproxy: invalid upstream: %w", err)
	}
	tok := NewToken()
	if err := p.IssueWithToken(tok, g); err != nil {
		return "", err
	}
	return tok, nil
}

// IssueWithToken registers a grant under an existing token (restoring a
// persisted grant after a daemon restart, or replacing its credential).
func (p *Proxy) IssueWithToken(token string, g Grant) error {
	if !strings.HasPrefix(token, "ohs_") {
		return errors.New("credproxy: invalid token format")
	}
	if g.Provider == "" || g.Upstream.BaseURL == "" || g.Upstream.Auth == nil {
		return errors.New("credproxy: grant needs provider, upstream URL and auth")
	}
	p.mu.Lock()
	prev := p.byToken[token]
	st := &grantState{Grant: g, token: token}
	if prev != nil {
		prev.mu.Lock()
		st.usage = prev.usage
		prev.mu.Unlock()
	}
	p.byToken[token] = st
	p.mu.Unlock()
	return nil
}

// Has reports whether a token is currently active.
func (p *Proxy) Has(token string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.byToken[token]
	return ok
}

// Revoke invalidates a session token.
func (p *Proxy) Revoke(token string) {
	p.mu.Lock()
	delete(p.byToken, token)
	p.mu.Unlock()
}

// RevokeSession invalidates every token of a session.
func (p *Proxy) RevokeSession(sessionID string) {
	p.mu.Lock()
	for t, g := range p.byToken {
		if g.SessionID == sessionID {
			delete(p.byToken, t)
		}
	}
	p.mu.Unlock()
}

// Usage returns the usage accounted to a token.
func (p *Proxy) Usage(token string) (Usage, bool) {
	p.mu.RLock()
	g, ok := p.byToken[token]
	p.mu.RUnlock()
	if !ok {
		return Usage{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage, true
}

// Exhausted reports whether a token has used up its token budget.
func (p *Proxy) Exhausted(token string) bool {
	p.mu.RLock()
	g, ok := p.byToken[token]
	p.mu.RUnlock()
	if !ok || g.MaxTokens <= 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage.InputTokens+g.usage.OutputTokens >= g.MaxTokens
}

// NewToken returns a new random session token ("ohs_" + 64 hex chars).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("credproxy: crypto/rand failed: %v", err))
	}
	return "ohs_" + hex.EncodeToString(b)
}

// inboundToken extracts the session token the tool sent.
func inboundToken(r *http.Request) string {
	if v := r.Header.Get("x-api-key"); v != "" {
		return v
	}
	if v := r.Header.Get("Authorization"); v != "" {
		if t, ok := strings.CutPrefix(v, "Bearer "); ok {
			return strings.TrimSpace(t)
		}
	}
	return ""
}

func (p *Proxy) serveHook(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	g, ok := p.byToken[inboundToken(r)]
	hooks := p.Hooks
	p.mu.RUnlock()
	if !ok {
		http.Error(w, "credproxy: invalid session token", http.StatusUnauthorized)
		return
	}
	if hooks == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	hooks.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, g.SessionID)))
}

// strippedHeaders never reach the upstream (tool credentials, hop-by-hop).
// Accept-Encoding is removed so that the transport negotiates compression
// itself and hands back a decoded body (usage accounting reads it).
var strippedHeaders = []string{"Authorization", "X-Api-Key", "Proxy-Authorization", "X-Amz-Security-Token", "X-Amz-Date", "X-Amz-Content-Sha256", "Accept-Encoding"}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, HooksPrefix) {
		p.serveHook(w, r)
		return
	}
	provider, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	p.mu.RLock()
	g, ok := p.byToken[inboundToken(r)]
	p.mu.RUnlock()
	if !ok || g.Provider != provider {
		http.Error(w, "credproxy: invalid session token", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		http.Error(w, "credproxy: reading request", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	if model := requestModel(rest, body); !modelAllowed(g.AllowedModels, model) {
		http.Error(w, fmt.Sprintf("credproxy: model %q not allowed for this session", model), http.StatusForbidden)
		return
	}
	if g.MaxTokens > 0 {
		g.mu.Lock()
		used := g.usage.InputTokens + g.usage.OutputTokens
		g.mu.Unlock()
		if used >= g.MaxTokens {
			http.Error(w, "credproxy: session token budget exhausted", http.StatusTooManyRequests)
			return
		}
	}

	upstream, _ := url.Parse(g.Upstream.BaseURL)
	rp := &httputil.ReverseProxy{
		Transport:     p.client,
		FlushInterval: -1,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			// Keep the inbound escaping (e.g. Bedrock "%3A" in model ids).
			rawRest := strings.TrimPrefix(pr.In.URL.EscapedPath(), "/"+provider)
			pr.Out.URL.Path = strings.TrimRight(upstream.Path, "/") + "/" + rest
			pr.Out.URL.RawPath = strings.TrimRight(upstream.EscapedPath(), "/") + rawRest
			pr.Out.Host = upstream.Host
			for _, h := range strippedHeaders {
				pr.Out.Header.Del(h)
			}
			pr.Out.Body = io.NopCloser(bytes.NewReader(body))
			pr.Out.ContentLength = int64(len(body))
			if err := g.Upstream.Auth.Apply(pr.Out, body); err != nil {
				slog.Error("credproxy: applying credentials", "provider", provider, "error", err)
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Body = &usageReader{rc: resp.Body, g: g}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			slog.Warn("credproxy: upstream error", "provider", provider, "error", err)
			http.Error(w, "credproxy: upstream unavailable", http.StatusBadGateway)
		},
	}
	g.mu.Lock()
	g.usage.Requests++
	g.mu.Unlock()
	rp.ServeHTTP(w, r)
}

// requestModel extracts the model id: Bedrock carries it in the path
// (/model/<id>/converse[-stream] or /invoke…), others in the JSON body.
func requestModel(path string, body []byte) string {
	if rest, ok := strings.CutPrefix(path, "model/"); ok {
		id, _, _ := strings.Cut(rest, "/")
		if dec, err := url.PathUnescape(id); err == nil {
			return dec
		}
		return id
	}
	var b struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &b) == nil {
		return b.Model
	}
	return ""
}

func modelAllowed(patterns []string, model string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if wildcard(p, model) {
			return true
		}
	}
	return false
}

func wildcard(pattern, s string) bool {
	p, v := []rune(pattern), []rune(s)
	pi, si, star, mark := 0, 0, -1, 0
	for si < len(v) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == v[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}
