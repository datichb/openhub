package cmd

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Doctor › life cycle of the claims and of the members of each team of this
// machine. Unlike the checks of the claims of the active member
// (doctor_claims*.go), these look at the whole team-state of each enabled team
// that has a clone, and say only what is wrong (warnings, never a failure):
//
//   - claims_daemon: the daemon is stopped while claims of open sessions need
//     its heartbeat;
//   - claims_sync: the tracker of the team was never synchronized from this
//     machine, or not for long;
//   - claims_board_status: claims in a status that is not a column of the board;
//   - claims_duplicates: two claims for one issue of the tracker;
//   - claims_gone_members: claims held by a member who has no card any more;
//   - member_removed: the member of hub.toml has no card in the team any more.

// Identifiers of the results.
const (
	doctorIDClaimsDaemon      = "claims_daemon"
	doctorIDClaimsSync        = "claims_sync"
	doctorIDClaimsBoardStatus = "claims_board_status"
	doctorIDClaimsDuplicates  = "claims_duplicates"
	doctorIDClaimsGone        = "claims_gone_members"
	doctorIDMemberRemoved     = "member_removed"
)

// A sync older than this many intervals is reported when the team synchronizes
// by itself; without auto_sync a sync is expected from time to time only.
const (
	syncStaleIntervals = 3
	syncStaleManual    = 14 * 24 * time.Hour
)

func init() { registerDoctorCheck(teamLifecycleDoctorChecks) }

// lifecycleTeam is a team of hub.toml with a clone, as the checks read it.
type lifecycleTeam struct {
	Team    config.TeamConfig
	Repo    lifecycleStore
	Cfg     *teamstate.TeamConfig
	Claims  []teamstate.Claim
	Members map[string]bool
	// MembersKnown: members.toml could be read (an unreadable card file says
	// nothing about who is gone).
	MembersKnown bool
}

// lifecycleStore is what the checks read of a team-state.
type lifecycleStore interface {
	teamstate.RepoInfo
	teamstate.ClaimReader
	teamstate.ConfigReader
	teamstate.MemberReader
}

func (t lifecycleTeam) name() string { return t.Team.DisplayName() }

func teamLifecycleDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	return teamLifecycleResults(ctx, a, time.Now())
}

// readLifecycleTeams reads the enabled teams that have a clone.
func readLifecycleTeams(ctx context.Context, a *app.App) []lifecycleTeam {
	var out []lifecycleTeam
	for _, team := range a.Config.Teams {
		if !team.Enabled || team.Solo || team.StateRepo == "" {
			continue
		}
		repo := teamstate.NewRepo(team.StateRepo, team.Resolved().StatePath)
		if !repo.IsCloned() {
			continue
		}
		t := lifecycleTeam{Team: team, Repo: repo}
		t.Cfg, _ = repo.LoadConfig(ctx)
		t.Claims, _ = repo.ListClaims(ctx, "")
		if members, err := repo.ListMembers(ctx); err == nil {
			t.MembersKnown = true
			t.Members = make(map[string]bool, len(members))
			for _, m := range members {
				t.Members[m.ID] = true
			}
		}
		out = append(out, t)
	}
	return out
}

func teamLifecycleResults(ctx context.Context, a *app.App, now time.Time) []doctor.Result {
	teams := readLifecycleTeams(ctx, a)
	if len(teams) == 0 {
		return nil
	}
	var out []doctor.Result
	out = append(out, daemonClaimsResult(ctx, a, now)...)
	out = append(out, syncAgeResults(ctx, a, teams, now)...)
	trackerOf := projectTrackerOf(ctx, a)
	for _, t := range teams {
		out = append(out, boardStatusResult(t)...)
		out = append(out, duplicateClaimsResult(t, trackerOf)...)
		out = append(out, goneMembersResult(t)...)
		out = append(out, removedMemberResult(t)...)
	}
	return out
}

func warnResult(id, nameKey, detail string) doctor.Result {
	return doctor.Result{ID: id, Name: i18n.T(nameKey), OK: true, Warn: true, Detail: detail}
}

// ─── Daemon stopped with claims of open sessions ────────────────────────────

// daemonClaimsResult warns when claims of open sessions wait for a daemon that
// is not running: nothing renews their lease, and they expire.
func daemonClaimsResult(ctx context.Context, a *app.App, now time.Time) []doctor.Result {
	if a.Projects == nil || a.Sessions == nil {
		return nil
	}
	b := newClaimsBeater(a)
	open := b.openSessions(ctx)
	if len(open) == 0 {
		return nil
	}
	count := 0
	for _, t := range b.teams(ctx) {
		cfg, _ := t.repo.LoadConfig(ctx)
		var claimCfg teamstate.ClaimConfig
		if cfg != nil {
			claimCfg = cfg.Claim
		}
		count += len(t.refsOf(open, claimCfg, now))
	}
	if count == 0 || daemonRunning(ctx) {
		return nil
	}
	return []doctor.Result{warnResult(doctorIDClaimsDaemon, "v54l.doctor.daemon.name", i18n.Tf("v54l.doctor.daemon.found", count, len(open)))}
}

// ─── Tracker sync never made, or old ─────────────────────────────────────────

// syncFinding says what is wrong with the last sync of a tracker, "" when
// nothing: last is the time of the last sync from this machine (zero: never),
// auto and interval the automatic sync of the team.
func syncFinding(last time.Time, auto bool, interval time.Duration, now time.Time) (key string, age time.Duration) {
	if last.IsZero() {
		return "never", 0
	}
	limit := syncStaleManual
	if auto {
		if interval <= 0 {
			interval = tracker.DefaultSyncInterval
		}
		limit = syncStaleIntervals * interval
	}
	if age = now.Sub(last); age > limit {
		return "old", age
	}
	return "", 0
}

func syncAgeResults(ctx context.Context, a *app.App, teams []lifecycleTeam, now time.Time) []doctor.Result {
	state, err := tracker.LoadSyncState(config.HubDir())
	if err != nil {
		return nil
	}
	attached := map[string]bool{}
	if a.Projects != nil {
		if projects, err := a.Projects.List(ctx, domain.ProjectStatusActive); err == nil {
			for _, p := range projects {
				if p.TeamID != nil {
					attached[*p.TeamID] = true
				}
			}
		}
	}
	var out []doctor.Result
	for _, t := range teams {
		if t.Cfg == nil || t.Cfg.Tracker.Type == "" || !attached[t.Team.ID] {
			continue
		}
		eff := tracker.ResolveTrackerConfig(&t.Cfg.Tracker, a.Config.TrackerFor(t.Team.ID))
		if !eff.Enabled {
			continue
		}
		var last time.Time
		for key, at := range state.LastSyncAt {
			if strings.HasPrefix(key, eff.Type+"/") && at.After(last) {
				last = at
			}
		}
		interval := time.Duration(eff.SyncIntervalMinutes) * time.Minute
		switch kind, age := syncFinding(last, eff.AutoSync, interval, now); kind {
		case "never":
			out = append(out, warnResult(doctorIDClaimsSync, "v54l.doctor.sync.name",
				i18n.Tf("v54l.doctor.sync.never", t.name(), "oh team sync-tracker --team "+t.Team.ID)))
		case "old":
			out = append(out, warnResult(doctorIDClaimsSync, "v54l.doctor.sync.name",
				i18n.Tf("v54l.doctor.sync.old", t.name(), roundAge(age), "oh team sync-tracker --team "+t.Team.ID)))
		}
	}
	return out
}

// roundAge writes an age as « 3 jours » / « 5 h ».
func roundAge(d time.Duration) string {
	if d >= 48*time.Hour {
		return i18n.Tf("v54l.doctor.age.days", int(d.Hours()/24))
	}
	if d >= time.Hour {
		return i18n.Tf("v54l.doctor.age.hours", int(d.Hours()))
	}
	return i18n.Tf("v54l.doctor.age.minutes", int(d.Minutes()))
}

// ─── Claims in a status the board does not have ──────────────────────────────

// claimsOutsideBoard are the claims whose status is none of the statuses of the
// board (a column removed or renamed): they appear in no column.
func claimsOutsideBoard(claims []teamstate.Claim, board teamstate.BoardConfig) []teamstate.Claim {
	statuses := board.AllStatuses()
	var out []teamstate.Claim
	for _, c := range claims {
		if !teamstate.IsValidBoardStatus(c.Status, statuses) {
			out = append(out, c)
		}
	}
	return out
}

func boardStatusResult(t lifecycleTeam) []doctor.Result {
	var board teamstate.BoardConfig
	if t.Cfg != nil {
		board = t.Cfg.Board
	}
	outside := claimsOutsideBoard(t.Claims, board)
	if len(outside) == 0 {
		return nil
	}
	parts := make([]string, 0, len(outside))
	for _, c := range outside {
		parts = append(parts, fmt.Sprintf("%s/%s (%s)", c.Project, c.TicketID, c.Status))
	}
	return []doctor.Result{warnResult(doctorIDClaimsBoardStatus, "v54l.doctor.board.name",
		i18n.Tf("v54l.doctor.board.found", t.name(), len(outside), shortList(parts)))}
}

// ─── Two claims for one issue ────────────────────────────────────────────────

// claimDuplicate is the claims that stand for one issue of the tracker.
type claimDuplicate struct {
	IID    int
	Claims []string // project/ticket
}

// duplicateClaims finds the issues that more than one claim stands for: in one
// project (SRU-42 and 42), or in several projects of the team that are linked
// to the same tracker project (trackerOf gives the tracker project of a hub
// project directory, "" when it does not know).
func duplicateClaims(claims []teamstate.Claim, trackerOf func(project string) string) []claimDuplicate {
	type key struct {
		tracker string
		iid     int
	}
	groups := map[key][]string{}
	for _, c := range claims {
		if c.ExternalIID == 0 {
			continue
		}
		scope := trackerOf(c.Project)
		if scope == "" {
			scope = "project:" + c.Project // no known tracker project: duplicates inside the project only
		}
		k := key{tracker: scope, iid: c.ExternalIID}
		groups[k] = append(groups[k], c.Project+"/"+c.TicketID)
	}
	var out []claimDuplicate
	for k, refs := range groups {
		if len(refs) > 1 {
			slices.Sort(refs)
			out = append(out, claimDuplicate{IID: k.iid, Claims: refs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Claims[0] < out[j].Claims[0] })
	return out
}

func duplicateClaimsResult(t lifecycleTeam, trackerOf func(teamID, project string) string) []doctor.Result {
	dups := duplicateClaims(t.Claims, func(project string) string { return trackerOf(t.Team.ID, project) })
	if len(dups) == 0 {
		return nil
	}
	parts := make([]string, len(dups))
	for i, d := range dups {
		parts[i] = fmt.Sprintf("#%d : %s", d.IID, strings.Join(d.Claims, " = "))
	}
	return []doctor.Result{warnResult(doctorIDClaimsDuplicates, "v54l.doctor.duplicates.name",
		i18n.Tf("v54l.doctor.duplicates.found", t.name(), len(dups), shortList(parts)))}
}

// ─── Claims of members who are gone ──────────────────────────────────────────

// goneMemberClaims counts the open claims per member who has no card. The
// finished claims are left to the cleanup of the done claims.
func goneMemberClaims(claims []teamstate.Claim, members map[string]bool, cfg *teamstate.TeamConfig) map[string]int {
	var board teamstate.BoardConfig
	if cfg != nil {
		board = cfg.Board
	}
	terminals := board.TerminalStatuses()
	out := map[string]int{}
	for _, c := range claims {
		if c.ClaimedBy == "" || members[c.ClaimedBy] || slices.Contains(terminals, c.Status) {
			continue
		}
		out[c.ClaimedBy]++
	}
	return out
}

func goneMembersResult(t lifecycleTeam) []doctor.Result {
	if !t.MembersKnown {
		return nil
	}
	gone := goneMemberClaims(t.Claims, t.Members, t.Cfg)
	if len(gone) == 0 {
		return nil
	}
	ids := make([]string, 0, len(gone))
	for id := range gone {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%s (%d)", id, gone[id])
	}
	return []doctor.Result{warnResult(doctorIDClaimsGone, "v54l.doctor.gone.name",
		i18n.Tf("v54l.doctor.gone.found", t.name(), strings.Join(parts, ", "),
			"oh team claim transfer --from "+ids[0]+" --to <"+i18n.T("v54l.doctor.member")+"> --team "+t.Team.ID,
			"oh team claim transfer --from "+ids[0]+" --release --team "+t.Team.ID))}
}

// ─── The member of hub.toml has no card any more ─────────────────────────────

func removedMemberResult(t lifecycleTeam) []doctor.Result {
	if !t.MembersKnown || t.Team.MemberID == "" || t.Members[t.Team.MemberID] {
		return nil
	}
	return []doctor.Result{warnResult(doctorIDMemberRemoved, "v54l.doctor.removed.name",
		i18n.Tf("v54l.doctor.removed.found", t.name(), t.Team.MemberID,
			"oh team rejoin --repo "+t.Team.StateRepo+" --new-member",
			"oh teams leave "+t.Team.ID))}
}

// projectTrackerOf gives, for a hub project directory of a team, the tracker
// project it is linked to ("" when unknown): the override of the project, else
// the project of the team.
func projectTrackerOf(ctx context.Context, a *app.App) func(teamID, project string) string {
	byID := map[string]*domain.Project{}
	if a.Projects != nil {
		if projects, err := a.Projects.List(ctx, domain.ProjectStatusActive); err == nil {
			for i := range projects {
				byID[projects[i].ID] = &projects[i]
			}
		}
	}
	cache := map[string]*teamstate.TeamConfig{}
	return func(teamID, project string) string {
		p := byID[project]
		cfg, ok := cache[teamID]
		if !ok {
			cfg = nil
			if team := a.Config.FindTeam(teamID); team != nil {
				cfg, _ = teamstate.NewRepo(team.StateRepo, team.Resolved().StatePath).LoadConfig(ctx)
			}
			cache[teamID] = cfg
		}
		if cfg == nil {
			return ""
		}
		var override *domain.ProjectTrackerConfig
		if p != nil {
			override = p.TrackerConfig
		}
		eff := tracker.ResolveFullTrackerConfig(&cfg.Tracker, a.Config.TrackerFor(teamID), override)
		if eff.TrackerProject != "" {
			return teamID + "|" + eff.TrackerProject
		}
		if tp := eff.Projects[project]; tp != "" {
			return teamID + "|" + tp
		}
		return ""
	}
}
