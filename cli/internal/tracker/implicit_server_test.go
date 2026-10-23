package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The servers that a source implies when none is declared: the key of the
// tracker serves the tracker, the MCP tokens serve gitlab.com when the MCP has
// no url; GITLAB_TOKEN alone stays strict.
func TestResolveGitLabTokenImplicitServers(t *testing.T) {
	const (
		octo = "https://gitlab.octo.example"
		mcp  = config.DefaultGitLabTokenKey
	)
	tests := []struct {
		name    string
		server  string
		env     string
		query   GitLabTokenQuery
		secrets map[string]string
		want    string
		wantSrc TokenSource
		looked  map[TokenSource]TokenOutcome
	}{
		{"tracker key, implicit server is the tracker called", forge, "",
			GitLabTokenQuery{TrackerKey: "k", TrackerServer: forge, TrackerImplicit: true},
			map[string]string{"k": "t"}, "t", TokenSourceTrackerKey, nil},
		{"tracker key, implicit server is another server: refused", forge, "",
			GitLabTokenQuery{TrackerKey: "k", TrackerServer: octo, TrackerImplicit: true},
			map[string]string{"k": "t"}, "", TokenSourceNone, map[TokenSource]TokenOutcome{TokenSourceTrackerKey: OutcomeOtherServer}},
		{"former tracker key follows the implicit server of the tracker", forge, "",
			GitLabTokenQuery{TrackerServer: forge, TrackerImplicit: true},
			map[string]string{DefaultTrackerGitLabTokenKey: "t"}, "t", TokenSourceTracker, nil},
		{"former tracker key, implicit server is another server: refused", forge, "",
			GitLabTokenQuery{TrackerServer: octo, TrackerImplicit: true},
			map[string]string{DefaultTrackerGitLabTokenKey: "t"}, "", TokenSourceNone, map[TokenSource]TokenOutcome{TokenSourceTracker: OutcomeOtherServer}},
		{"MCP token, no url: gitlab.com", DefaultMCPGitLabServer, "",
			GitLabTokenQuery{MCPServer: DefaultMCPGitLabServer, MCPImplicit: true},
			map[string]string{mcp: "t"}, "t", TokenSourceMCP, nil},
		{"MCP token, no url: not sent to another server", forge, "",
			GitLabTokenQuery{MCPServer: DefaultMCPGitLabServer, MCPImplicit: true},
			map[string]string{mcp: "t"}, "", TokenSourceNone, map[TokenSource]TokenOutcome{TokenSourceMCP: OutcomeOtherServer}},
		{"project token, no url: gitlab.com", "https://GitLab.com/", "",
			GitLabTokenQuery{ProjectID: "p1", MCPServer: DefaultMCPGitLabServer, MCPImplicit: true},
			map[string]string{config.MCPProjectTokenKey("gitlab", "p1"): "t"}, "t", TokenSourceProject, nil},
		{"MCP token with an explicit url keeps it", forge, "",
			GitLabTokenQuery{MCPServer: octo},
			map[string]string{mcp: "t"}, "", TokenSourceNone, map[TokenSource]TokenOutcome{TokenSourceMCP: OutcomeOtherServer}},
		{"GITLAB_TOKEN alone stays strict", DefaultMCPGitLabServer, "t-env",
			GitLabTokenQuery{},
			map[string]string{}, "", TokenSourceNone, map[TokenSource]TokenOutcome{TokenSourceEnv: OutcomeNoServer}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(GitLabTokenEnv, tt.env)
			t.Setenv(GitLabServerEnv, "")
			t.Setenv("GITLAB_URL", "")
			q := tt.query
			q.Server = tt.server
			q.Secrets = getOnlySecrets(tt.secrets)
			res := ResolveGitLabToken(t.Context(), q)
			assert.Equal(t, tt.want, res.Token)
			if tt.want != "" {
				assert.Equal(t, tt.wantSrc, res.Source)
			}
			got := map[TokenSource]TokenOutcome{}
			for _, l := range res.Looks {
				got[l.Source] = l.Outcome
			}
			for src, outcome := range tt.looked {
				assert.Equal(t, outcome, got[src], string(src))
			}
		})
	}
}

// The server of the key of the tracker: the declared one wins over the implicit
// one; the implicit one is the tracker_url of the level of the key, else the
// effective tracker.
func TestTokenServer(t *testing.T) {
	shared := func(key, url, tokenURL string) *teamstate.TrackerConfig {
		return &teamstate.TrackerConfig{Type: "gitlab", TrackerTokenKey: key, TrackerURL: url, TrackerTokenURL: tokenURL}
	}
	tests := []struct {
		name         string
		shared       *teamstate.TrackerConfig
		local        config.TrackerLocalConfig
		project      *domain.ProjectTrackerConfig
		wantServer   string
		wantImplicit bool
	}{
		{"declared wins over the url of the level", shared("k", forge, "https://declared.example"), config.TrackerLocalConfig{}, nil, "https://declared.example", false},
		{"key of the team: the url of the team", shared("k", forge, ""), config.TrackerLocalConfig{}, nil, forge, true},
		{"key of the team, tracker url overridden by hub.toml: still the url of the key's level", shared("k", forge, ""),
			config.TrackerLocalConfig{TrackerURL: "https://other.example"}, nil, forge, true},
		{"key of hub.toml with its own url", shared("team.k", "https://team.example", ""),
			config.TrackerLocalConfig{TrackerTokenKey: "k", TrackerURL: forge}, nil, forge, true},
		{"key of hub.toml without url: the effective tracker", shared("team.k", "https://team.example", ""),
			config.TrackerLocalConfig{TrackerTokenKey: "k"}, nil, "https://team.example", true},
		{"key of hub.toml does not inherit the server declared with the team key", shared("team.k", "https://team.example", "https://declared.example"),
			config.TrackerLocalConfig{TrackerTokenKey: "k"}, nil, "https://team.example", true},
		{"key of the project", shared("team.k", "https://team.example", ""), config.TrackerLocalConfig{},
			&domain.ProjectTrackerConfig{TrackerTokenKey: "k", TrackerURL: forge}, forge, true},
		{"no url anywhere: gitlab.com", shared("k", "", ""), config.TrackerLocalConfig{}, nil, "https://gitlab.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_URL", "")
			eff := ResolveFullTrackerConfig(tt.shared, tt.local, tt.project)
			server, implicit := eff.TokenServer()
			assert.Equal(t, tt.wantServer, server)
			assert.Equal(t, tt.wantImplicit, implicit)
		})
	}
}

// ResolveCredentials: the key of the tracker serves the tracker without any
// declaration, never another server, and a declaration beats the implicit one.
func TestResolveCredentialsImplicitServerOfTheTrackerKey(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	t.Setenv(GitLabServerEnv, "")
	t.Setenv("GITLAB_URL", "")
	secrets := getOnlySecrets{"k": "t-key"}

	cfg, err := ResolveCredentials(t.Context(), CredentialSource{URL: forge, TokenKey: "k", Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "t-key", cfg.Token)
	assert.Equal(t, TokenSourceTrackerKey, cfg.Origin.Source)

	cfg, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, TokenKey: "k", TokenLevelURL: forge, Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "t-key", cfg.Token)

	// the key was set next to another tracker_url than the one called
	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, TokenKey: "k", TokenLevelURL: "https://other.example", Secrets: secrets}, TypeGitLab)
	assert.True(t, IsMissingToken(err))

	// a declared server wins over the implicit one, even when the implicit one matches
	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, TokenKey: "k", TokenURL: "https://other.example", TokenLevelURL: forge, Secrets: secrets}, TypeGitLab)
	assert.True(t, IsMissingToken(err))
	cfg, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, TokenKey: "k", TokenURL: forge, TokenLevelURL: "https://other.example", Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "t-key", cfg.Token)

	// the default tracker is gitlab.com
	cfg, err = ResolveCredentials(t.Context(), CredentialSource{TokenKey: "k", Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.com", cfg.BaseURL)

	// MCP token without url: only gitlab.com
	mcp := getOnlySecrets{config.DefaultGitLabTokenKey: "t-mcp"}
	cfg, err = ResolveCredentials(t.Context(), CredentialSource{MCPServer: DefaultMCPGitLabServer, MCPImplicit: true, Secrets: mcp}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "t-mcp", cfg.Token)
	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, MCPServer: DefaultMCPGitLabServer, MCPImplicit: true, Secrets: mcp}, TypeGitLab)
	assert.True(t, IsMissingToken(err))
}

func TestMissingTokenErrorSaysWhenAServerIsImplicit(t *testing.T) {
	i18ntest.Set(t, "fr")
	e := &MissingTokenError{
		Server:  forge,
		Purpose: PurposeTracker,
		Looks: []TokenLook{
			{Source: TokenSourceTrackerKey, Key: "k", Server: "https://gitlab.octo.example", Implicit: true, Outcome: OutcomeOtherServer},
			{Source: TokenSourceMCP, Key: config.DefaultGitLabTokenKey, Server: DefaultMCPGitLabServer, Implicit: true, Outcome: OutcomeEmpty},
			{Source: TokenSourceTracker, Key: DefaultTrackerGitLabTokenKey, Server: "https://gitlab.forge.example", Implicit: true, Outcome: OutcomeEmpty},
		},
	}
	msg := e.Error()
	assert.Contains(t, msg, "tracker_token_key « k » : serveur du tracker (implicite) : gitlab.octo.example, pas pour gitlab.forge.example")
	assert.Contains(t, msg, "jeton MCP (clé openhub.mcp.gitlab.token) : aucun jeton enregistré — serveur du MCP GitLab (implicite, défaut) : gitlab.com")
	assert.Contains(t, msg, "ancienne clé du tracker (openhub.tracker.gitlab.token) : aucun jeton enregistré — serveur du tracker (implicite) : gitlab.forge.example")
}
