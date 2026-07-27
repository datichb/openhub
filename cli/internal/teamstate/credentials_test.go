package teamstate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── isAuthError ───────────────────────────────────────────────────────────────

func TestIsAuthError_TerminalPromptsDisabled(t *testing.T) {
	err := errors.New("git pull --rebase: fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled")
	assert.True(t, isAuthError(err))
}

func TestIsAuthError_AuthenticationFailed(t *testing.T) {
	err := errors.New("git pull: remote: HTTP Basic: Access denied. The provided password or token is incorrect")
	assert.True(t, isAuthError(err))
}

func TestIsAuthError_CouldNotReadUsername(t *testing.T) {
	err := errors.New("fatal: could not read Username for 'https://github.com': No such device or address")
	assert.True(t, isAuthError(err))
}

func TestIsAuthError_FalseForNetworkError(t *testing.T) {
	err := errors.New("git pull: fatal: unable to connect to github.com: connection refused")
	assert.False(t, isAuthError(err), "network errors should not be treated as auth errors")
}

func TestIsAuthError_FalseForRebaseConflict(t *testing.T) {
	err := errors.New("git pull --rebase: error: could not apply abc1234... fix: merge conflict in foo.go")
	assert.False(t, isAuthError(err), "rebase conflicts should not be treated as auth errors")
}

func TestIsAuthError_NilError(t *testing.T) {
	assert.False(t, isAuthError(nil))
}

// ── IsHTTPS ───────────────────────────────────────────────────────────────────

func TestIsHTTPS_HTTPS(t *testing.T) {
	assert.True(t, IsHTTPS("https://gitlab.octo.tools/org/team-state.git"))
}

func TestIsHTTPS_HTTP(t *testing.T) {
	assert.True(t, IsHTTPS("http://gitlab.internal/org/repo.git"))
}

func TestIsHTTPS_SSH_SCP(t *testing.T) {
	assert.False(t, IsHTTPS("git@gitlab.octo.tools:org/team-state.git"))
}

func TestIsHTTPS_SSH_Full(t *testing.T) {
	assert.False(t, IsHTTPS("ssh://git@gitlab.octo.tools/org/team-state.git"))
}

func TestIsHTTPS_Empty(t *testing.T) {
	assert.False(t, IsHTTPS(""))
}

// ── authErrorMessage ──────────────────────────────────────────────────────────

func TestAuthErrorMessage_ContainsRemote(t *testing.T) {
	remote := "https://gitlab.octo.tools/org/team-state.git"
	msg := authErrorMessage(remote)
	assert.Contains(t, msg, remote)
}

func TestAuthErrorMessage_ContainsActionableHints(t *testing.T) {
	msg := authErrorMessage("https://gitlab.octo.tools/org/team-state.git")
	assert.Contains(t, msg, "oauth2", "should mention GitLab oauth2 username")
	assert.Contains(t, msg, "token", "should mention token")
	assert.Contains(t, msg, "team configure", "should tell user how to reconfigure")
}

// ── credentialHelperForPlatform ───────────────────────────────────────────────

func TestCredentialHelperForPlatform_NeverReturnsEmpty(t *testing.T) {
	helper := credentialHelperForPlatform()
	assert.NotEmpty(t, helper, "a credential helper must always be available")
}
