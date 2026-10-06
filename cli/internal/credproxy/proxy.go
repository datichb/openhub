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
	"crypto/sha256"
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
	hash  string // token hash (the token itself is never kept)
	mu    sync.Mutex
	usage Usage
}

// Proxy is a local HTTP proxy. Routes: /<provider>/... → upstream /... .
type Proxy struct {
	mu       sync.RWMutex
	byToken  map[string]*grantState // by token hash (TokenHash)
	listener net.Listener
	server   *http.Server
	url      string
	client   *http.Transport
	extra    map[string]*http.Server // additional listeners by host (containers)
	extraURL map[string]string
	mounts   map[string]http.Handler // other services sharing the listeners (gateways)
	// ready is non-nil while persisted grants are being restored (Hold):
	// requests with an unknown token wait for it instead of failing.
	ready chan struct{}
	// OnUsage, when set, receives the usage accounted to the grants (by
	// owner), for a persistent ledger shared across restarts and new tokens.
	OnUsage func(owner string, delta Usage)
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

// Close stops the proxy: in-flight requests may finish until ctx is done,
// then the remaining connections (long streams) are closed.
func (p *Proxy) Close(ctx context.Context) error {
	p.mu.Lock()
	extra := p.extra
	p.extra, p.extraURL = nil, nil
	p.mu.Unlock()
	for _, s := range extra {
		if s.Shutdown(ctx) != nil {
			_ = s.Close()
		}
	}
	if p.server == nil {
		return nil
	}
	if err := p.server.Shutdown(ctx); err != nil {
		_ = p.server.Close()
		return err
	}
	return nil
}

// RestoreWait bounds how long a request with an unknown token waits for the
// grants being restored (Hold).
var RestoreWait = 30 * time.Second

// Hold makes requests with an unknown token wait until Release: the grants
// persisted by a previous daemon are being restored.
func (p *Proxy) Hold() {
	p.mu.Lock()
	if p.ready == nil {
		p.ready = make(chan struct{})
	}
	p.mu.Unlock()
}

// Release ends Hold.
func (p *Proxy) Release() {
	p.mu.Lock()
	if p.ready != nil {
		close(p.ready)
		p.ready = nil
	}
	p.mu.Unlock()
}

// lookup returns the grant of a token, waiting for a restoration in progress.
func (p *Proxy) lookup(ctx context.Context, token string) (*grantState, bool) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return nil, false // a hash is not a credential
	}
	key := TokenHash(token)
	p.mu.RLock()
	g, ok := p.byToken[key]
	ready := p.ready
	p.mu.RUnlock()
	if ok || ready == nil || token == "" {
		return g, ok
	}
	t := time.NewTimer(RestoreWait)
	defer t.Stop()
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, false
	case <-t.C:
		return nil, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok = p.byToken[key]
	return g, ok
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

// IssueWithToken registers a grant under an existing token (replacing its
// credential). Only the token hash is kept.
func (p *Proxy) IssueWithToken(token string, g Grant) error {
	if !strings.HasPrefix(token, TokenPrefix) {
		return errors.New("credproxy: invalid token format")
	}
	return p.IssueWithHash(TokenHash(token), g)
}

// IssueWithHash registers a grant under a token hash (restoring a persisted
// grant after a daemon restart: only hashes are stored).
func (p *Proxy) IssueWithHash(hash string, g Grant) error {
	if !isHash(hash) {
		return errors.New("credproxy: invalid token hash")
	}
	if g.Provider == "" || g.Upstream.BaseURL == "" || g.Upstream.Auth == nil {
		return errors.New("credproxy: grant needs provider, upstream URL and auth")
	}
	p.mu.Lock()
	prev := p.byToken[hash]
	st := &grantState{Grant: g, hash: hash}
	if prev != nil {
		prev.mu.Lock()
		st.usage = prev.usage
		prev.mu.Unlock()
	}
	p.byToken[hash] = st
	p.mu.Unlock()
	return nil
}

// Has reports whether a token (or token hash) is currently active.
func (p *Proxy) Has(ref string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.byToken[RefHash(ref)]
	return ok
}

// Revoke invalidates a token (or token hash).
func (p *Proxy) Revoke(ref string) {
	p.mu.Lock()
	delete(p.byToken, RefHash(ref))
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

// Usage returns the usage accounted to a token (or token hash).
func (p *Proxy) Usage(ref string) (Usage, bool) {
	p.mu.RLock()
	g, ok := p.byToken[RefHash(ref)]
	p.mu.RUnlock()
	if !ok {
		return Usage{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage, true
}

// Exhausted reports whether a token (or token hash) has used up its token budget.
func (p *Proxy) Exhausted(ref string) bool {
	p.mu.RLock()
	g, ok := p.byToken[RefHash(ref)]
	p.mu.RUnlock()
	if !ok || g.MaxTokens <= 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage.InputTokens+g.usage.OutputTokens >= g.MaxTokens
}

// TokenPrefix starts every proxy session token.
const TokenPrefix = "ohs_"

// TokenHash is the hash under which a token is known (SHA-256, hex): the
// proxy, the database and the server registry only keep this value.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RefHash returns the hash of a token reference: a token is hashed, a hash
// is returned as is.
func RefHash(ref string) string {
	if strings.HasPrefix(ref, TokenPrefix) {
		return TokenHash(ref)
	}
	return ref
}

func isHash(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// NewToken returns a new random session token ("ohs_" + 64 hex chars).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("credproxy: crypto/rand failed: %v", err))
	}
	return TokenPrefix + hex.EncodeToString(b)
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
	g, ok := p.lookup(r.Context(), inboundToken(r))
	p.mu.RLock()
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

// Mount serves h under /<prefix>/ on every listener (prefix stripped), for
// services that containers reach like the proxy (oh gateways). h does its
// own authentication. prefix must not be a provider name.
func (p *Proxy) Mount(prefix string, h http.Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mounts == nil {
		p.mounts = map[string]http.Handler{}
	}
	p.mounts[prefix] = http.StripPrefix("/"+prefix, h)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, HooksPrefix) {
		p.serveHook(w, r)
		return
	}
	provider, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	p.mu.RLock()
	mounted := p.mounts[provider]
	p.mu.RUnlock()
	if mounted != nil {
		mounted.ServeHTTP(w, r)
		return
	}
	g, ok := p.lookup(r.Context(), inboundToken(r))
	if !ok || g.Provider != provider {
		http.Error(w, "credproxy: invalid session token", http.StatusUnauthorized)
		return
	}

	escapedRest, _ := strings.CutPrefix(r.URL.EscapedPath(), "/"+provider+"/")
	pathModel, err := parsePath(provider, r.Method, escapedRest)
	if err != nil {
		slog.Warn("credproxy: path not relayed", "provider", provider, "method", r.Method, "path", r.URL.EscapedPath())
		http.Error(w, "credproxy: path not relayed for this provider", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxRequestBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "credproxy: request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "credproxy: reading request", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	model := pathModel
	if pathModel == "" {
		m, err := bodyModel(body)
		if err != nil && len(g.AllowedModels) > 0 {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		model = m
	}
	if !modelAllowed(g.AllowedModels, model) {
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

	body = withStreamUsage(provider, escapedRest, body)
	upstream, _ := url.Parse(g.Upstream.BaseURL)
	rp := &httputil.ReverseProxy{
		// Credentials are applied last, on the final request: a failure
		// (e.g. SigV4 credentials expired) refuses the request instead of
		// sending it unsigned.
		Transport:     authTransport{base: p.client, auth: g.Upstream.Auth, body: body},
		FlushInterval: -1,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			// Keep the inbound escaping (e.g. Bedrock "%3A" in model ids).
			pr.Out.URL.Path = strings.TrimRight(upstream.Path, "/") + "/" + rest
			pr.Out.URL.RawPath = strings.TrimRight(upstream.EscapedPath(), "/") + "/" + escapedRest
			pr.Out.Host = upstream.Host
			for _, h := range strippedHeaders {
				pr.Out.Header.Del(h)
			}
			pr.Out.Body = io.NopCloser(bytes.NewReader(body))
			pr.Out.ContentLength = int64(len(body))
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Body = &usageReader{rc: resp.Body, g: g, frames: base64Frames(provider, escapedRest),
				onReport: func(in, out int64) { p.account(g, Usage{InputTokens: in, OutputTokens: out}) }}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if errors.Is(err, errAuth) {
				slog.Error("credproxy: applying credentials", "provider", provider, "error", err)
				http.Error(w, "credproxy: cannot apply the provider credentials", http.StatusBadGateway)
				return
			}
			slog.Warn("credproxy: upstream error", "provider", provider, "error", err)
			http.Error(w, "credproxy: upstream unavailable", http.StatusBadGateway)
		},
	}
	g.mu.Lock()
	g.usage.Requests++
	g.mu.Unlock()
	p.account(g, Usage{Requests: 1})
	rp.ServeHTTP(w, r)
}

func (p *Proxy) account(g *grantState, delta Usage) {
	p.mu.RLock()
	f := p.OnUsage
	p.mu.RUnlock()
	if f != nil {
		f(g.SessionID, delta)
	}
}

// SetUsage sets the usage of a token (or token hash): a restored or newly
// issued grant of a group continues the group's counters.
func (p *Proxy) SetUsage(ref string, u Usage) {
	p.mu.RLock()
	g, ok := p.byToken[RefHash(ref)]
	p.mu.RUnlock()
	if !ok {
		return
	}
	g.mu.Lock()
	g.usage = u
	g.mu.Unlock()
}

// base64Frames reports whether a route answers AWS event-stream frames with
// base64 model chunks (Bedrock InvokeModel with response stream).
func base64Frames(provider, escapedRest string) bool {
	return provider == ProviderBedrock && strings.HasSuffix(escapedRest, "/invoke-with-response-stream")
}

// withStreamUsage asks OpenAI-compatible streams to end with a usage chunk
// (stream_options.include_usage), which they omit otherwise: without it the
// proxy could not count streamed tokens.
func withStreamUsage(provider, escapedRest string, body []byte) []byte {
	if (provider != ProviderOpenAI && provider != ProviderOpenRouter) || (escapedRest != "chat/completions" && escapedRest != "completions") {
		return body
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return body
	}
	var stream bool
	if json.Unmarshal(m["stream"], &stream) != nil || !stream {
		return body
	}
	opts := map[string]json.RawMessage{}
	if raw, ok := m["stream_options"]; ok && json.Unmarshal(raw, &opts) != nil {
		return body
	}
	if string(opts["include_usage"]) == "true" {
		return body
	}
	opts["include_usage"] = json.RawMessage("true")
	raw, err := json.Marshal(opts)
	if err != nil {
		return body
	}
	m["stream_options"] = raw
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

// MaxRequestBytes bounds a relayed request body (413 beyond).
const MaxRequestBytes = 64 << 20

var errAuth = errors.New("credproxy: credentials")

// authTransport applies the real credential to the outgoing request; a
// failure aborts the request before anything reaches the provider.
type authTransport struct {
	base http.RoundTripper
	auth Auth
	body []byte
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.auth.Apply(req, t.body); err != nil {
		return nil, fmt.Errorf("%w: %w", errAuth, err)
	}
	return t.base.RoundTrip(req)
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
