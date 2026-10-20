package credproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/safego"
)

// CredentialRef names the real credential of a grant without containing it:
// it is logged and reported when the provider refuses the credential.
type CredentialRef struct {
	Kind    string // bearer | api_key | sigv4
	Key     string // secret store key (bearer/api_key)
	Scope   string // project | team | hub | aws
	Profile string // AWS profile (sigv4)
	// ID distinguishes two values of the same key (an opaque fingerprint
	// computed by the caller, never logged): a renewed key refused again is
	// a new refusal.
	ID string
}

// AuthRejection is a request the provider refused with 401/403 because of
// the credential of the grant.
type AuthRejection struct {
	Owner      string // grant owner (server group)
	Provider   string
	Status     int
	Message    string // excerpt of the provider error message
	Credential CredentialRef
	At         time.Time
}

// maxErrorExcerpt bounds the provider error message kept and logged.
const maxErrorExcerpt = 200

// UpstreamErrorMessage extracts the error message of a provider answer
// (AWS {"message"}/{"Message"}, Anthropic/OpenAI {"error":{"message"}}),
// else the start of the body, on one line and truncated.
func UpstreamErrorMessage(body []byte) string {
	var m struct {
		Message  string `json:"message"`
		MessageU string `json:"Message"`
		Error    any    `json:"error"`
	}
	msg := ""
	if json.Unmarshal(body, &m) == nil {
		switch {
		case m.Message != "":
			msg = m.Message
		case m.MessageU != "":
			msg = m.MessageU
		default:
			switch e := m.Error.(type) {
			case string:
				msg = e
			case map[string]any:
				msg, _ = e["message"].(string)
			}
		}
	}
	if msg == "" {
		msg = string(body)
	}
	msg = strings.Join(strings.Fields(msg), " ")
	if r := []rune(msg); len(r) > maxErrorExcerpt {
		msg = string(r[:maxErrorExcerpt]) + "…"
	}
	return msg
}

// IsAuthFailure reports whether a provider answer refuses the credential
// itself: 401, or 403 other than a denied model or action (the credential
// is valid but lacks a permission).
func IsAuthFailure(status int, message string) bool {
	switch status {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		m := strings.ToLower(message)
		for _, s := range []string{"access to the model", "not authorized to perform", "model access"} {
			if strings.Contains(m, s) {
				return false
			}
		}
		return true
	}
	return false
}

// AuthRejection returns the last credential refusal of a token (or token
// hash) not followed by an accepted request.
func (p *Proxy) AuthRejection(ref string) (AuthRejection, bool) {
	p.mu.RLock()
	g, ok := p.byToken[RefHash(ref)]
	p.mu.RUnlock()
	if !ok {
		return AuthRejection{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.rejected == nil {
		return AuthRejection{}, false
	}
	return *g.rejected, true
}

// checkAuth records the outcome of an upstream answer for the credential of
// g: a refusal (401/403) is logged — key and scope, never the secret — and
// reported once per refusal episode (OnAuthRejected); a success ends it.
func (p *Proxy) checkAuth(g *grantState, provider string, resp *http.Response) {
	if resp.StatusCode < 300 {
		g.mu.Lock()
		g.rejected = nil
		g.mu.Unlock()
		return
	}
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		return
	}
	head, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body = readCloser{io.MultiReader(bytes.NewReader(head), resp.Body), resp.Body}
	msg := UpstreamErrorMessage(head)
	attrs := []any{"provider", provider, "owner", g.SessionID, "status", resp.StatusCode, "message", msg,
		"credential_kind", g.Credential.Kind, "credential_key", g.Credential.Key, "credential_scope", g.Credential.Scope}
	if g.Credential.Profile != "" {
		attrs = append(attrs, "aws_profile", g.Credential.Profile)
	}
	if !IsAuthFailure(resp.StatusCode, msg) {
		slog.Warn("credproxy: upstream refused the request (permission)", attrs...)
		return
	}
	rej := AuthRejection{Owner: g.SessionID, Provider: provider, Status: resp.StatusCode, Message: msg, Credential: g.Credential, At: time.Now()}
	g.mu.Lock()
	first := g.rejected == nil
	g.rejected = &rej
	g.mu.Unlock()
	if !first {
		slog.Debug("credproxy: upstream refused the credential again", attrs...)
		return
	}
	slog.Warn("credproxy: upstream refused the credential", attrs...)
	p.mu.RLock()
	f := p.OnAuthRejected
	p.mu.RUnlock()
	if f != nil {
		safego.Go(func() { f(rej) })
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}

// BedrockControlURL is the Bedrock control plane endpoint of a region
// (bedrock.<region>.amazonaws.com), which accepts Bedrock API keys.
func BedrockControlURL(region string) (string, error) {
	if region == "" {
		region = "us-east-1"
	}
	if err := CheckRegion(region); err != nil {
		return "", err
	}
	return "https://bedrock." + region + ".amazonaws.com", nil
}

// BearerCheck is the outcome of CheckBedrockBearer.
type BearerCheck struct {
	Status  int    // HTTP status of the answer
	Message string // provider error message (refusal)
}

// Accepted reports whether the credential was accepted (it may still lack
// the permission of the probe).
func (c BearerCheck) Accepted() bool {
	return c.Status < 300 || !IsAuthFailure(c.Status, c.Message)
}

// CheckBedrockBearer validates a Bedrock API key with a cheap authenticated
// call (ListFoundationModels of Anthropic) on baseURL (BedrockControlURL).
// err is a network failure (no answer).
func CheckBedrockBearer(ctx context.Context, client *http.Client, baseURL, token string) (BearerCheck, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/foundation-models?byProvider=anthropic", http.NoBody)
	if err != nil {
		return BearerCheck{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return BearerCheck{}, err
	}
	defer resp.Body.Close()
	out := BearerCheck{Status: resp.StatusCode}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		out.Message = UpstreamErrorMessage(body)
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	}
	return out, nil
}
