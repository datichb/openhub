package team

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Errors of the tracker engine. The callers translate them (the command line
// prints a message, the screen a toast, the automatic engine skips).
var (
	// ErrTeamNotConfigured: no team with team features active.
	ErrTeamNotConfigured = errors.New("team: no team configured")
	// ErrNotCloned: the team-state of the team is not cloned.
	ErrNotCloned = errors.New("team: the team-state is not cloned")
	// ErrTrackerNotConfigured: the team config has no [tracker] type.
	ErrTrackerNotConfigured = errors.New("team: no tracker configured in the team config")
	// ErrTrackerDisabled: [tracker] enabled = false in hub.toml or in the
	// team config.
	ErrTrackerDisabled = errors.New("team: the tracker is disabled (enabled = false)")
	// ErrNoProjects: no hub project is linked to a tracker project.
	ErrNoProjects = errors.New("team: no hub project is linked to a tracker project")
)

// TrackerInitError is a failure to build the tracker client.
type TrackerInitError struct{ Err error }

func (e *TrackerInitError) Error() string {
	return "team: cannot initialize the tracker: " + e.Err.Error()
}
func (e *TrackerInitError) Unwrap() error { return e.Err }

// ConfigError is a failure to read the config.toml of the team-state.
type ConfigError struct{ Err error }

func (e *ConfigError) Error() string { return "team: cannot read the team config: " + e.Err.Error() }
func (e *ConfigError) Unwrap() error { return e.Err }

// CredentialsError is a failure to find or build the tracker credentials.
type CredentialsError struct {
	// TrackerType is the tracker type ("gitlab", "jira").
	TrackerType string
	// EnvVar is the environment variable that would provide the token.
	EnvVar string
	Err    error
}

func (e *CredentialsError) Error() string { return e.Err.Error() }
func (e *CredentialsError) Unwrap() error { return e.Err }

// built is a tracker engine and what it was built from.
type built struct {
	engine      *tracker.Engine
	repo        *teamstate.Repo
	trackerType string
	projects    map[string]string
}

// TrackerEngine builds the tracker engine of the team of project (nil: the
// hub team). It fails with ErrTeamNotConfigured, ErrNotCloned,
// *ConfigError, ErrTrackerNotConfigured, ErrTrackerDisabled,
// *CredentialsError or *TrackerInitError. An engine without any linked
// project is still returned: it serves ticket details and assignments.
func (s *Service) TrackerEngine(ctx context.Context, project *domain.Project) (*tracker.Engine, error) {
	b, err := s.build(ctx, project)
	if err != nil {
		return nil, err
	}
	return b.engine, nil
}

// SyncOptions tune SyncTracker.
type SyncOptions struct {
	// Project selects the team (nil: the hub team).
	Project *domain.Project
	// Pull updates the local team-state before the sync (a failing pull is
	// ignored: the local content is used).
	Pull bool
	// OnRun, if set, is called when the sync is about to start with the
	// tracker type; the function it returns is called when the sync is over
	// (progress display).
	OnRun func(trackerType string) (done func())
}

// SyncTracker synchronizes the tracker of the team with its team-state. On
// top of the errors of TrackerEngine it fails with ErrNoProjects when no hub
// project is linked to a tracker project.
func (s *Service) SyncTracker(ctx context.Context, opts SyncOptions) (*tracker.SyncResult, error) {
	b, err := s.build(ctx, opts.Project)
	if err != nil {
		return nil, err
	}
	if len(b.projects) == 0 {
		return nil, ErrNoProjects
	}
	if opts.Pull {
		if err := b.repo.Pull(ctx); err != nil {
			slog.Debug("team.sync_tracker.pull_failed", "error", err)
		}
	}
	if opts.OnRun != nil {
		if done := opts.OnRun(b.trackerType); done != nil {
			defer done()
		}
	}
	return b.engine.Run(ctx)
}

// trackerSetup is what the tracker of the team of project is built from: the
// team-state, its config and the effective tracker config (team-state merged
// with hub.toml).
type trackerSetup struct {
	repo *teamstate.Repo
	eff  tracker.EffectiveTrackerConfig
}

// loadTracker reads the tracker config of the team of project. It fails like
// TrackerEngine does, up to ErrTrackerDisabled.
func (s *Service) loadTracker(project *domain.Project) (*trackerSetup, error) {
	tc := s.ResolveTeam(project)
	if !tc.Enabled {
		return nil, ErrTeamNotConfigured
	}
	statePath := tc.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, ErrNotCloned
	}
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return nil, &ConfigError{Err: err}
	}
	if teamCfg.Tracker.Type == "" {
		return nil, ErrTrackerNotConfigured
	}

	// Shared team-state config merged with the local hub.toml overrides.
	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, s.Config.Tracker)
	if !eff.Enabled {
		return nil, ErrTrackerDisabled
	}
	return &trackerSetup{repo: repo, eff: eff}, nil
}

// TrackerCredentials resolves the connection config of the tracker of the
// team of project (URL, token and where the token comes from), without
// building the engine. It fails like TrackerEngine up to *CredentialsError,
// whose error names the server and the sources read when no token serves the
// tracker.
func (s *Service) TrackerCredentials(ctx context.Context, project *domain.Project) (tracker.Config, error) {
	set, err := s.loadTracker(project)
	if err != nil {
		return tracker.Config{}, err
	}
	return s.credentials(ctx, set.eff, project)
}

func (s *Service) credentials(ctx context.Context, eff tracker.EffectiveTrackerConfig, project *domain.Project) (tracker.Config, error) {
	trackerType := tracker.Type(eff.Type)
	creds, err := tracker.ResolveCredentials(ctx, s.CredentialSource(tracker.NewCredentialSource(eff, nil), project), trackerType)
	if err != nil {
		return tracker.Config{}, &CredentialsError{TrackerType: eff.Type, EnvVar: envVar(trackerType), Err: err}
	}
	return creds, nil
}

func (s *Service) build(ctx context.Context, project *domain.Project) (*built, error) {
	set, err := s.loadTracker(project)
	if err != nil {
		return nil, err
	}
	repo, eff := set.repo, set.eff
	creds, err := s.credentials(ctx, eff, project)
	if err != nil {
		return nil, err
	}
	t, err := tracker.New(creds)
	if err != nil {
		return nil, &TrackerInitError{Err: err}
	}

	projects, patterns := s.trackerProjects(ctx, eff)
	slog.Debug("team.tracker_engine.ready", "type", eff.Type, "projects", len(projects))

	hubDir := s.HubDir
	if hubDir == "" {
		hubDir = config.HubDir()
	}
	engine := tracker.NewEngine(t, repo, engineConfig(eff, projects, patterns), hubDir)
	tc := s.ResolveTeam(project)
	engine.SetIdentity(tracker.Identity{
		TeamID: tc.TeamID, MemberID: tc.MemberID,
		CrossServer: crossServer(trackerBaseURL(eff), s.TeamRepoServer(project)),
	})
	return &built{
		engine:      engine,
		repo:        repo,
		trackerType: eff.Type,
		projects:    projects,
	}, nil
}

// CredentialSource completes src with what the services know: the keychain,
// the team of project (nil: the hub team) with the server of its team-state
// repository, and the key and url of [mcp.gitlab], so that the tracker finds
// its GitLab token with the unified cascade, which only answers for the
// server of the tracker.
func (s *Service) CredentialSource(src tracker.CredentialSource, project *domain.Project) tracker.CredentialSource {
	src.Secrets = s.Secrets
	r := tokenRequest{project: project, q: tracker.GitLabTokenQuery{TeamID: s.ResolveTeam(project).TeamID}}
	s.completeQuery(&r)
	src.TeamID = r.q.TeamID
	src.TeamServer = r.q.TeamServer
	src.MCPTokenKey = r.q.MCPKey
	src.MCPServer, src.MCPImplicit = r.q.MCPServer, r.q.MCPImplicit
	return src
}

// envVar is the environment variable that holds the token of a tracker.
func envVar(t tracker.Type) string {
	switch t {
	case tracker.TypeGitLab:
		return tracker.GitLabTokenEnv
	case tracker.TypeJira:
		return "JIRA_TOKEN"
	}
	return "TOKEN"
}

func engineConfig(eff tracker.EffectiveTrackerConfig, projects, patterns map[string]string) teamstate.TrackerConfig {
	return teamstate.TrackerConfig{
		Type:                 eff.Type,
		Enabled:              eff.Enabled,
		AutoSync:             eff.AutoSync,
		SyncIntervalMinutes:  eff.SyncIntervalMinutes,
		AutoPlanAssigned:     eff.AutoPlanAssigned,
		MaxAutoPlanPerMember: eff.MaxAutoPlanPerMember,
		AutoPlanUnassigned:   eff.AutoPlanUnassigned,
		UnassignedLabels:     eff.UnassignedLabels,
		MaxUnassignedIssues:  eff.MaxUnassignedIssues,
		PushLabels:           eff.PushLabels,
		TicketPatterns:       patterns, //nolint:staticcheck // legacy tracker fields still read by the sync engine (backward compatibility)
		Projects:             projects, //nolint:staticcheck // legacy tracker fields still read by the sync engine (backward compatibility)
		StatusMapping:        eff.StatusMapping,
		LabelStatusMapping:   eff.LabelStatusMapping,
	}
}

// trackerProjects builds the Projects and TicketPatterns maps of the engine
// from the hub projects. For each active project it resolves the tracker
// project and the ticket pattern with the cascade project override, then
// team default, then the old maps (which win when they have entries).
func (s *Service) trackerProjects(ctx context.Context, eff tracker.EffectiveTrackerConfig) (projects, patterns map[string]string) {
	projects = eff.Projects
	if projects == nil {
		projects = make(map[string]string)
	}
	patterns = eff.TicketPatterns
	if patterns == nil {
		patterns = make(map[string]string)
	}

	if len(projects) > 0 {
		if eff.TicketPattern != "" {
			for k := range projects {
				if patterns[k] == "" {
					patterns[k] = eff.TicketPattern
				}
			}
		}
		return projects, patterns
	}
	if eff.TrackerProject == "" || s.Projects == nil {
		return projects, patterns
	}
	all, err := s.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil || len(all) == 0 {
		return projects, patterns
	}
	for _, p := range all {
		trackerProject := eff.TrackerProject
		if p.TrackerConfig != nil && p.TrackerConfig.TrackerProject != "" {
			trackerProject = p.TrackerConfig.TrackerProject
		}
		projects[p.ID] = trackerProject

		pattern := eff.TicketPattern
		if p.TrackerConfig != nil && p.TrackerConfig.TicketPattern != "" {
			pattern = p.TrackerConfig.TicketPattern
		}
		if pattern != "" {
			patterns[p.ID] = pattern
		}
	}
	slog.Debug("team.tracker_projects", "count", len(projects), "projects", projects)
	return projects, patterns
}
