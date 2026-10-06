package domain

import (
	"context"
	"time"
)

// ServerStatus is the lifecycle state of a tool server.
type ServerStatus string

const (
	ServerStarting ServerStatus = "starting"
	ServerReady    ServerStatus = "ready"
	ServerSleeping ServerStatus = "sleeping"
	ServerStopped  ServerStatus = "stopped"
)

// Server is a tool server (e.g. `opencode serve`) shared by the sessions of a
// group (bundle hash, project, runtime).
type Server struct {
	GroupKey       string
	Adapter        string // e.g. "opencode-v2"
	AdapterVersion string
	Runtime        string // local | container | remote
	ProjectID      string
	BundleHash     string
	PID            int
	URL            string
	Port           int
	Password       string
	DataDir        string
	WorkDir        string
	ProxyTokenHash string // hash of the credential proxy token held by the server process (credproxy.TokenHash)
	Status         ServerStatus
	CreatedAt      time.Time // (re)start time of the current process
	LastActivityAt time.Time
}

// ServerStore persists tool servers.
type ServerStore interface {
	Upsert(ctx context.Context, s *Server) error
	Get(ctx context.Context, groupKey string) (*Server, error)
	List(ctx context.Context) ([]Server, error)
	SetStatus(ctx context.Context, groupKey string, status ServerStatus) error
	// SetStatusIf changes the status only if the row still describes the same
	// process (pid) in the expected status. Returns whether it changed.
	SetStatusIf(ctx context.Context, groupKey string, pid int, from, to ServerStatus) (bool, error)
	Touch(ctx context.Context, groupKey string, at time.Time) error
	Delete(ctx context.Context, groupKey string) error
}

// CredentialSourceKind tells how a credential is obtained on the host.
type CredentialSourceKind string

const (
	CredentialBearer CredentialSourceKind = "bearer"  // Authorization: Bearer <secret>
	CredentialAPIKey CredentialSourceKind = "api_key" // provider API key header
	CredentialSigV4  CredentialSourceKind = "sigv4"   // AWS profile / default chain
)

// CredentialSource references a credential without containing it.
type CredentialSource struct {
	Kind        CredentialSourceKind `json:"kind"`
	KeychainKey string               `json:"keychain_key,omitempty"` // secret store key (bearer/api_key)
	Profile     string               `json:"profile,omitempty"`      // AWS profile (sigv4; empty = default chain)
	Scope       string               `json:"scope,omitempty"`        // project | team | hub | aws (informational)
}

// ProxyGrant is a credential proxy token persisted so that the oh daemon can
// restore it after a restart. It never contains the secret itself.
type ProxyGrant struct {
	TokenHash     string // credproxy.TokenHash of the token (the token itself is never stored)
	Owner         string // server group key (one token per tool server)
	Provider      string
	Region        string
	Source        CredentialSource
	AllowedModels []string
	MaxTokens     int64
	CreatedAt     time.Time
	RevokedAt     *time.Time
}

// GrantStore persists proxy grants.
type GrantStore interface {
	Insert(ctx context.Context, g *ProxyGrant) error
	ListActive(ctx context.Context) ([]ProxyGrant, error)
	Revoke(ctx context.Context, token string, at time.Time) error
	RevokeOwner(ctx context.Context, owner string, at time.Time) error
}

// LegacyTokenHasher converts the tokens stored in clear by older oh versions
// into their hash (hash(token) for each value starting with prefix).
type LegacyTokenHasher interface {
	HashLegacyTokens(ctx context.Context, prefix string, hash func(string) string) (int, error)
}
