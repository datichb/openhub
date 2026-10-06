package cmd

import (
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
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
// Each service is resolved independently via mcpresolve.ResolveFull.
func buildMCPServersForProject(a *app.App, mcpConfig *domain.ProjectMCPConfig, resolvedTeam config.ResolvedTeamConfig) []bricks.MCPServerDef {
	// Load team-state shared MCP config (if team is enabled and cloned)
	var sharedMCP map[string]teamstate.SharedMCPConfig
	if resolvedTeam.Enabled && resolvedTeam.StatePath != "" {
		repo := teamstate.NewRepo(resolvedTeam.StateRepo, resolvedTeam.StatePath)
		if repo.IsCloned() {
			if teamCfg, err := repo.LoadConfig(); err == nil {
				sharedMCP = teamCfg.MCP
			}
		}
	}

	// Build project-level override index
	projectSet := make(map[string]*domain.ProjectMCPService)
	if mcpConfig != nil {
		for i := range mcpConfig.Services {
			projectSet[mcpConfig.Services[i].Name] = &mcpConfig.Services[i]
		}
	}

	// Hub-level MCP configs indexed by service name
	hubServices := map[string]config.MCPServerConfig{
		"figma":   a.Config.MCP.Figma,
		"gitlab":  a.Config.MCP.Gitlab,
		"jira":    a.Config.MCP.Jira,
		"gslides": a.Config.MCP.Gslides,
	}

	// Token environment variable fallbacks
	tokenEnvs := map[string]string{
		"figma":   "FIGMA_TOKEN",
		"gitlab":  "GITLAB_TOKEN",
		"jira":    "JIRA_TOKEN",
		"gslides": "GOOGLE_ACCESS_TOKEN",
	}

	teamID := ""
	if resolvedTeam.Enabled {
		teamID = resolvedTeam.TeamID
	}

	// Resolve each service using the full 3-level cascade
	var servers []bricks.MCPServerDef
	for _, name := range []string{"figma", "gitlab", "jira", "gslides"} {
		hub := hubServices[name]
		var shared *teamstate.SharedMCPConfig
		if sharedMCP != nil {
			if s, ok := sharedMCP[name]; ok {
				shared = &s
			}
		}
		project := projectSet[name]

		eff := mcpresolve.ResolveFull(shared, hub, project, teamID)

		servers = append(servers, bricks.MCPServerDef{
			Name:         name,
			Enabled:      eff.Enabled,
			TokenKey:     eff.TokenKey,
			TokenEnv:     tokenEnvs[name],
			WriteEnabled: eff.WriteEnabled,
			URL:          eff.URL,
		})
	}

	// Team MCP server (special — always derived from team config, no project override)
	servers = append(servers, bricks.MCPServerDef{
		Name:    "team",
		Enabled: resolvedTeam.Enabled,
	})

	return servers
}
