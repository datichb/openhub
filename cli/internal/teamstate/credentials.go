package teamstate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
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

// sshAuthPhrases are what ssh and the git servers say when the key is not
// accepted (the HTTPS ones are in isAuthError).
var sshAuthPhrases = []string{
	"permission denied (publickey",
	"permission denied, please try again",
	"you don't have permission to access it",
	"returned error: 401",
	"returned error: 403",
}

// AuthError is a failure of the access to the repository, worded for the
// user (authErrorMessage); Cause is what git said.
type AuthError struct {
	Message string
	Cause   error
}

func (e *AuthError) Error() string { return e.Message }
func (e *AuthError) Unwrap() error { return e.Cause }

// IsAuthError reports whether err is a failure of the access to the
// repository, not of the network or of the repository itself: HTTPS
// credentials refused or missing, SSH key not accepted. The wizards offer to
// set the access up when it is true.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	var authErr *AuthError
	if errors.As(err, &authErr) || isAuthError(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, phrase := range sshAuthPhrases {
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

// authErrorMessage returns a user-facing error message for an authentication
// failure, with actionable remediation steps: the token for an HTTPS remote,
// the key of the agent for an SSH one.
func authErrorMessage(remote string) string {
	if IsHTTPS(remote) {
		return i18n.Tf("v532w.auth.https", remote)
	}
	return i18n.Tf("v532w.auth.ssh", remote)
}

// remoteProbeTimeout bounds the probe of a remote.
const remoteProbeTimeout = 10 * time.Second

// ProbeRemote reads remote the way a clone would, without writing anything
// (`git ls-remote` with every prompt disabled): with what git already has
// (credential helper, SSH agent, netrc, a public repository) and the token oh
// holds for it (SetCredentialSource). It returns nil when the repository
// answers, otherwise the error of git (IsAuthError tells an access problem
// from the rest).
func ProbeRemote(ctx context.Context, remote string) error {
	return probeRemote(ctx, remote, nil)
}

// ProbeRemoteAs is ProbeRemote with the credentials given instead of the ones
// oh holds: a token is checked before it is kept.
func ProbeRemoteAs(ctx context.Context, remote string, creds GitCredentials) error {
	return probeRemote(ctx, remote, &creds)
}

func probeRemote(ctx context.Context, remote string, creds *GitCredentials) error {
	if strings.TrimSpace(remote) == "" {
		return errors.New("no repository")
	}
	pctx, cancel := context.WithTimeout(ctx, remoteProbeTimeout)
	defer cancel()
	cmd, cleanup := gitCommand(pctx, remote, "", creds, "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=5",
		[]string{"GIT_HTTP_LOW_SPEED_LIMIT=1000", "GIT_HTTP_LOW_SPEED_TIME=8"},
		"ls-remote", "--", remote, "HEAD")
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if pctx.Err() != nil && ctx.Err() == nil {
			return fmt.Errorf("git ls-remote: no answer from %s in %s", remote, remoteProbeTimeout)
		}
		return fmt.Errorf("git ls-remote: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoteAccessible reports whether remote answers a read with what git
// already has and the token oh holds for it (ProbeRemote). It is how the
// HTTPS steps of the wizards know that no token has to be asked.
func RemoteAccessible(ctx context.Context, remote string) bool {
	return ProbeRemote(ctx, remote) == nil
}
