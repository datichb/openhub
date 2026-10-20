package cmd

import (
	"context"
	"slices"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › MCP URLs imposed by the team (audit v5, S-31): a URL the team
// imposes on a service is used only once the member pinned it; the check
// warns about the ones that are not, and says how to pin them.

func init() { registerDoctorCheck(mcpURLDoctorChecks) }

func mcpURLDoctorChecks(ctx context.Context) []views.DoctorCheck {
	a := TryApp()
	if a == nil {
		return nil
	}
	teams := []config.ResolvedTeamConfig{resolvedTeamConfig(a, nil)}
	if a.Projects != nil {
		if projects, err := a.Projects.List(ctx, domain.ProjectStatusActive); err == nil {
			for i := range projects {
				teams = append(teams, resolvedTeamConfig(a, &projects[i]))
			}
		}
	}
	return mcpURLChecks(a.Config, teams, sharedMCPForTeam)
}

// mcpURLChecks returns one warning per URL imposed and not used, over the
// teams of the hub and of its projects (one warning per team and service).
func mcpURLChecks(cfg *config.Config, teams []config.ResolvedTeamConfig, load func(config.ResolvedTeamConfig) map[string]teamstate.SharedMCPConfig) []views.DoctorCheck {
	var out []views.DoctorCheck
	var seen []string
	for _, tc := range teams {
		if slices.Contains(seen, tc.TeamID) {
			continue
		}
		seen = append(seen, tc.TeamID)
		for _, n := range enforcedURLNotices(cfg, load(tc)) {
			out = append(out, views.DoctorCheck{
				Name: i18n.Tf("cmd.doctor.mcp_url.name", n.Service), OK: true, Warn: true, Detail: n.Message(),
			})
		}
	}
	return out
}
