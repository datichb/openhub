package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Doctor › claims under an unknown project ID (K4a, bug 7). The claims of a
// team live in the team-state under projects/<project ID>/claims. A project
// that is created again gets a new ID, and the claims its member took under
// the previous one stay where they were: nothing links them to the project
// any more, and the check of the orphan claims, which reads the claims of the
// IDs it knows, said all was well.
//
// A claim is reported when it belongs to the active member, sits under an ID
// no project of this machine has, and a local project of the same team space
// is that project created again. The team-state does not record the git
// remote of a project, so "the same project" is established from what it does
// keep (rules in claimEvidence):
//
//   - remote: a claim of the ID carries the web URL of its merge request
//     (mr_url, set by oh review --publish), and its host and repository path
//     are those of the origin remote of the local project;
//   - name: the ID was made by domain.NewProjectID (<slug>-<uuid8>) and its
//     slug is the slug of the name of the local project. Refused when an
//     mr_url of the ID names another repository than the origin remote.
//
// With one candidate project (or only one with the "remote" evidence) the
// claims are proposed for attachment: oh doctor --fix moves them, after a
// confirmation, in one commit. With several candidates nothing is proposed.

// Identifiers of the results with a repair or without.
const (
	doctorIDClaimsProject = "claims_project" // claims that --fix can attach
	doctorIDClaimsUnknown = "claims_unknown" // claims under an unknown ID that need a manual step
)

// Evidence that a project is the former ID's project.
const (
	claimEvidenceRemote = "remote"
	claimEvidenceName   = "name"
)

// claimsRehome is the proposal to attach the claims of a member under an
// unknown project ID to a local project.
type claimsRehome struct {
	Repo     *teamstate.Repo
	Member   string
	From     string
	To       domain.Project
	Evidence string
	// Movable are the tickets whose claim moves; Conflicts the tickets that
	// already have a claim under the project (left under the old ID).
	Movable, Conflicts []string
}

// claimsAmbiguity is a former ID with several candidate projects.
type claimsAmbiguity struct {
	From       string
	Candidates []string // names of the projects
	Tickets    []string
}

// remoteOfDir reads the origin remote of the repository at dir.
type remoteOfDir func(dir string) (config.GitRemote, bool)

func gitOriginOf(dir string) (config.GitRemote, bool) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return config.GitRemote{}, false
	}
	gr, err := config.ParseGitRemote(string(out))
	return gr, err == nil
}

// mrRepository is the repository a merge or pull request URL belongs to
// (https://host/group/project/-/merge_requests/12, …/pull/12).
func mrRepository(raw string) (config.GitRemote, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return config.GitRemote{}, false
	}
	path := u.Path
	for _, marker := range []string{"/-/merge_requests", "/merge_requests", "/-/pull", "/pull/", "/pulls/", "/-/"} {
		if i := strings.Index(path, marker); i >= 0 {
			path = path[:i]
			break
		}
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return config.GitRemote{}, false
	}
	return config.GitRemote{Host: strings.ToLower(u.Host), Path: path}, true
}

// sameRepository compares two remotes: same host (port ignored, case ignored)
// and same path (case ignored, .git ignored).
func sameRepository(a, b config.GitRemote) bool {
	host := func(h string) string {
		if i := strings.LastIndexByte(h, ':'); i >= 0 {
			return h[:i]
		}
		return h
	}
	return strings.EqualFold(host(a.Host), host(b.Host)) &&
		strings.EqualFold(strings.TrimSuffix(a.Path, ".git"), strings.TrimSuffix(b.Path, ".git"))
}

// claimEvidence says why project p is the one the claims of the former ID from
// belong to: claimEvidenceRemote, claimEvidenceName, or "" (not the same).
func claimEvidence(from string, claims []teamstate.Claim, p domain.Project, remoteOf remoteOfDir) string {
	var mrRepos []config.GitRemote
	for _, c := range claims {
		if r, ok := mrRepository(c.MRURL); ok {
			mrRepos = append(mrRepos, r)
		}
	}
	if len(mrRepos) > 0 {
		if origin, ok := remoteOf(p.Path); ok {
			for _, r := range mrRepos {
				if sameRepository(r, origin) {
					return claimEvidenceRemote
				}
			}
			return "" // the merge requests are another repository's
		}
	}
	if slug, ok := domain.ProjectIDSlug(from); ok && slug == domain.ProjectSlug(p.Name) {
		return claimEvidenceName
	}
	return ""
}

// matchClaimsRehomes finds, among the claims of member, those under an ID that
// is not in known, and the local project each one is attached to. claimsOf
// lists the claims already under a project (to find the conflicts).
func matchClaimsRehomes(repo *teamstate.Repo, member string, claims []teamstate.Claim, known map[string]bool,
	candidates []domain.Project, claimsOf func(project string) []teamstate.Claim, remoteOf remoteOfDir,
) (found []claimsRehome, ambiguous []claimsAmbiguity) {
	byID := map[string][]teamstate.Claim{}
	var ids []string
	for _, c := range claims {
		if c.ClaimedBy != member || known[c.Project] {
			continue
		}
		if _, seen := byID[c.Project]; !seen {
			ids = append(ids, c.Project)
		}
		byID[c.Project] = append(byID[c.Project], c)
	}
	sort.Strings(ids)
	for _, from := range ids {
		group := byID[from]
		var hits []claimsRehome
		for _, p := range candidates {
			ev := claimEvidence(from, group, p, remoteOf)
			if ev == "" {
				continue
			}
			hits = append(hits, claimsRehome{Repo: repo, Member: member, From: from, To: p, Evidence: ev})
		}
		if len(hits) > 1 {
			var strong []claimsRehome
			for _, h := range hits {
				if h.Evidence == claimEvidenceRemote {
					strong = append(strong, h)
				}
			}
			if len(strong) == 1 {
				hits = strong
			}
		}
		switch len(hits) {
		case 0:
		case 1:
			h := hits[0]
			existing := map[string]bool{}
			for _, c := range claimsOf(h.To.ID) {
				existing[c.TicketID] = true
			}
			for _, c := range group {
				if existing[c.TicketID] {
					h.Conflicts = append(h.Conflicts, c.TicketID)
				} else {
					h.Movable = append(h.Movable, c.TicketID)
				}
			}
			sort.Strings(h.Movable)
			sort.Strings(h.Conflicts)
			found = append(found, h)
		default:
			amb := claimsAmbiguity{From: from}
			for _, h := range hits {
				amb.Candidates = append(amb.Candidates, h.To.Name)
			}
			for _, c := range group {
				amb.Tickets = append(amb.Tickets, c.TicketID)
			}
			sort.Strings(amb.Tickets)
			ambiguous = append(ambiguous, amb)
		}
	}
	return found, ambiguous
}

// findClaimsRehomes looks for the claims under unknown IDs in the team space
// of every active project of the hub.
func findClaimsRehomes(ctx context.Context, a *app.App) ([]claimsRehome, []claimsAmbiguity) {
	if a == nil || a.Projects == nil {
		return nil, nil
	}
	all, err := a.Projects.List(ctx, "")
	if err != nil {
		return nil, nil
	}
	known := map[string]bool{}
	for i := range all {
		known[all[i].ID] = true
	}
	type space struct {
		repo       *teamstate.Repo
		member     string
		candidates []domain.Project
	}
	var order []string
	spaces := map[string]*space{}
	for i := range all {
		p := all[i]
		if p.Status != domain.ProjectStatusActive {
			continue
		}
		repo, member, ok := teamClaimsRepo(a, &p)
		if !ok {
			continue
		}
		key := repo.Path() + "\x00" + member
		s := spaces[key]
		if s == nil {
			s = &space{repo: repo, member: member}
			spaces[key] = s
			order = append(order, key)
		}
		s.candidates = append(s.candidates, p)
	}
	var found []claimsRehome
	var ambiguous []claimsAmbiguity
	for _, key := range order {
		s := spaces[key]
		claims, err := s.repo.ListClaims("")
		if err != nil {
			continue
		}
		claimsOf := func(project string) []teamstate.Claim {
			cs, _ := s.repo.ListClaims(project)
			return cs
		}
		f, amb := matchClaimsRehomes(s.repo, s.member, claims, known, s.candidates, claimsOf, gitOriginOf)
		found = append(found, f...)
		ambiguous = append(ambiguous, amb...)
	}
	return found, ambiguous
}

func (r claimsRehome) describe() string {
	return i18n.Tf("cmd.doctor.claims.rehome_item", r.From, r.To.Name, r.To.ID, len(r.Movable),
		i18n.T("cmd.doctor.claims.evidence_"+r.Evidence))
}

// claimsProjectDetail is the detail of the claims check for the claims under
// unknown IDs: what to attach (with the repair) and what needs a hand.
func claimsProjectDetail(found []claimsRehome, ambiguous []claimsAmbiguity) (detail string, repairable bool) {
	var parts []string
	var movable []string
	for _, r := range found {
		if len(r.Movable) > 0 {
			repairable = true
			movable = append(movable, r.describe())
		}
		for _, t := range r.Conflicts {
			parts = append(parts, i18n.Tf("cmd.doctor.claims.rehome_conflict", t, r.To.Name, t, r.From))
		}
	}
	if len(movable) > 0 {
		parts = append([]string{i18n.Tf("cmd.doctor.claims.rehome", strings.Join(movable, " ; "))}, parts...)
	}
	for _, amb := range ambiguous {
		parts = append(parts, i18n.Tf("cmd.doctor.claims.ambiguous", amb.From, strings.Join(amb.Candidates, ", "), amb.Tickets[0], amb.From))
	}
	return strings.Join(parts, " — "), repairable
}

// planClaimsRehome is the plan of the repair, shown before the confirmation.
func planClaimsRehome(found []claimsRehome) string {
	var lines []string
	for _, r := range found {
		if len(r.Movable) == 0 {
			continue
		}
		lines = append(lines, i18n.Tf("cmd.doctor.claims.rehome_plan", r.From, r.To.Name, r.To.ID, r.Member, strings.Join(r.Movable, ", ")))
		if len(r.Conflicts) > 0 {
			lines = append(lines, "  "+i18n.Tf("cmd.doctor.claims.rehome_plan_conflicts", strings.Join(r.Conflicts, ", ")))
		}
	}
	return strings.Join(lines, "\n")
}

// fixClaimsProject is oh doctor --fix for the claims under an unknown ID,
// after the common confirmation (--yes without a terminal).
func fixClaimsProject(cmd *cobra.Command) error {
	return repairClaimsProject(cmd.Context(), cmd.OutOrStdout(), cmdConfirm(cmd))
}

// repairClaimsProject attaches the claims of the active member under the
// former ID of a project to the project, one commit per team-state, after one
// confirmation. It asks nothing and does nothing when no claim is left to
// attach. It writes its report to w and asks through confirm.
func repairClaimsProject(ctx context.Context, w io.Writer, confirm confirmFunc) error {
	found, _ := findClaimsRehomes(ctx, TryApp())
	var todo []claimsRehome
	for _, r := range found {
		if len(r.Movable) > 0 {
			todo = append(todo, r)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.T("cmd.doctor.claims.rehome_title")))
	fmt.Fprintln(w, planClaimsRehome(todo))
	ok, err := confirm(i18n.T("cmd.doctor.claims.rehome_confirm"))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
		return nil
	}
	byRepo := map[*teamstate.Repo][]claimsRehome{}
	var repos []*teamstate.Repo
	for _, r := range todo {
		if _, seen := byRepo[r.Repo]; !seen {
			repos = append(repos, r.Repo)
		}
		byRepo[r.Repo] = append(byRepo[r.Repo], r)
	}
	moved := 0
	for _, repo := range repos {
		// A team space has one member (the key of findClaimsRehomes).
		group := byRepo[repo]
		moves := make([]teamstate.ClaimMove, len(group))
		for i, r := range group {
			moves[i] = teamstate.ClaimMove{From: r.From, To: r.To.ID}
		}
		res, err := repo.RehomeClaims(ctx, group[0].Member, moves)
		if err != nil {
			if errors.Is(err, teamstate.ErrOffline) {
				return fmt.Errorf("%s", i18n.Tf("cmd.doctor.claims.rehome_offline", err))
			}
			return fmt.Errorf("%s", i18n.Tf("cmd.doctor.claims.rehome_failed", err))
		}
		moved += len(res.Moved)
		for _, c := range res.Conflicts {
			fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.doctor.claims.rehome_conflict", c.TicketID, c.To, c.TicketID, c.From))
		}
	}
	fmt.Fprintf(w, "%s %s\n\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.doctor.claims.rehome_done", moved))
	return nil
}

// planClaimsRehomeFix is the plan shown by the Doctor view.
func planClaimsRehomeFix(ctx context.Context) (string, error) {
	found, _ := findClaimsRehomes(ctx, TryApp())
	if plan := planClaimsRehome(found); plan != "" {
		return plan, nil
	}
	return i18n.T("cmd.doctor.fix.nothing"), nil
}

// claimsProjectFix is the repair of the claims check, for the Doctor view.
func claimsProjectFix(r doctor.Result) *doctor.Fix {
	if r.ID != doctorIDClaimsProject || r.OK {
		return nil
	}
	return &doctor.Fix{Plan: planClaimsRehomeFix, Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
		return repairClaimsProject(ctx, w, alwaysConfirm)
	})}
}
