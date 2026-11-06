package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The git token of a team-state that expired (or was revoked) after the team
// was joined. The daemon used to log a warning at every beat of the claims,
// tried the network twice each time, and told the user nothing. Now the first
// refusal of a repository is one alert in « À traiter » (it names the
// repository and says how to renew the access), and the daemon stops trying
// that repository until the credentials oh holds for it change; an SSH key or
// a repository without a token has no change to observe, it is tried again
// after a while.
//
// accessWatch is the memory of those refusals. It is what any job of the
// daemon that works on a team-state asks: Blocked before the attempt, Report
// with the error of the attempt (true: it was a refusal of the access, the
// job has nothing more to do with it), Accepted when the attempt succeeded.
// The type of the error is teamstate.AuthError (teamstate.IsAuthError,
// teamstate.AccessError), never the words of its message.

// alertTeamAccess is the kind of the alert (daemon.Alert.Kind).
const alertTeamAccess = "team_access"

// accessRetryAfter is how long a refusal is trusted when there is no token to
// watch (an SSH key, a credential helper): the key may have been loaded since.
const accessRetryAfter = 30 * time.Minute

type accessRefusal struct {
	fingerprint string
	retryAt     time.Time
}

type accessWatch struct {
	mu      sync.Mutex
	refused map[string]accessRefusal // by clone path
	// fingerprint tells the credentials oh holds for a remote (""= none: SSH,
	// public repository, credential helper). A change of it is a new chance.
	fingerprint func(ctx context.Context, remote string) string
	// unknownHost tells the SSH server of a remote that known_hosts lacks (nil:
	// teamstate.UnknownSSHHostForJob), and hostsSaid the servers already
	// logged (daemon_ssh_host.go).
	unknownHost func(ctx context.Context, remote string, opts ...teamstate.SSHHostOption) *teamstate.SSHHost
	hostsSaid   map[string]bool
}

func newAccessWatch() *accessWatch {
	return &accessWatch{refused: map[string]accessRefusal{}, fingerprint: credentialFingerprint}
}

// daemonAccess is the memory shared by the jobs of the daemon of this process.
var daemonAccess = newAccessWatch()

// credentialFingerprint identifies the token oh holds for remote without
// keeping it.
func credentialFingerprint(ctx context.Context, remote string) string {
	if teamGitCreds == nil {
		return ""
	}
	c := teamGitCreds.source(ctx, remote)
	if c.Token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(c.Username + "\x00" + c.Token))
	return hex.EncodeToString(sum[:6])
}

// Blocked reports whether the attempts on repo are suspended: it refused the
// access and the credentials are the same since (or, with no token to watch,
// the pause is not over).
func (w *accessWatch) Blocked(ctx context.Context, repo teamstate.RepoInfo, now time.Time) bool {
	w.mu.Lock()
	r, ok := w.refused[repo.Path()]
	w.mu.Unlock()
	if !ok {
		return false
	}
	fp := w.fingerprint(ctx, repo.Remote())
	if fp != r.fingerprint {
		w.Accepted(repo.Path()) // renewed: try again
		return false
	}
	if fp != "" {
		return true // the same token, refused: only a new one can change that
	}
	return now.Before(r.retryAt)
}

// Accepted forgets the refusal of the clone at path: the repository answers.
func (w *accessWatch) Accepted(path string) {
	w.mu.Lock()
	delete(w.refused, path)
	w.mu.Unlock()
}

// Report handles the error of an attempt on repo. It returns true when err is a
// refusal of the access: the first one is logged and raised as an alert under
// the most recent of sessions (the open sessions that work on this team-state),
// the next ones are silent; the attempts are suspended (Blocked).
func (w *accessWatch) Report(ctx context.Context, repo teamstate.RepoInfo, sessions []domain.Session, err error, now time.Time) bool {
	authErr := teamstate.AccessError(err)
	if authErr == nil {
		return false
	}
	path := repo.Path()
	fp := w.fingerprint(ctx, repo.Remote())
	w.mu.Lock()
	prev, known := w.refused[path]
	w.refused[path] = accessRefusal{fingerprint: fp, retryAt: now.Add(accessRetryAfter)}
	w.mu.Unlock()
	if known && prev.fingerprint == fp {
		slog.Debug("ohd: team-state access still refused", "repo", repo.Remote())
		return true
	}
	slog.Warn("ohd: team-state access refused: attempts suspended until the credentials change",
		"repo", repo.Remote(), "error", gitCause(authErr))
	if s := latestSession(sessions); s != nil {
		sum := sha256.Sum256([]byte(repo.Remote()))
		daemon.RaiseAlert(ctx, daemon.Alert{
			SessionID: s.ID,
			Kind:      alertTeamAccess,
			Ref:       hex.EncodeToString(sum[:6]) + ":" + fp,
			Title:     i18n.T("v54a.access.alert.title"),
			Message:   accessAlertMessage(repo.Remote(), authErr),
		})
	}
	return true
}

// accessAlertMessage says which repository refused, why, and the way to renew
// the access.
func accessAlertMessage(remote string, authErr *teamstate.AuthError) string {
	key := "v54a.access.alert.message_ssh"
	if teamstate.IsHTTPS(remote) {
		key = "v54a.access.alert.message_https"
	}
	return i18n.Tf(key, remote, gitCause(authErr), remote)
}

// latestSession is the session that started last (nil for none).
func latestSession(sessions []domain.Session) *domain.Session {
	var best *domain.Session
	for i := range sessions {
		if best == nil || sessions[i].StartedAt.After(best.StartedAt) {
			best = &sessions[i]
		}
	}
	return best
}
