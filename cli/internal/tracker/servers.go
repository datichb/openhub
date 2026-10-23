package tracker

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// MCPGitLabURL is the URL of the GitLab the MCP server of a project talks to,
// resolved like the MCP server does (team enforcement pinned by the member,
// project, hub, team recommendation). Empty when none of them sets one: the
// MCP tokens then have no declared server.
func MCPGitLabURL(hub config.MCPServerConfig, shared *teamstate.SharedMCPConfig, project *domain.Project, teamID string) string {
	var ps *domain.ProjectMCPService
	if project != nil && project.MCPConfig != nil {
		for i := range project.MCPConfig.Services {
			if project.MCPConfig.Services[i].Name == "gitlab" {
				ps = &project.MCPConfig.Services[i]
			}
		}
	}
	return mcpresolve.ResolveFull(shared, hub, ps, teamID).URL
}

// MCPServerOf is the server of the MCP tokens given the url of the GitLab MCP:
// that url, else gitlab.com (where the MCP server goes with no url), in which
// case implicit is true.
func MCPServerOf(url string) (server string, implicit bool) {
	if url != "" {
		return url, false
	}
	return DefaultMCPGitLabServer, true
}
