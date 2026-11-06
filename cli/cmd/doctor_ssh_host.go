package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › known SSH server: the SSH server of each shared team is in
// known_hosts. The team assistants make the user confirm the fingerprint of a
// server met for the first time, but the daemon (heartbeat of the claims,
// automatic sync) and a hub.toml edited by hand reach the server without that:
// the daemon does not connect to a server that known_hosts lacks. Nothing here
// talks to the server (known_hosts is read, `ssh -G` resolves the alias and the
// files, with the options of the ssh command of git), so the check also runs
// with --offline. The detail also says what became of the ssh command the user
// told git to use (GIT_SSH_COMMAND, core.sshCommand, GIT_SSH).

// doctorIDSSHHost is the identifier of the check.
const doctorIDSSHHost = "ssh_host"

func sshHostDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var out []doctor.Result
	for _, t := range sharedTeams(a.Config) {
		h, ok := teamstate.ResolveSSHHost(ctx, t.StateRepo, teamstate.InRepo(t.StatePath))
		if !ok || !h.Resolved {
			continue // not an SSH remote, or ssh cannot say how it resolves the server
		}
		r := doctor.Result{ID: doctorIDSSHHost, Name: i18n.Tf("v54z.doctor.ssh_host.name", firstNonEmpty(t.ID, "hub")), OK: true}
		switch {
		case h.Unchecked:
			r.Detail = i18n.Tf("v54z.doctor.ssh_host.unchecked", h.Display())
		case h.Known():
			r.Detail = i18n.Tf("v54z.doctor.ssh_host.ok", h.Display())
		case h.VouchedByCA():
			r.Detail = i18n.Tf("v54z.doctor.ssh_host.ca", h.Display())
		default:
			sshCmd, _ := h.ManualCommands()
			r.Warn = true
			r.Detail = i18n.Tf("v54z.doctor.ssh_host.unknown", h.Display(), sshCmd)
		}
		if note, warn := h.CustomSSHNote(); note != "" {
			r.Detail += "\n" + note
			r.Warn = r.Warn || warn
		}
		out = append(out, r)
	}
	return out
}
