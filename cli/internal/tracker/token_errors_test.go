package tracker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func missingFor(purpose TokenPurpose) *MissingTokenError {
	return &MissingTokenError{
		Server:  "https://gitlab.forge.example",
		Purpose: purpose,
		TeamID:  "core",
		Looks: []TokenLook{
			{Source: TokenSourceEnv, Key: "GITLAB_TOKEN", Outcome: OutcomeEmpty},
			{Source: TokenSourceTrackerKey, Key: "forge.token", Server: "https://gitlab.forge.example", Outcome: OutcomeEmpty},
			{Source: TokenSourceTeam, Key: config.TeamGitLabTokenKey("core"), Server: "git@gitlab.octo.example:acme/core.git", Outcome: OutcomeOtherServer},
			{Source: TokenSourceMCP, Key: config.DefaultGitLabTokenKey, Outcome: OutcomeNoServer},
		},
	}
}

func TestMissingTokenErrorFrench(t *testing.T) {
	i18ntest.Set(t, "fr")
	want := `aucun jeton GitLab pour le serveur gitlab.forge.example
Sources lues :
  - variable d'environnement GITLAB_TOKEN : variable non définie
  - tracker_token_key « forge.token » : aucun jeton enregistré
  - jeton de l'équipe (clé openhub.team.core.gitlab.token) : déclaré pour le serveur gitlab.octo.example, pas pour gitlab.forge.example
  - jeton MCP (clé openhub.mcp.gitlab.token) : jeton présent, mais son serveur n'est pas déclaré
Pour utiliser un jeton avec ce serveur :
  1. stockez-le : oh secrets set <clé>
  2. déclarez la clé : oh config set tracker.tracker_token_key <clé>
  3. déclarez son serveur : oh config set tracker.tracker_token_url https://gitlab.forge.example
(tracker_token_key et tracker_token_url se déclarent aussi dans le [tracker] de la config d'équipe.)
Ou, avec une variable d'environnement : export GITLAB_TOKEN=… OH_GITLAB_URL=https://gitlab.forge.example`
	assert.Equal(t, want, missingFor(PurposeTracker).Error())

	mr := missingFor(PurposeMergeRequest).Error()
	assert.Contains(t, mr, "oh config set mcp.gitlab.token_key <clé>")
	assert.Contains(t, mr, "oh config set mcp.gitlab.url https://gitlab.forge.example")

	team := missingFor(PurposeTeam).Error()
	assert.Contains(t, team, "oh secrets set openhub.team.core.gitlab.token")
}

func TestMissingTokenErrorEnglishHasTheSameParts(t *testing.T) {
	i18ntest.Set(t, "en")
	msg := missingFor(PurposeTracker).Error()
	assert.Contains(t, msg, "no GitLab token for the server gitlab.forge.example")
	assert.Contains(t, msg, "oh config set tracker.tracker_token_url https://gitlab.forge.example")
	assert.Contains(t, msg, "export GITLAB_TOKEN=… OH_GITLAB_URL=https://gitlab.forge.example")
}

func TestTokenRejectedErrorFrench(t *testing.T) {
	i18ntest.Set(t, "fr")
	err := &TokenRejectedError{
		Origin: TokenOrigin{Source: TokenSourceTeam, Key: config.TeamGitLabTokenKey("core"), Server: "https://gitlab.forge.example/"},
		Status: 401,
	}
	assert.Equal(t, "le serveur gitlab.forge.example a refusé le jeton (HTTP 401) — source : jeton de l'équipe (clé openhub.team.core.gitlab.token). Remplacez-le : oh secrets set openhub.team.core.gitlab.token", err.Error())
	assert.ErrorIs(t, err, ErrTokenInvalid, "the sync engine still recognizes a refused token")

	env := &TokenRejectedError{Origin: TokenOrigin{Source: TokenSourceEnv, Key: "GITLAB_TOKEN", Server: "https://gitlab.forge.example"}, Status: 403}
	assert.Contains(t, env.Error(), "variable d'environnement GITLAB_TOKEN")
	assert.Contains(t, env.Error(), "Corrigez la variable GITLAB_TOKEN")

	unknown := &TokenRejectedError{Origin: TokenOrigin{Server: "https://gitlab.forge.example"}, Status: 401}
	assert.Equal(t, "le serveur gitlab.forge.example a refusé le jeton (HTTP 401) — source inconnue", unknown.Error())

	var target *TokenRejectedError
	require.True(t, errors.As(err, &target))
}
