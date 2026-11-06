package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// An SSH server that known_hosts lacks. The team assistants make the user
// confirm its fingerprint before the first connection; the jobs of the daemon
// (heartbeat of the claims, claims queue, automatic sync) ran git with
// StrictHostKeyChecking=accept-new, which accepts and remembers an unknown
// server the first time it answers: a team added to hub.toml by hand, or one
// whose server key was removed from known_hosts, was trusted in silence.
// A job now asks accessWatch.UnknownHost before it touches a team-state: for a
// server absent from known_hosts it does not connect, the first refusal is a
// warning in the log of the daemon and one alert in « À traiter » (the same
// one whatever the number of beats, even when it was dismissed), and the job
// tries again at its next turn, so that the repository is taken up as soon as
// the user has made the server known (ssh -T git@host).
//
// The check is the one of the assistants, offline (teamstate.UnknownSSHHostForJob:
// known_hosts and `ssh -G`). It is certain enough to stop a job: git runs with
// a GIT_SSH_COMMAND that oh builds, ssh plus the options of the ssh command the
// user told git to use (GIT_SSH_COMMAND, or core.sshCommand of the global
// configuration: key, configuration file, known_hosts; never the one of a
// repository), and `ssh -G` is asked with the same options, so ssh is the one it
// describes; a server that a @cert-authority line covers, or that ssh could not
// be asked about, is not reported. A command oh does not follow is said in the
// log (once) and by `oh doctor`.

// alertTeamHost is the kind of the alert (daemon.Alert.Kind).
const alertTeamHost = "team_host"

// UnknownHost reports whether the SSH server of remote is absent from
// known_hosts: the job must not connect to it. Said once in the log; raised as
// an alert under the most recent of sessions when there is one (the open
// sessions that work on this team-state; none: the next job that has some
// raises it).
func (w *accessWatch) UnknownHost(ctx context.Context, remote string, sessions []domain.Session) bool {
	w.mu.Lock()
	lookup := w.unknownHost
	w.mu.Unlock()
	if lookup == nil {
		lookup = teamstate.UnknownSSHHostForJob
	}
	h := lookup(ctx, remote)
	if h == nil {
		return false
	}
	w.mu.Lock()
	if w.hostsSaid == nil {
		w.hostsSaid = map[string]bool{}
	}
	said := w.hostsSaid[h.Name]
	w.hostsSaid[h.Name] = true
	w.mu.Unlock()
	if !said {
		slog.Warn("ohd: SSH server not in known_hosts: no connection until it is known", "server", h.Display(), "repo", remote)
	}
	if s := latestSession(sessions); s != nil {
		sshCmd, keyscanCmd := h.ManualCommands()
		sum := sha256.Sum256([]byte(remote))
		daemon.RaiseAlert(ctx, daemon.Alert{
			SessionID: s.ID,
			Kind:      alertTeamHost,
			Ref:       hex.EncodeToString(sum[:6]) + ":" + h.Name,
			Title:     i18n.T("v54z.host.alert.title"),
			Message:   i18n.Tf("v54z.host.alert.message", h.Display(), remote, sshCmd, keyscanCmd),
		})
	}
	return true
}
