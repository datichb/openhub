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

func TestResolveGitLabTokenCascade(t *testing.T) {
	const (
		trackerKey = "custom.tracker.key"
		mcpKey     = "custom.mcp.key"
	)
	projectKey := config.MCPProjectTokenKey("gitlab", "p1")
	teamKey := config.TeamGitLabTokenKey("core")
	allQuery := GitLabTokenQuery{TrackerKey: trackerKey, ProjectID: "p1", TeamID: "core", MCPKey: mcpKey}
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
	}{
		{"env wins over every key", "t-env", every, allQuery, "t-env", TokenSourceEnv},
		{"tracker_token_key comes right after env", "", every, allQuery, "t-tracker-key", TokenSourceTrackerKey},
		{"project MCP token before the team token", "", without(trackerKey), allQuery, "t-project", TokenSourceProject},
		{"team token before the hub MCP token", "", without(trackerKey, projectKey), allQuery, "t-team", TokenSourceTeam},
		{"configured MCP key before the default MCP key", "", without(trackerKey, projectKey, teamKey), allQuery, "t-mcp-configured", TokenSourceMCP},
		{"default MCP key before the legacy name", "", without(trackerKey, projectKey, teamKey, mcpKey), allQuery, "t-mcp-default", TokenSourceMCP},
		{"legacy MCP name before the former tracker key", "", without(trackerKey, projectKey, teamKey, mcpKey, config.DefaultGitLabTokenKey), allQuery, "t-mcp-legacy", TokenSourceMCP},
		{"former tracker key last", "", map[string]string{DefaultTrackerGitLabTokenKey: "t-tracker-default"}, allQuery, "t-tracker-default", TokenSourceTracker},
		{"nothing found", "", map[string]string{}, allQuery, "", TokenSourceNone},
		{"empty query only reads the hub keys", "", every, GitLabTokenQuery{}, "t-mcp-default", TokenSourceMCP},
		{"team key skipped without team", "", map[string]string{teamKey: "t-team"}, GitLabTokenQuery{}, "", TokenSourceNone},
		{"project key skipped without project", "", map[string]string{projectKey: "t-project"}, GitLabTokenQuery{TeamID: "core"}, "", TokenSourceNone},
		{"another team's key is not used", "", map[string]string{config.TeamGitLabTokenKey("other"): "t-other"}, GitLabTokenQuery{TeamID: "core"}, "", TokenSourceNone},
		{"MCP key equal to the default is read once", "", map[string]string{config.DefaultGitLabTokenKey: "t-mcp-default"}, GitLabTokenQuery{MCPKey: config.DefaultGitLabTokenKey}, "t-mcp-default", TokenSourceMCP},
	}
	for _, tt := range tests {
		for _, store := range []string{"read-only", "read-write"} {
			t.Run(tt.name+"/"+store, func(t *testing.T) {
				t.Setenv(GitLabTokenEnv, tt.env)
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
				tok, src := ResolveGitLabToken(t.Context(), q)
				assert.Equal(t, tt.wantToken, tok)
				assert.Equal(t, tt.wantSource, src)
			})
		}
	}
}

func TestResolveGitLabTokenWithoutKeychain(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	tok, src := ResolveGitLabToken(t.Context(), GitLabTokenQuery{TeamID: "core"})
	assert.Empty(t, tok)
	assert.Equal(t, TokenSourceNone, src)

	t.Setenv(GitLabTokenEnv, "from-env")
	tok, src = ResolveGitLabToken(t.Context(), GitLabTokenQuery{})
	assert.Equal(t, "from-env", tok)
	assert.Equal(t, TokenSourceEnv, src)
}

func TestResolveGitLabTokenKeychainFailureIsAMiss(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	tok, src := ResolveGitLabToken(t.Context(), GitLabTokenQuery{TeamID: "core", TrackerKey: "k", Secrets: failingSecrets{}})
	assert.Empty(t, tok)
	assert.Equal(t, TokenSourceNone, src)
}

func TestResolveGitLabTokenCopiesLegacyMCPKeyOnWritableStore(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	data := getOnlySecrets{config.LegacyMCPTokenKey("gitlab"): "old"}
	tok, src := ResolveGitLabToken(t.Context(), GitLabTokenQuery{Secrets: rwSecrets{data}})
	assert.Equal(t, "old", tok)
	assert.Equal(t, TokenSourceMCP, src)
	assert.Equal(t, "old", data[config.DefaultGitLabTokenKey], "the token is copied under the current name")
}

func TestResolveCredentialsGitLabUsesCascade(t *testing.T) {
	t.Setenv(GitLabTokenEnv, "")
	t.Setenv("GITLAB_URL", "")
	secrets := getOnlySecrets{config.TeamGitLabTokenKey("core"): "team-tok"}

	cfg, err := ResolveCredentials(t.Context(), CredentialSource{TeamID: "core", Secrets: secrets}, TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "team-tok", cfg.Token)
	assert.Equal(t, "https://gitlab.com", cfg.BaseURL)

	_, err = ResolveCredentials(t.Context(), CredentialSource{TeamID: "other", Secrets: secrets}, TypeGitLab)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestResolveCredentialsJiraKeepsItsOwnCascade(t *testing.T) {
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv(GitLabTokenEnv, "gl-env")
	secrets := getOnlySecrets{config.TeamGitLabTokenKey("core"): "team-tok", "jira.key": "jira-tok"}

	cfg, err := ResolveCredentials(t.Context(), CredentialSource{URL: "https://j.example", TokenKey: "jira.key", TeamID: "core", Secrets: secrets}, TypeJira)
	require.NoError(t, err)
	assert.Equal(t, "jira-tok", cfg.Token, "a GitLab token is never used for Jira")

	_, err = ResolveCredentials(t.Context(), CredentialSource{URL: "https://j.example", TeamID: "core", Secrets: secrets}, TypeJira)
	require.Error(t, err)
}
