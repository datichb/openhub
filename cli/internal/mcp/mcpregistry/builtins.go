package mcpregistry

import (
	"github.com/datichb/openhub/cli/internal/mcp/figma"
	"github.com/datichb/openhub/cli/internal/mcp/github"
	"github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/gslides"
	"github.com/datichb/openhub/cli/internal/mcp/jira"
	"github.com/datichb/openhub/cli/internal/mcp/linear"
	"github.com/datichb/openhub/cli/internal/mcp/team"
)

// NewDefaultRegistry creates a registry with all built-in servers + custom servers loaded.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&figmaServer{})
	r.Register(&githubServer{})
	r.Register(&gitlabServer{})
	r.Register(&gslidesServer{})
	r.Register(&jiraServer{})
	r.Register(&linearServer{})
	r.Register(&teamServer{})
	r.LoadCustomServers()
	return r
}

// ─── Built-in server adapters ───────────────────────────────────────────────

type figmaServer struct{}

func (s *figmaServer) Name() string             { return "figma" }
func (s *figmaServer) Description() string      { return "Figma MCP — lecture des fichiers et composants Figma" }
func (s *figmaServer) RequiredTokens() []string { return []string{"FIGMA_TOKEN"} }
func (s *figmaServer) Serve() error             { return figma.Serve() }

type githubServer struct{}

func (s *githubServer) Name() string             { return "github" }
func (s *githubServer) Description() string      { return "GitHub MCP — issues, PRs, Actions" }
func (s *githubServer) RequiredTokens() []string { return []string{"GITHUB_TOKEN"} }
func (s *githubServer) Serve() error             { return github.Serve() }

type gitlabServer struct{}

func (s *gitlabServer) Name() string             { return "gitlab" }
func (s *gitlabServer) Description() string      { return "GitLab MCP — issues, MRs, pipelines" }
func (s *gitlabServer) RequiredTokens() []string { return []string{"GITLAB_TOKEN"} }
func (s *gitlabServer) Serve() error             { return gitlab.Serve() }

type gslidesServer struct{}

func (s *gslidesServer) Name() string             { return "gslides" }
func (s *gslidesServer) Description() string      { return "Google Slides MCP — lecture des présentations" }
func (s *gslidesServer) RequiredTokens() []string { return []string{"GOOGLE_ACCESS_TOKEN"} }
func (s *gslidesServer) Serve() error             { return gslides.Serve() }

type jiraServer struct{}

func (s *jiraServer) Name() string             { return "jira" }
func (s *jiraServer) Description() string      { return "Jira MCP — issues, projets, transitions (Cloud + Server)" }
func (s *jiraServer) RequiredTokens() []string { return []string{"JIRA_URL", "JIRA_TOKEN"} }
func (s *jiraServer) Serve() error             { return jira.Serve() }

type linearServer struct{}

func (s *linearServer) Name() string             { return "linear" }
func (s *linearServer) Description() string      { return "Linear MCP — issues, cycles, projets (GraphQL API)" }
func (s *linearServer) RequiredTokens() []string { return []string{"LINEAR_API_KEY"} }
func (s *linearServer) Serve() error             { return linear.Serve() }

type teamServer struct{}

func (s *teamServer) Name() string             { return "team" }
func (s *teamServer) Description() string      { return "Team MCP — coordination équipe, wiki, claims, notifications" }
func (s *teamServer) RequiredTokens() []string { return nil } // uses team-state Git repo, no token
func (s *teamServer) Serve() error             { return team.Serve() }
