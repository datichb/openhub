package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
)

// findHubDir locates the hub directory (~/.oh/hub/).
func findHubDir() string {
	hubDir := hubcontent.HubContentDir()
	if _, err := os.Stat(filepath.Join(hubDir, "agents")); err == nil {
		return hubDir
	}
	return ""
}

// buildMCPServersForProject constructs MCP server definitions using the full
// 3-level cascade: team-state (enforced/recommended) → hub → project.
// Each service is resolved independently via mcpresolve.ResolveFull
// (resolveMCPStates).
func buildMCPServersForProject(ctx context.Context, a *app.App, mcpConfig *domain.ProjectMCPConfig, resolvedTeam config.ResolvedTeamConfig) []bricks.MCPServerDef {
	var servers []bricks.MCPServerDef
	for _, st := range resolveMCPStates(ctx, a.Config, mcpConfig, resolvedTeam) {
		servers = append(servers, bricks.MCPServerDef{
			Name:         st.Service.Name,
			Enabled:      st.Eff.Enabled,
			TokenKey:     st.Eff.TokenKey,
			TokenEnv:     st.Service.TokenEnv,
			WriteEnabled: st.Eff.WriteEnabled,
			URL:          st.Eff.URL,
		})
	}

	// Team MCP server (special — always derived from team config, no project override)
	servers = append(servers, bricks.MCPServerDef{
		Name:    "team",
		Enabled: resolvedTeam.Enabled,
	})

	return servers
}
