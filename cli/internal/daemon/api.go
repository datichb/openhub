package daemon

import "github.com/datichb/openhub/cli/internal/domain"

// API version prefix.
const apiPrefix = "/v1"

// Health is returned by GET /v1/health.
type Health struct {
	Version       string `json:"version"`
	PID           int    `json:"pid"`
	ProxyURL      string `json:"proxy_url"`
	Servers       int    `json:"servers"`        // live tool servers
	Grants        int    `json:"grants"`         // active proxy grants
	PendingGrants int    `json:"pending_grants"` // grants waiting for their secret
}

// GrantRequest is the body of POST /v1/grants.
type GrantRequest struct {
	Owner         string                  `json:"owner"` // server group key
	Provider      string                  `json:"provider"`
	Region        string                  `json:"region,omitempty"`
	Source        domain.CredentialSource `json:"source"`
	Secret        string                  `json:"secret,omitempty"` // bearer / api key, never persisted
	AllowedModels []string                `json:"allowed_models,omitempty"`
	MaxTokens     int64                   `json:"max_tokens,omitempty"`
}

// GrantResponse is returned by POST /v1/grants.
type GrantResponse struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
}

// SecretRequest provides the secret of a pending grant (POST /v1/grants/secret).
type SecretRequest struct {
	Token  string `json:"token"`
	Secret string `json:"secret"`
}

// UsageResponse is returned by GET /v1/usage?token=…
type UsageResponse struct {
	Requests     int64 `json:"requests"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// PendingGrant describes a restored grant whose secret is not yet available.
type PendingGrant struct {
	Token  string                  `json:"token"`
	Owner  string                  `json:"owner"`
	Source domain.CredentialSource `json:"source"`
}
