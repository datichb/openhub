package teamstate

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ── Auth detection ────────────────────────────────────────────────────────────

// isAuthError reports whether err contains a git authentication / credential
// failure message. These errors are permanent until the user provides valid
// credentials — they must not be silently downgraded to a PullWarning.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	authPhrases := []string{
		"terminal prompts disabled",
		"authentication failed",
		"could not read username",
		"could not read password",
		"invalid credentials",
		"http basic: access denied",
		"bad credentials",
		"401 unauthorized",
		"403 forbidden",
		"remote: invalid username or password",
	}
	for _, phrase := range authPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}

// IsHTTPS reports whether remote uses the HTTP or HTTPS protocol.
// SSH remotes (git@host:...) return false.
func IsHTTPS(remote string) bool {
	return strings.HasPrefix(remote, "https://") ||
		strings.HasPrefix(remote, "http://")
}

// IsGitLabHost reports whether the remote URL points to a GitLab instance.
// It checks for "gitlab" in the hostname, which covers gitlab.com and
// self-hosted instances (e.g. gitlab.example.com, gitlab.octo.tools).
func IsGitLabHost(remote string) bool {
	u, err := url.Parse(remote)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "gitlab")
}

// ── Credential injection ──────────────────────────────────────────────────────

// ConfigureCredential injects a username/password (or token) into git's
// credential storage for the given remote URL via "git credential approve".
// The credential is handled by whatever credential.helper is configured
// (osxkeychain, store, etc.) — it is NEVER stored in oh's database or config.
//
// For GitLab Personal Access Tokens: username = "oauth2", password = token.
// For GitHub tokens: username = your GitHub username, password = token.
// For Bitbucket: username = "x-token-auth", password = token.
func ConfigureCredential(ctx context.Context, remote, username, password string) error {
	u, err := url.Parse(remote)
	if err != nil {
		return fmt.Errorf("parsing remote URL: %w", err)
	}

	// Format expected by "git credential approve"
	// See: https://git-scm.com/docs/git-credential
	input := fmt.Sprintf("protocol=%s\nhost=%s\nusername=%s\npassword=%s\n\n",
		u.Scheme, u.Host, username, password)

	// Timeout so that a blocking credential helper (e.g. keychain popup that
	// cannot display in a headless subprocess) never hard-freezes the TUI.
	credCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(credCtx, "git", "credential", "approve")
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git credential approve: %s: %w",
			strings.TrimSpace(string(out)), err)
	}
	return nil
}

// authErrorMessage returns a user-facing error message for an authentication
// failure, with actionable remediation steps.
func authErrorMessage(remote string) string {
	return fmt.Sprintf(
		"Authentification échouée pour %s.\n"+
			"Vérifiez que :\n"+
			"  • Le token est valide et non expiré\n"+
			"  • Le token a les permissions read_repository et write_repository\n"+
			"  • Le username est correct (GitLab : 'oauth2', GitHub : votre username)\n"+
			"Reconfigurer : omnibar → 'team configure' → fournir un nouveau token",
		remote,
	)
}
