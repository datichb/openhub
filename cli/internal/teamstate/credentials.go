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

// The refusals of a server are recognised by their words, in one place: git
// and ssh say them in English (the network commands run with LC_ALL=C,
// gitCommand), whatever the language of the machine or of oh. Past this file
// the access is a type: *AuthError.

// authPhrases are what git says when the credentials are missing or refused
// (HTTPS).
var authPhrases = []string{
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

// sshAuthPhrases are what ssh and the git servers say when the key is not
// accepted, or the access is read-only.
var sshAuthPhrases = []string{
	"permission denied (publickey",
	"permission denied, please try again",
	"you don't have permission to access it",
	"you are not allowed to push",
	"returned error: 401",
	"returned error: 403",
}

// gitLabNotFoundPhrases are what a GitLab says for a repository the token
// cannot see: it answers 404, not 403, so as not to tell that it exists.
var gitLabNotFoundPhrases = []string{
	"the project you were looking for could not be found",
	"returned error: 404",
}

// isAuthError reports whether err contains a git authentication / credential
// failure message (HTTPS). These errors are permanent until the user provides
// valid credentials.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(strings.ToLower(err.Error()), authPhrases)
}

func containsAny(msg string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}

// isGitLabNotFound: a GitLab host says the repository is not found. On a
// GitLab that is probably an access problem (a token without the right, a
// private project), not a wrong URL.
func isGitLabNotFound(remote string, err error) bool {
	if err == nil || !IsHTTPS(remote) || !IsGitLabHost(remote) {
		return false
	}
	msg := strings.ToLower(err.Error())
	if containsAny(msg, gitLabNotFoundPhrases) {
		return true
	}
	return strings.Contains(msg, "repository '") && strings.Contains(msg, "' not found")
}

// AuthError is a failure of the access to the repository, worded for the
// user (authErrorMessage); Cause is what git said. It is what every
// operation of the team-state returns when the server refuses the access
// (clone, pull, push, probe): HTTPS credentials missing, expired or without
// the right to write, SSH key not accepted. The callers tell it with
// IsAuthError or errors.As, never by the words of the message.
type AuthError struct {
	Message string
	// Remote is the repository whose access was refused.
	Remote string
	Cause  error
}

func (e *AuthError) Error() string { return e.Message }
func (e *AuthError) Unwrap() error { return e.Cause }

// accessError is the *AuthError that err is, nil when err is not a refusal of
// the access (the network, a conflict, a missing repository). An error that
// already is one is returned as it is.
func accessError(remote string, err error) *AuthError {
	if err == nil {
		return nil
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr
	}
	msg := strings.ToLower(err.Error())
	if containsAny(msg, authPhrases) || containsAny(msg, sshAuthPhrases) || isGitLabNotFound(remote, err) {
		return &AuthError{Message: authErrorMessage(remote), Remote: remote, Cause: err}
	}
	return nil
}

// asAccessError returns the access error that err is, else err itself.
func asAccessError(remote string, err error) error {
	if authErr := accessError(remote, err); authErr != nil {
		return authErr
	}
	return err
}

// IsAuthError reports whether err is a failure of the access to the
// repository, not of the network or of the repository itself: HTTPS
// credentials refused or missing, SSH key not accepted. The wizards offer to
// set the access up when it is true, the daemon raises its alert.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return containsAny(msg, authPhrases) || containsAny(msg, sshAuthPhrases)
}

// AccessError returns the *AuthError inside err (nil when there is none).
func AccessError(err error) *AuthError {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr
	}
	return nil
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
// answers; a refusal of the access is an *AuthError (a GitLab that does not
// find the repository included: it says so to a token that cannot see it),
// anything else the error of git.
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
	cmd, cleanup := gitCommand(pctx, remote, "", creds, probeSSHOptions,
		probeHTTPEnv,
		"ls-remote", "--", remote, "HEAD")
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return probeError(pctx, ctx, remote, "ls-remote", out, err)
	}
	return nil
}

const probeSSHOptions = "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=5"

var probeHTTPEnv = []string{"GIT_HTTP_LOW_SPEED_LIMIT=1000", "GIT_HTTP_LOW_SPEED_TIME=8"}

// probeError is the error of a probe that failed: an *AuthError for a refusal
// of the access, a timeout in plain words, else what git said.
func probeError(pctx, ctx context.Context, remote, op string, out []byte, err error) error {
	if pctx.Err() != nil && ctx.Err() == nil {
		return fmt.Errorf("git %s: no answer from %s in %s", op, remote, remoteProbeTimeout)
	}
	return asAccessError(remote, fmt.Errorf("git %s: %s: %w", op, strings.TrimSpace(string(out)), err))
}

// RemoteAccessible reports whether remote answers a read with what git
// already has and the token oh holds for it (ProbeRemote). It is how the
// HTTPS steps of the wizards know that no token has to be asked.
func RemoteAccessible(ctx context.Context, remote string) bool {
	return ProbeRemote(ctx, remote) == nil
}
