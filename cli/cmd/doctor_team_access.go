package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › team-state access: the repository of each shared team answers a read
// with the credentials oh holds (the token of the keychain, the SSH key, the
// helper of the machine). A token that expired after the team was joined used
// to show up as a warning in the log of the daemon and nowhere else. The probe
// is the one of the assistants (a git ls-remote, nothing is written), bounded
// in time, and skipped with --offline.

// doctorIDTeamAccess is the identifier of the check.
const doctorIDTeamAccess = "team_access"

// teamAccessProbe is the probe of a repository (a variable for the tests).
var teamAccessProbe = teamstate.ProbeRemote

func teamAccessDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	teams := sharedTeams(a.Config)
	if len(teams) == 0 {
		return nil
	}
	if doctorOffline() {
		return []doctor.Result{{OK: true, Detail: i18n.T("v54a.doctor.team_access.offline")}}
	}
	out := make([]doctor.Result, 0, len(teams))
	for _, t := range teams {
		out = append(out, teamAccessResult(ctx, t))
	}
	return out
}

// sharedTeams are the teams of hub.toml that have a repository to reach: not
// archived, not solo, one per repository.
func sharedTeams(cfg *config.Config) []config.TeamConfig {
	var out []config.TeamConfig
	for _, t := range append(append([]config.TeamConfig{}, cfg.Teams...), cfg.Team) {
		if !reconfigurable(t) {
			continue
		}
		dup := false
		for _, seen := range out {
			dup = dup || teamstate.SameRemote(seen.StateRepo, t.StateRepo)
		}
		if !dup {
			out = append(out, t)
		}
	}
	return out
}

func teamAccessResult(ctx context.Context, t config.TeamConfig) doctor.Result {
	r := doctor.Result{ID: doctorIDTeamAccess, Name: i18n.Tf("v54a.doctor.team_access.name", firstNonEmpty(t.ID, "hub"))}
	cctx, cancel := context.WithTimeout(ctx, doctorConnectionTimeout)
	defer cancel()
	err := teamAccessProbe(cctx, t.StateRepo)
	switch {
	case err == nil:
		r.OK, r.Detail = true, i18n.Tf("v54a.doctor.team_access.ok", maskRepoURL(t.StateRepo))
		if t.StatePath != "" && !teamstate.NewRepo(t.StateRepo, t.StatePath).IsCloned() {
			r.Detail = i18n.Tf("v54a.doctor.team_access.not_cloned", maskRepoURL(t.StateRepo), t.StateRepo)
		}
	case teamstate.AccessError(err) != nil:
		r.Detail = i18n.Tf("v54a.doctor.team_access.refused", maskRepoURL(t.StateRepo), gitCause(err), t.StateRepo)
	default:
		r.OK, r.Warn = true, true
		r.Detail = i18n.Tf("v54a.doctor.team_access.unreachable", maskRepoURL(t.StateRepo), gitCause(err))
	}
	return r
}
