package team

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Errors of the member profile.
var (
	// ErrNoMemberID: the team of hub.toml has no member_id (oh team rejoin).
	ErrNoMemberID = errors.New("team: no member_id in hub.toml for this team")
	// ErrEmptyDisplayName: a display name cannot be cleared.
	ErrEmptyDisplayName = errors.New("team: the display name cannot be empty")
)

// ProfilePatch is a change of the profile of a member: a nil field is left as
// it is, an empty one is cleared.
type ProfilePatch struct {
	DisplayName        *string
	GitLabUsername     *string
	MattermostUsername *string
	// TrackerUsername is the name of the member on the tracker; it must be set
	// when the tracker is on another server than the team-state repository
	// (the name on one is not the name on the other).
	TrackerUsername *string
}

// Empty reports whether the patch changes nothing.
func (p ProfilePatch) Empty() bool {
	return p.DisplayName == nil && p.GitLabUsername == nil && p.MattermostUsername == nil && p.TrackerUsername == nil
}

// Apply changes m as the patch says (names are trimmed; the « @ » of a
// username typed as on screen is dropped, the servers do not know it).
func (p ProfilePatch) Apply(m *teamstate.Member) error {
	if p.DisplayName != nil {
		name := strings.TrimSpace(*p.DisplayName)
		if name == "" {
			return ErrEmptyDisplayName
		}
		m.DisplayName = name
	}
	if p.GitLabUsername != nil {
		m.GitLabUsername = CleanUsername(*p.GitLabUsername)
	}
	if p.MattermostUsername != nil {
		m.MattermostUsername = strings.TrimSpace(*p.MattermostUsername)
	}
	if p.TrackerUsername != nil {
		m.TrackerUsername = CleanUsername(*p.TrackerUsername)
	}
	return nil
}

// CleanUsername trims a user name of GitLab or Jira and drops the leading « @ »
// (@benjamin.datiche is benjamin.datiche on the server).
func CleanUsername(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), "@")
}

// profileRepo is the local clone of the team teamID ("" = the hub team) with
// the member of hub.toml.
func (s *Service) profileRepo(teamID string) (*teamstate.Repo, string, error) {
	tc := s.ResolveTeam(nil)
	if t := s.Config.FindTeam(teamID); teamID != "" && teamID != tc.TeamID && t != nil {
		tc = config.ResolvedTeamConfig{Enabled: t.Enabled, TeamID: t.ID, StateRepo: t.StateRepo, StatePath: t.StatePath,
			GitLabURL: t.GitLabURL, MemberID: t.MemberID}
		if tc.StatePath == "" && tc.StateRepo != "" {
			tc.StatePath = config.DefaultTeamStatePath()
		}
	} else if teamID != "" && teamID != tc.TeamID {
		return nil, "", ErrTeamNotConfigured
	}
	if !tc.Enabled {
		return nil, "", ErrTeamNotConfigured
	}
	if tc.MemberID == "" {
		return nil, "", ErrNoMemberID
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil, "", ErrNotCloned
	}
	return repo, tc.MemberID, nil
}

// Profile reads the member of the team teamID ("" = the hub team) from the
// local clone of the team-state. It fails with ErrTeamNotConfigured,
// ErrNoMemberID, ErrNotCloned or teamstate.ErrMemberNotFound.
func (s *Service) Profile(teamID string) (*teamstate.Member, error) {
	repo, id, err := s.profileRepo(teamID)
	if err != nil {
		return nil, err
	}
	return repo.GetMember(id)
}

// UpdateProfile changes the profile of the current member in the team-state
// of the team teamID ("" = the hub team): one transaction, one commit on top
// of the remote (teamstate.Repo.PatchMember). It returns the member as written.
func (s *Service) UpdateProfile(ctx context.Context, teamID string, patch ProfilePatch) (*teamstate.Member, error) {
	repo, id, err := s.profileRepo(teamID)
	if err != nil {
		return nil, err
	}
	if err := repo.PatchMember(ctx, id, patch.Apply); err != nil {
		return nil, err
	}
	return repo.GetMember(id)
}

// TrackerIdentity is what the name of the current member on the tracker
// depends on: who they are in the team, where the tracker is relative to the
// team-state repository, and the names the sync maps to members.
type TrackerIdentity struct {
	TeamID   string
	MemberID string
	// Member is the profile of the member; nil when members.toml has none.
	Member *teamstate.Member
	// TrackerType and TrackerURL describe the tracker (the URL the sync calls).
	TrackerType string
	TrackerURL  string
	// TeamServer is the server of the team-state repository.
	TeamServer string
	// CrossServer: both servers are known and are not the same, so the GitLab
	// name of the member is not necessarily their name on the tracker.
	CrossServer bool
	// MemberNames are the names the sync maps to a member (lower case): the
	// tracker_username of each member, else its gitlab_username.
	MemberNames []string
}

// TrackerIdentity reads the identity data of the tracker of the team of
// project (nil: the hub team). It fails like TrackerEngine up to
// ErrTrackerDisabled; it needs no credentials.
func (s *Service) TrackerIdentity(project *domain.Project) (*TrackerIdentity, error) {
	set, err := s.loadTracker(project)
	if err != nil {
		return nil, err
	}
	tc := s.ResolveTeam(project)
	ti := &TrackerIdentity{
		TeamID: tc.TeamID, MemberID: tc.MemberID, TrackerType: set.eff.Type,
		TrackerURL: trackerBaseURL(set.eff), TeamServer: s.TeamRepoServer(project),
	}
	ti.CrossServer = crossServer(ti.TrackerURL, ti.TeamServer)
	if members, err := set.repo.ListMembers(); err == nil {
		for i := range members {
			m := members[i]
			if name := firstNonEmpty(m.TrackerUsername, m.GitLabUsername); name != "" {
				ti.MemberNames = append(ti.MemberNames, strings.ToLower(name))
			}
			if m.ID == tc.MemberID {
				ti.Member = &members[i]
			}
		}
	}
	return ti, nil
}

// HasName reports whether name is the name of a member on the tracker.
func (ti *TrackerIdentity) HasName(name string) bool {
	return slices.Contains(ti.MemberNames, strings.ToLower(strings.TrimSpace(name)))
}

// trackerBaseURL is the URL of the tracker the sync calls.
func trackerBaseURL(eff tracker.EffectiveTrackerConfig) string {
	if eff.Type == string(tracker.TypeJira) {
		return firstNonEmpty(eff.TrackerURL, os.Getenv("JIRA_URL"))
	}
	return tracker.GitLabBaseURL(eff.TrackerURL)
}

// crossServer: both servers are known and are not the same.
func crossServer(trackerURL, teamServer string) bool {
	a, b := tracker.ServerOf(trackerURL), tracker.ServerOf(teamServer)
	return a != "" && b != "" && a != b
}
