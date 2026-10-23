package team

import (
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// TrackerKind is the tracker of the tickets of a project: its type ("gitlab",
// "jira"; empty when the team declares none) and its project (the GitLab path
// or ID).
type TrackerKind struct {
	Type    string
	Project string
}

// TrackerOf reads the tracker declared for project (its own settings, then
// those of its team, with the hub overrides), without the network and
// whether or not the sync is enabled: where the tickets live does not depend
// on the sync.
func (s *Service) TrackerOf(project *domain.Project) TrackerKind {
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
	return TrackerKind{Type: eff.Type, Project: eff.TrackerProject}
}
