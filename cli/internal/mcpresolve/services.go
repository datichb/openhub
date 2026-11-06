package mcpresolve

import (
	"slices"
	"strings"
)

// Service is an MCP service oh knows: the one list that `oh mcp setup`,
// `oh mcp status`, `oh doctor`, the session launch and the screens of the
// TUI read, so that a service added here shows up everywhere.
//
// The team server ("team") is not a service in this sense: it has no token and
// no hub.toml section, and is derived from the team configuration.
type Service struct {
	// Name is the identifier of the service ("gitlab"): the key of
	// [mcp.<name>] in hub.toml and of the team config.toml.
	Name string
	// Label is the name as the screens write it ("GitLab").
	Label string
	// TokenEnv is the variable the server reads its token from; the keychain
	// token of the service is put there when the server starts.
	TokenEnv string
	// URLEnv is the variable the server reads its base URL from. Empty: the
	// server always calls the same address (api.figma.com, slides.googleapis.com,
	// api.github.com, api.linear.app), so a URL recorded for the service
	// (hub.toml, project, team) would be ignored and is never offered.
	URLEnv string
}

// catalog is in the order the screens show the services.
var catalog = []Service{
	{Name: "gitlab", Label: "GitLab", TokenEnv: "GITLAB_TOKEN", URLEnv: "GITLAB_URL"},
	{Name: "jira", Label: "Jira", TokenEnv: "JIRA_TOKEN", URLEnv: "JIRA_URL"},
	{Name: "figma", Label: "Figma", TokenEnv: "FIGMA_TOKEN"},
	{Name: "gslides", Label: "Google Slides", TokenEnv: "GOOGLE_ACCESS_TOKEN"},
	{Name: "github", Label: "GitHub", TokenEnv: "GITHUB_TOKEN"},
	{Name: "linear", Label: "Linear", TokenEnv: "LINEAR_API_KEY"},
}

// Services lists the known MCP services, in the order the screens show them.
func Services() []Service { return slices.Clone(catalog) }

// Names lists the names of the known services, sorted.
func Names() []string {
	names := make([]string, len(catalog))
	for i, s := range catalog {
		names[i] = s.Name
	}
	slices.Sort(names)
	return names
}

// Lookup finds a service by name (case and surrounding spaces are ignored).
func Lookup(name string) (Service, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, s := range catalog {
		if s.Name == name {
			return s, true
		}
	}
	return Service{}, false
}

// ServiceHasURL reports whether the server of an MCP service takes a URL.
// The Figma, Google Slides, GitHub and Linear servers always call the same
// address: a URL recorded for them (hub.toml, project, team) is ignored,
// never offered and never reported. A name oh does not know is given the
// benefit of the doubt.
func ServiceHasURL(service string) bool {
	s, ok := Lookup(service)
	return !ok || s.URLEnv != ""
}
