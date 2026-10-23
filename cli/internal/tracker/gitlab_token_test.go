package tracker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

// getOnlySecrets is a read-only keychain (no Set): the legacy copy is not tried.
type getOnlySecrets map[string]string

func (s getOnlySecrets) Get(_ context.Context, key string) (string, error) { return s[key], nil }

// rwSecrets also implements config.SecretReadWriter.
type rwSecrets struct{ getOnlySecrets }

func (s rwSecrets) Set(_ context.Context, key, val string) error {
	s.getOnlySecrets[key] = val
	return nil
}

type failingSecrets struct{}

func (failingSecrets) Get(context.Context, string) (string, error) { return "", errors.New("locked") }

const forge = "https://gitlab.forge.example"

// everyServer declares the server `forge` for every source that needs one.
func everyServer(q GitLabTokenQuery) GitLabTokenQuery {
	q.Server = forge
	q.TrackerServer, q.TeamServer, q.MCPServer = forge, forge, forge
	return q
}

func TestResolveGitLabTokenCascade(t *testing.T) {
	const (
		trackerKey = "custom.tracker.key"
		mcpKey     = "custom.mcp.key"
	)
	projectKey := config.MCPProjectTokenKey("gitlab", "p1")
	teamKey := config.TeamGitLabTokenKey("core")
	allQuery := everyServer(GitLabTokenQuery{TrackerKey: trackerKey, ProjectID: "p1", TeamID: "core", MCPKey: mcpKey})
	every := map[string]string{
		trackerKey:                         "t-tracker-key",
		projectKey:                         "t-project",
		teamKey:                            "t-team",
		mcpKey:                             "t-mcp-configured",
		config.DefaultGitLabTokenKey:       "t-mcp-default",
		DefaultTrackerGitLabTokenKey:       "t-tracker-default",
		config.LegacyMCPTokenKey("gitlab"): "t-mcp-legacy",
	}
	without := func(keys ...string) map[string]string {
		out := map[string]string{}
		for k, v := range every {
			out[k] = v
		}
		for _, k := range keys {
			delete(out, k)
		}
		return out
	}

	tests := []struct {
		name       string
		env        string
		secrets    map[string]string
		query      GitLabTokenQuery
		wantToken  string
		wantSource TokenSource
		wantKey    string
	}{
		{"env wins over every key", "t-env", every, allQuery, "t-env", TokenSourceEnv, GitLabTokenEnv},
		{"tracker_token_key comes right after env", "", every, allQuery, "t-tracker-key", TokenSourceTrackerKey, trackerKey},
		{"project MCP token before the team token", "", without(trackerKey), allQuery, "t-project", TokenSourceProject, projectKey},
		{"team token before the hub MCP token", "", without(trackerKey, projectKey), allQuery, "t-team", TokenSourceTeam, teamKey},
		{"configured MCP key before the default MCP key", "", without(trackerKey, projectKey, teamKey), allQuery, "t-mcp-configured", TokenSourceMCP, mcpKey},
		{"default MCP key before the legacy name", "", without(trackerKey, projectKey, teamKey, mcpKey), allQuery, "t-mcp-default", TokenSourceMCP, config.DefaultGitLabTokenKey},
		{"legacy MCP name before the former tracker key", "", without(trackerKey, projectKey, teamKey, mcpKey, config.DefaultGitLabTokenKey), allQuery, "t-mcp-legacy", TokenSourceMCP, config.DefaultGitLabTokenKey},
		{"former tracker key last", "", map[string]string{DefaultTrackerGitLabTokenKey: "t-tracker-default"}, allQuery, "t-tracker-default", TokenSourceTracker, DefaultTrackerGitLabTokenKey},
		{"nothing found", "", map[string]string{}, allQuery, "", TokenSourceNone, ""},
		{"empty query only reads the hub keys", "", every, everyServer(GitLabTokenQuery{}), "t-mcp-default", TokenSourceMCP, config.DefaultGitLabTokenKey},
		{"team key skipped without team", "", map[string]string{teamKey: "t-team"}, everyServer(GitLabTokenQuery{}), "", TokenSourceNone, ""},
		{"project key skipped without project", "", map[string]string{projectKey: "t-project"}, everyServer(GitLabTokenQuery{TeamID: "core"}), "", TokenSourceNone, ""},
		{"another team's key is not used", "", map[string]string{config.TeamGitLabTokenKey("other"): "t-other"}, everyServer(GitLabTokenQuery{TeamID: "core"}), "", TokenSourceNone, ""},
		{"MCP key equal to the default is read once", "", map[string]string{config.DefaultGitLabTokenKey: "t-mcp-default"}, everyServer(GitLabTokenQuery{MCPKey: config.DefaultGitLabTokenKey}), "t-mcp-default", TokenSourceMCP, config.DefaultGitLabTokenKey},
	}
	for _, tt := range tests {
		for _, store := range []string{"read-only", "read-write"} {
			t.Run(tt.name+"/"+store, func(t *testing.T) {
				t.Setenv(GitLabTokenEnv, tt.env)
				t.Setenv(GitLabServerEnv, forge)
				data := getOnlySecrets{}
				for k, v := range tt.secrets {
					data[k] = v
				}
				q := tt.query
				if store == "read-only" {
					q.Secrets = data
				} else {
					q.Secrets = rwSecrets{data}
				}
				res := ResolveGitLabToken(t.Context(), q)
				assert.Equal(t, tt.wantToken, res.Token)
				assert.Equal(t, tt.wantSource, res.Source)
				assert.Equal(t, tt.wantKey, res.Key)
			})
		}
	}
}

// A token is only returned for the server its source is declared for.
func TestResolveGitLabTokenOnlyAnswersForItsServer(t *testing.T) {
	const octo = "https://gitlab.octo.example"
	secrets := getOnlySecrets{
		"tracker.key":                             "t-tracker",
		config.TeamGitLabTokenKey("core"):         "t-team",
		config.DefaultGitLabTokenKey:              "t-mcp",
		config.MCPProjectTokenKey("gitlab", "p1"): "t-project",
		DefaultTrackerGitLabTokenKey:              "t-former",
	}
	base := GitLabTokenQuery{
		TrackerKey: "tracker.key", TrackerServer: forge,
		ProjectID: "p1",
		TeamID:    "core", TeamServer: octo,
		MCPServer: octo,
		Secrets:   secrets,
	}
	tests := []struct {
		name      string
		server    string
		env       string
		envServer string
		mutate    func(*GitLabTokenQuery)
		want      string
		wantSrc   TokenSource
	}{
		{"the tracker on forge gets the tracker key, not the team token", forge, "", "", nil, "t-tracker", TokenSourceTrackerKey},
		{"the project on octo gets its own token", octo, "", "", nil, "t-project", TokenSourceProject},
		{"the team token serves the server of the team repository", octo, "", "", func(q *GitLabTokenQuery) { q.ProjectID = "" }, "t-team", TokenSourceTeam},
		{"nothing serves a third server", "https://other.example", "", "", nil, "", TokenSourceNone},
		{"the former tracker key follows the declared tracker server", forge, "", "", func(q *GitLabTokenQuery) { q.TrackerKey = "" }, "t-former", TokenSourceTracker},
		{"without a declared server the tracker key is never used", forge, "", "", func(q *GitLabTokenQuery) { q.TrackerServer = "" }, "", TokenSourceNone},
		{"the environment token follows OH_GITLAB_URL", forge, "t-env", forge, nil, "t-env", TokenSourceEnv},
		{"the environment token follows GITLAB_URL as a fallback", forge, "t-env", "", func(q *GitLabTokenQuery) { t.Setenv("GITLAB_URL", forge) }, "t-env", TokenSourceEnv},
		{"the environment token is not sent to another server", octo, "t-env", forge, func(q *GitLabTokenQuery) { q.ProjectID = ""; q.TeamID = ""; q.MCPServer = "" }, "", TokenSourceNone},
		{"the environment token without a declared server is never sent", forge, "t-env", "", nil, "t-tracker", TokenSourceTrackerKey},
		{"AnyServer lifts the rule", "https://other.example", "", "", func(q *GitLabTokenQuery) { q.AnyServer = true }, "t-tracker", TokenSourceTrackerKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(GitLabTokenEnv, tt.env)
			t.Setenv(GitLabServerEnv, tt.envServer)
			t.Setenv("GITLAB_URL", "")
			q := base
			q.Server = tt.server
			if tt.mutate != nil {
				tt.mutate(&q)
			}
			res := ResolveGitLabToken(t.Context(), q)
			assert.Equal(t, tt.want, res.Token)
			if tt.want != "" {
				assert.Equal(t, tt.wantSrc, res.Source)
			}
		})
	}
}

func TestResolveGitLabTokenReportsTheSourcesRead(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "t-env")
	t.Setenv(GitLabServerEnv, "")
	t.Setenv("GITLAB_URL", "")
	res := ResolveGitLabToken(t.Context(), GitLabTokenQuery{
		Server:     forge,
		TrackerKey: "tracker.key",
		TeamID:     "core", TeamServer: "https://gitlab.octo.example",
		MCPServer: "",
		Secrets: getOnlySecrets{
			"tracker.key":                     "",
			config.TeamGitLabTokenKey("core"): "t-team",
			config.DefaultGitLabTokenKey:      "t-mcp",
		},
	})
	require.False(t, res.Found())
	outcomes := map[TokenSource]TokenOutcome{}
	for _, l := range res.Looks {
		outcomes[l.Source] = l.Outcome
	}
	assert.Equal(t, map[TokenSource]TokenOutcome{
		TokenSourceEnv:        OutcomeNoServer,
		TokenSourceTrackerKey: OutcomeEmpty,
		TokenSourceTeam:       OutcomeOtherServer,
		TokenSourceMCP:        OutcomeNoServer,
		TokenSourceTracker:    OutcomeEmpty,
	}, outcomes)
	assert.Equal(t, forge, res.Called)
}

func TestResolveGitLabTokenWithoutKeychain(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	res := ResolveGitLabToken(t.Context(), GitLabTokenQuery{TeamID: "core", Server: forge})
	assert.Empty(t, res.Token)
	assert.Equal(t, TokenSourceNone, res.Source)

	t.Setenv(GitLabTokenEnv, "from-env")
	t.Setenv(GitLabServerEnv, forge)
	res = ResolveGitLabToken(t.Context(), GitLabTokenQuery{Server: forge})
	assert.Equal(t, "from-env", res.Token)
	assert.Equal(t, TokenSourceEnv, res.Source)
}

func TestResolveGitLabTokenKeychainFailureIsAMiss(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	res := ResolveGitLabToken(t.Context(), GitLabTokenQuery{Server: forge, TeamID: "core", TeamServer: forge, TrackerKey: "k", TrackerServer: forge, Secrets: failingSecrets{}})
	assert.Empty(t, res.Token)
	assert.Equal(t, TokenSourceNone, res.Source)
}

func TestResolveGitLabTokenCopiesLegacyMCPKeyOnWritableStore(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	data := getOnlySecrets{config.LegacyMCPTokenKey("gitlab"): "old"}
	res := ResolveGitLabToken(t.Context(), GitLabTokenQuery{Server: forge, MCPServer: forge, Secrets: rwSecrets{data}})
	assert.Equal(t, "old", res.Token)
	assert.Equal(t, TokenSourceMCP, res.Source)
	assert.Equal(t, "old", data[config.DefaultGitLabTokenKey], "the token is copied under the current name")
}

func TestResolveCredentialsGitLabUsesCascade(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	t.Setenv("GITLAB_URL", "")
	t.Setenv(GitLabServerEnv, "")
	secrets := getOnlySecrets{config.TeamGitLabTokenKey("core"): "team-tok"}

	cfg, err := ResolveCredentials(t.Context(), CredentialSource{TeamID: "core", TeamServer: "gitlab.com", Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "team-tok", cfg.Token)
	assert.Equal(t, "https://gitlab.com", cfg.BaseURL)
	assert.Equal(t, TokenOrigin{Source: TokenSourceTeam, Key: config.TeamGitLabTokenKey("core"), Server: "https://gitlab.com"}, cfg.Origin)

	_, err = ResolveCredentials(t.Context(), CredentialSource{TeamID: "other", TeamServer: "gitlab.com", Secrets: secrets}, TypeGitLab)
	require.Error(t, err)
	assert.True(t, IsMissingToken(err))

	// the same token for a tracker on another server
	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: forge, TeamID: "core", TeamServer: "gitlab.com", Secrets: secrets}, TypeGitLab)
	var missing *MissingTokenError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, forge, missing.Server)
}

func TestResolveCredentialsJiraKeepsItsOwnCascade(t *testing.T) {
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv(GitLabTokenEnv, "gl-env")
	secrets := getOnlySecrets{config.TeamGitLabTokenKey("core"): "team-tok", "jira.key": "jira-tok"}

	cfg, err := ResolveCredentials(t.Context(), CredentialSource{URL: "https://j.example", TokenKey: "jira.key", TeamID: "core", Secrets: secrets}, TypeJira)
	require.NoError(t, err)
	assert.Equal(t, "jira-tok", cfg.Token, "a GitLab token is never used for Jira")
	assert.Equal(t, TokenOrigin{Source: TokenSourceTrackerKey, Key: "jira.key", Server: "https://j.example"}, cfg.Origin)

	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: "https://j.example", TeamID: "core", Secrets: secrets}, TypeJira)
	require.Error(t, err)
}
