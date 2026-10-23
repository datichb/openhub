// Package team is the team service: the rules that the command line, the
// terminal UI and the automatic tracker engine all follow, written once.
//
//   - ResolveTeam: which team a project belongs to (a project without team
//     falls back on the hub team);
//   - GitLabToken: which GitLab token is used (one cascade for tracker sync,
//     merge requests, identity check and MCP);
//   - TrackerEngine and SyncTracker: the tracker engine of the team, with
//     typed errors that each caller translates (ErrTeamNotConfigured,
//     ErrNotCloned, ErrTrackerNotConfigured, ErrTrackerDisabled,
//     ErrNoProjects, *CredentialsError);
//   - Init: join a team (clone the team-state, register the member, write
//     hub.toml).
//
// The package builds no message for the user: errors are typed and in
// English, the callers translate them. It knows neither cmd nor the screens.
package team

import (
	"context"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// ProjectLister is what the service needs from the project store.
// domain.ProjectStore implements it.
type ProjectLister interface {
	List(ctx context.Context, status domain.ProjectStatus) ([]domain.Project, error)
}

// Service holds what the team rules read: the hub configuration, the
// keychain and the project store. Build one per operation: it keeps no state.
type Service struct {
	// Config is the hub configuration (hub.toml).
	Config *config.Config
	// Secrets is the keychain; nil leaves only environment variables.
	Secrets tracker.SecretGetter
	// Projects lists the hub projects the tracker is synchronized for; nil
	// means none.
	Projects ProjectLister
	// HubDir is the directory where the tracker engine keeps its state;
	// empty means config.HubDir().
	HubDir string
}

// New returns a service on the hub configuration, the keychain and the
// project store (either may be nil).
func New(cfg *config.Config, secrets tracker.SecretGetter, projects ProjectLister) *Service {
	return &Service{Config: cfg, Secrets: secrets, Projects: projects}
}

// ResolveTeam is the one rule that says which team applies to a project: the
// team the project belongs to when team features are active for it, else the
// hub team (the default team of hub.toml). A nil project is the hub team.
// Enabled is false when neither applies. A team without ID gets the name of
// its repository, which is the name its keychain keys use.
func (s *Service) ResolveTeam(project *domain.Project) config.ResolvedTeamConfig {
	tc := s.resolveTeam(project)
	if tc.TeamID == "" && tc.StateRepo != "" {
		tc.TeamID = config.RepoNameFromRemote(tc.StateRepo)
	}
	if tc.StatePath == "" && tc.StateRepo != "" {
		tc.StatePath = config.DefaultTeamStatePath()
	}
	return tc
}

func (s *Service) resolveTeam(project *domain.Project) config.ResolvedTeamConfig {
	if project != nil {
		if tc := config.ResolveTeamForProject(s.Config, project); tc.Enabled {
			return tc
		}
	}
	hub := s.Config.ActiveTeam()
	return config.ResolvedTeamConfig{
		Enabled:   hub.Enabled,
		TeamID:    hub.ID,
		StateRepo: hub.StateRepo,
		StatePath: hub.StatePath,
		MemberID:  hub.MemberID,
	}
}

// TokenSource names where a GitLab token was found.
type TokenSource = tracker.TokenSource

// GitLabTokenResult is the answer of the GitLab token cascade: the token, its
// source, its key and the sources that were read.
type GitLabTokenResult = tracker.GitLabTokenResult

// TokenOption narrows the GitLab token cascade.
type TokenOption func(*tokenRequest)

type tokenRequest struct {
	q       tracker.GitLabTokenQuery
	project *domain.Project
	// teamRepo replaces the team-state repository of the team (team not
	// registered yet in hub.toml, as during oh team rejoin).
	teamRepo string
}

// WithProject adds the MCP token private to a hub project to the cascade; the
// URL of the GitLab MCP is that of the project.
func WithProject(project *domain.Project) TokenOption {
	return func(r *tokenRequest) {
		r.project = project
		if project != nil {
			r.q.ProjectID = project.ID
		}
	}
}

// WithTrackerToken adds the key set by tracker_token_key to the cascade, with
// the server declared for it (tracker_token_url, "" when none). The former
// tracker key shares that server.
func WithTrackerToken(key, server string) TokenOption {
	return func(r *tokenRequest) { r.q.TrackerKey, r.q.TrackerServer = key, server }
}

// WithTeamRepo gives the team-state repository of the team when the team is
// not (yet) in hub.toml: its host is the server of the token of the team.
func WithTeamRepo(remote string) TokenOption {
	return func(r *tokenRequest) { r.teamRepo = remote }
}

// WithAnyServer lifts the rule that a token is returned only for the server
// called. It only serves to know whether the MCP server can be given a token.
func WithAnyServer() TokenOption {
	return func(r *tokenRequest) { r.q.AnyServer = true }
}

// GitLabToken returns the GitLab token to use for the server (the URL about to
// be called) and where it comes from; the token is empty when no source can
// serve this server. It is the only cascade (tracker.ResolveGitLabToken):
//
//  1. GITLAB_TOKEN, for the server OH_GITLAB_URL (else GITLAB_URL)
//  2. tracker_token_key (WithTrackerToken), for tracker_token_url
//  3. the token of the project (WithProject), for the URL of the GitLab MCP
//  4. the token of the team teamID, for the host of its team-state repository
//  5. the hub MCP token ([mcp.gitlab] token key, then openhub.mcp.gitlab.token),
//     for the URL of the GitLab MCP ([mcp.gitlab] url, project, or team)
//  6. the former tracker key openhub.tracker.gitlab.token, for tracker_token_url
//
// A source whose server is not the one called, or is not declared, is skipped;
// result.Looks tells why, and tracker.NewMissingTokenError turns it into the
// error to show.
func (s *Service) GitLabToken(ctx context.Context, teamID, server string, opts ...TokenOption) GitLabTokenResult {
	r := tokenRequest{q: tracker.GitLabTokenQuery{TeamID: teamID, Server: server, Secrets: s.Secrets}}
	for _, o := range opts {
		o(&r)
	}
	s.completeQuery(&r)
	return tracker.ResolveGitLabToken(ctx, r.q)
}

// completeQuery fills the servers of the sources that come from the hub
// configuration and the team-state.
func (s *Service) completeQuery(r *tokenRequest) {
	q := &r.q
	if s.Config != nil {
		q.MCPKey = s.Config.MCP.Gitlab.Token
	}
	repo, statePath := s.teamRepoOf(q.TeamID, r.project)
	if r.teamRepo != "" {
		repo = r.teamRepo
	}
	q.TeamServer = tracker.ServerOf(repo)
	if s.Config != nil {
		q.MCPServer = tracker.MCPGitLabURL(s.Config.MCP.Gitlab, sharedGitLabMCP(statePath), r.project, q.TeamID)
	}
	q.ProjectServer = q.MCPServer
}

// teamRepoOf returns the team-state repository and clone path of a team: the
// team of the project, else the team of hub.toml with this ID, else the one
// whose repository has this name.
func (s *Service) teamRepoOf(teamID string, project *domain.Project) (remote, statePath string) {
	if s.Config == nil || teamID == "" {
		return "", ""
	}
	if tc := s.ResolveTeam(project); tc.TeamID == teamID {
		return tc.StateRepo, tc.StatePath
	}
	t := s.Config.FindTeam(teamID)
	if t == nil {
		for _, c := range append(append([]config.TeamConfig{}, s.Config.Teams...), s.Config.Team) {
			if c.StateRepo != "" && config.RepoNameFromRemote(c.StateRepo) == teamID {
				t = &c
				break
			}
		}
	}
	if t == nil {
		return "", ""
	}
	if t.StatePath == "" {
		return t.StateRepo, config.DefaultTeamStatePath()
	}
	return t.StateRepo, t.StatePath
}

// sharedGitLabMCP reads the [mcp.gitlab] recommendation of the team-state
// cloned under statePath (nil when it is not cloned or has none).
func sharedGitLabMCP(statePath string) *teamstate.SharedMCPConfig {
	cfg := loadTeamConfig(statePath)
	if cfg == nil {
		return nil
	}
	if shared, ok := cfg.MCP["gitlab"]; ok {
		return &shared
	}
	return nil
}

// GitLabMCPURL is the URL of the GitLab the MCP server of project talks to:
// the server of the MCP tokens ("" when no url is set for it).
func (s *Service) GitLabMCPURL(project *domain.Project) string {
	tc := s.ResolveTeam(project)
	_, statePath := s.teamRepoOf(tc.TeamID, project)
	return tracker.MCPGitLabURL(s.Config.MCP.Gitlab, sharedGitLabMCP(statePath), project, tc.TeamID)
}

// TrackerTokenDecl is the key of the tracker token (tracker_token_key) and the
// server declared for it (tracker_token_url) as the tracker reads them: the
// project, then hub.toml, then the team config, the server being the one set
// at the level of the key. Both are empty when no key is set. It is the pair
// to show, and to edit with config.TrackerLocalConfig.SetTokenDecl.
func (s *Service) TrackerTokenDecl(project *domain.Project) (key, server string) {
	tc := s.ResolveTeam(project)
	var shared *teamstate.TrackerConfig
	if tc.Enabled {
		if cfg := loadTeamConfig(tc.StatePath); cfg != nil {
			shared = &cfg.Tracker
		}
	}
	var pc *domain.ProjectTrackerConfig
	if project != nil {
		pc = project.TrackerConfig
	}
	eff := tracker.ResolveFullTrackerConfig(shared, s.Config.Tracker, pc)
	return eff.TrackerTokenKey, eff.TrackerTokenURL
}

// loadTeamConfig reads the config.toml of the team-state cloned under
// statePath (nil when it is not cloned or unreadable).
func loadTeamConfig(statePath string) *teamstate.TeamConfig {
	if statePath == "" {
		return nil
	}
	repo := teamstate.NewRepo("", statePath)
	if !repo.IsCloned() {
		return nil
	}
	cfg, err := repo.LoadConfig()
	if err != nil {
		return nil
	}
	return cfg
}

// TeamRepoServer is the server of the team-state repository of the team of
// project (its host): where the token of the team is valid. Empty without team.
func (s *Service) TeamRepoServer(project *domain.Project) string {
	return tracker.ServerOf(s.ResolveTeam(project).StateRepo)
}
