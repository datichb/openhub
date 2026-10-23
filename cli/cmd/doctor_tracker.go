package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Doctor › tracker (K4b, end of bug 6): oh doctor stayed green while the
// tracker could not be reached with the token of the configuration. Two
// checks, over the team of the hub and the team of each active project:
//
//   - tracker_tokens: a GitLab token is declared for the server of the tracker
//     (a failure) and for the server of the team-state repository (a warning:
//     the token only verifies the identity at oh team rejoin), by the cascade of
//     tracker.ResolveGitLabToken, which only answers for the server called;
//     without one, the message of the cascade says the server, the sources read
//     and the commands to declare it. No network.
//   - tracker_connection: the connection test of oh team config (GitLab
//     /api/v4/user, Jira /rest/api/2/myself), bounded in time and skipped with
//     --offline / OH_DOCTOR_OFFLINE. A server that refuses the token (401, 403)
//     or does not know the API (404) is a failure that names the server, the
//     source and the key; a network error is a warning.

// Two more checks look at who the member is on the tracker (bug 12): the sync
// attributes the issues of the tracker to the members by name, and a tracker on
// another server than the team-state repository does not know the GitLab name
// of the member.
//
//   - tracker_username: when the tracker is not on the server of the team-state
//     repository, the tracker_username of the current member must be set (a
//     failure otherwise: none of their issues is attributed to them). No
//     network.
//   - tracker_identity: the name the tracker returns for the token (GitLab
//     /api/v4/user, Jira /myself) must be the name of a member (tracker_username,
//     else gitlab_username); a warning otherwise, with the command. Online,
//     skipped with --offline, bounded in time like tracker_connection; a tracker
//     that does not answer is the business of tracker_connection.

// Identifiers of the tracker checks.
const (
	doctorIDTrackerTokens     = "tracker_tokens"
	doctorIDTrackerConnection = "tracker_connection"
	doctorIDTrackerUsername   = "tracker_username"
	doctorIDTrackerIdentity   = "tracker_identity"
)

// doctorConnectionTimeout bounds the connection test of each tracker.
var doctorConnectionTimeout = 5 * time.Second

// notGitLabHosts are the code hosts that are never a GitLab: a team-state
// repository there needs no GitLab token.
var notGitLabHosts = []string{"github.com", "bitbucket.org"}

// trackerProbe is the team (and project) a tracker check looks at, with the
// credentials of its tracker or why there are none.
type trackerProbe struct {
	label   string
	teamID  string
	project *domain.Project
	creds   tracker.Config
	// err is the *CredentialsError of a tracker configured without a usable token.
	err error
}

// trackerProbes lists the trackers of the hub team and of the teams of the
// active projects; teams without a tracker (or not cloned, or disabled) are
// left out, and so are the projects that share the tracker and the token of an
// earlier one.
func trackerProbes(ctx context.Context, a *app.App) []trackerProbe {
	svc := teamService(a)
	var out []trackerProbe
	seen := map[string]bool{}
	add := func(project *domain.Project, label string) {
		tc := svc.ResolveTeam(project)
		if !tc.Enabled {
			return
		}
		creds, err := svc.TrackerCredentials(ctx, project)
		var credErr *teamsvc.CredentialsError
		if err != nil && !errors.As(err, &credErr) {
			return // no tracker to look at: not cloned, not configured, disabled…
		}
		key := tc.TeamID + "|"
		if err != nil {
			key += err.Error()
		} else {
			key += string(creds.Type) + "|" + creds.BaseURL + "|" + string(creds.Origin.Source) + "|" + creds.Origin.Key
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, trackerProbe{label: label, teamID: tc.TeamID, project: project, creds: creds, err: err})
	}
	add(nil, teamLabel(svc, nil))
	for _, p := range activeProjects(ctx, a) {
		add(&p, teamLabel(svc, &p)+" / "+p.Name)
	}
	return out
}

func teamLabel(svc *teamsvc.Service, project *domain.Project) string {
	if id := svc.ResolveTeam(project).TeamID; id != "" {
		return id
	}
	return "hub"
}

func activeProjects(ctx context.Context, a *app.App) []domain.Project {
	if a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil
	}
	return projects
}

// trackerTokensChecks reports, per team, the GitLab token of the tracker and
// the one of the team-state repository.
func trackerTokensChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	svc := teamService(a)
	var out []doctor.Result
	failedServers := map[string]bool{}
	for _, p := range trackerProbes(ctx, a) {
		r := doctor.Result{Name: i18n.Tf("cmd.doctor.tokens.tracker.name", p.label)}
		switch {
		case p.err != nil:
			r.Detail = p.err.Error()
			failedServers[p.teamID+"|"+tracker.ServerOf(trackerServerOfError(p.err))] = true
		case p.creds.Type == tracker.TypeGitLab:
			r.OK, r.Detail = true, p.creds.Origin.DescribeWithServer()
		default:
			r.OK, r.Detail = true, i18n.Tf("cmd.doctor.tokens.other_tracker", p.creds.Type, p.creds.BaseURL)
		}
		out = append(out, r)
	}
	seenTeams := map[string]bool{}
	add := func(project *domain.Project, label string) {
		tc := svc.ResolveTeam(project)
		server := svc.TeamRepoServer(project)
		if !tc.Enabled || server == "" || seenTeams[tc.TeamID+"|"+server] || isNotGitLab(server) {
			return
		}
		seenTeams[tc.TeamID+"|"+server] = true
		if failedServers[tc.TeamID+"|"+server] {
			return // the same server already failed for the tracker: said once
		}
		key, keyServer, implicit := svc.TrackerTokenDecl(project)
		res := svc.GitLabToken(ctx, tc.TeamID, server,
			teamsvc.WithProject(project), teamsvc.WithTrackerToken(key, keyServer, implicit))
		r := doctor.Result{Name: i18n.Tf("cmd.doctor.tokens.team.name", label)}
		if res.Found() {
			r.OK, r.Detail = true, res.Origin().DescribeWithServer()
		} else {
			// A warning: without it oh team rejoin only skips the identity check.
			r.OK, r.Warn = true, true
			r.Detail = tracker.NewMissingTokenError(res, tracker.PurposeTeam, tc.TeamID).Error() +
				"\n" + i18n.T("cmd.doctor.tokens.team.purpose")
		}
		out = append(out, r)
	}
	add(nil, teamLabel(svc, nil))
	for _, p := range activeProjects(ctx, a) {
		add(&p, teamLabel(svc, &p)+" / "+p.Name)
	}
	return out
}

// trackerServerOfError is the server a missing-token error is about.
func trackerServerOfError(err error) string {
	var m *tracker.MissingTokenError
	if errors.As(err, &m) {
		return m.Server
	}
	return ""
}

func isNotGitLab(server string) bool {
	host, _, _ := strings.Cut(server, ":")
	return slices.Contains(notGitLabHosts, host)
}

// trackerConnectionChecks tests the connection to the tracker of each team.
func trackerConnectionChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var probes []trackerProbe
	for _, p := range trackerProbes(ctx, a) {
		if p.err == nil { // without a token the tokens check already failed
			probes = append(probes, p)
		}
	}
	if len(probes) == 0 {
		return nil
	}
	if doctorOffline() {
		return []doctor.Result{{OK: true, Detail: i18n.T("cmd.doctor.tracker_conn.offline")}}
	}
	out := make([]doctor.Result, 0, len(probes))
	for _, p := range probes {
		out = append(out, trackerConnectionResult(ctx, p))
	}
	return out
}

func trackerConnectionResult(ctx context.Context, p trackerProbe) doctor.Result {
	r := doctor.Result{Name: i18n.Tf("cmd.doctor.tracker_conn.name", p.label)}
	t, err := tracker.New(p.creds)
	if err != nil {
		r.Detail = i18n.Tf("cmd.doctor.tracker_conn.init_failed", p.creds.Type, err)
		return r
	}
	cctx, cancel := context.WithTimeout(ctx, doctorConnectionTimeout)
	defer cancel()
	username, err := t.TestConnection(cctx)
	server := tracker.ServerOf(p.creds.BaseURL)
	var rejected *tracker.TokenRejectedError
	switch {
	case err == nil:
		r.OK, r.Detail = true, i18n.Tf("cmd.doctor.tracker_conn.ok", server, username)
	case errors.As(err, &rejected):
		r.Detail = rejected.Error()
	case errors.Is(err, tracker.ErrIssueNotFound):
		r.Detail = i18n.Tf("cmd.doctor.tracker_conn.not_found", server)
	default:
		r.OK, r.Warn = true, true
		r.Detail = i18n.Tf("cmd.doctor.tracker_conn.unreachable", server, err)
	}
	return r
}

// trackerUsernameChecks fails when the tracker is not on the server of the
// team-state repository and the current member has no tracker_username.
func trackerUsernameChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	svc := teamService(a)
	var out []doctor.Result
	for _, p := range trackerProbes(ctx, a) {
		ti, err := svc.TrackerIdentity(p.project)
		if err != nil || ti.Member == nil || !ti.CrossServer {
			continue // on the same server, the GitLab name of the member is their tracker name
		}
		r := doctor.Result{Name: i18n.Tf("cmd.doctor.tracker_username.name", p.label)}
		if ti.Member.TrackerUsername == "" {
			r.Detail = i18n.Tf("cmd.doctor.tracker_username.empty", ti.MemberID,
				tracker.ServerOf(ti.TrackerURL), tracker.ServerOf(ti.TeamServer), ti.TeamID)
		} else {
			r.OK, r.Detail = true, i18n.Tf("cmd.doctor.tracker_username.ok", ti.MemberID, ti.Member.TrackerUsername)
		}
		out = append(out, r)
	}
	return out
}

// trackerIdentityChecks compares the name the tracker knows the token's owner
// by with the names of the members.
func trackerIdentityChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var probes []trackerProbe
	for _, p := range trackerProbes(ctx, a) {
		if p.err == nil {
			probes = append(probes, p)
		}
	}
	if len(probes) == 0 {
		return nil
	}
	if doctorOffline() {
		return []doctor.Result{{OK: true, Detail: i18n.T("cmd.doctor.tracker_identity.offline")}}
	}
	svc := teamService(a)
	var out []doctor.Result
	for _, p := range probes {
		ti, err := svc.TrackerIdentity(p.project)
		if err != nil {
			continue
		}
		t, err := tracker.New(p.creds)
		if err != nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, doctorConnectionTimeout)
		username, err := t.TestConnection(cctx)
		cancel()
		if err != nil || username == "" {
			continue // tracker_connection reports it
		}
		r := doctor.Result{Name: i18n.Tf("cmd.doctor.tracker_identity.name", p.label), OK: true}
		server := tracker.ServerOf(p.creds.BaseURL)
		if ti.HasName(username) {
			r.Detail = i18n.Tf("cmd.doctor.tracker_identity.ok", server, username)
		} else {
			r.Warn = true
			r.Detail = i18n.Tf("cmd.doctor.tracker_identity.unknown", server, username, ti.TeamID)
		}
		out = append(out, r)
	}
	return out
}
