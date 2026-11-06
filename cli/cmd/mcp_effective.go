package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// mcpSource says which level decided whether a service is enabled.
type mcpSource int

const (
	mcpSourceHub mcpSource = iota
	mcpSourceProject
	mcpSourceTeam
)

// mcpState is a service as a scope (the hub, or a project) resolves it: the
// same resolution the sessions use (mcpresolve.ResolveFull), so that what
// oh mcp status, oh doctor and the launch say about a service is what the
// session gets.
type mcpState struct {
	Service mcpresolve.Service
	Hub     config.MCPServerConfig
	Eff     mcpresolve.EffectiveConfig
	Source  mcpSource
}

// resolveMCPStates resolves every known service for a scope: the hub when
// override is nil, a project when it carries one. tc is the team of the scope.
func resolveMCPStates(ctx context.Context, cfg *config.Config, override *domain.ProjectMCPConfig, tc config.ResolvedTeamConfig) []mcpState {
	return resolveMCPStatesWith(cfg, override, tc, sharedMCPForTeam(ctx, tc))
}

func resolveMCPStatesWith(cfg *config.Config, override *domain.ProjectMCPConfig, tc config.ResolvedTeamConfig, sharedMCP map[string]teamstate.SharedMCPConfig) []mcpState {
	projectSet := make(map[string]*domain.ProjectMCPService)
	if override != nil {
		for i := range override.Services {
			projectSet[override.Services[i].Name] = &override.Services[i]
		}
	}
	teamID := ""
	if tc.Enabled {
		teamID = tc.TeamID
	}

	var out []mcpState
	for _, svc := range mcpresolve.Services() {
		var hub config.MCPServerConfig
		if h := cfg.MCPServer(svc.Name); h != nil {
			hub = *h
		}
		var shared *teamstate.SharedMCPConfig
		if s, ok := sharedMCP[svc.Name]; ok {
			shared = &s
		}
		project := projectSet[svc.Name]
		eff := mcpresolve.ResolveFull(shared, hub, project, teamID)

		source := mcpSourceHub
		switch {
		case project != nil:
			source = mcpSourceProject
		case eff.EnabledEnforced, shared != nil && shared.Enabled != nil && !eff.LocalOverridesEnabled:
			source = mcpSourceTeam
		}
		out = append(out, mcpState{Service: svc, Hub: hub, Eff: eff, Source: source})
	}
	return out
}
