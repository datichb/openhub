package opencodev2

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// MCPHookPath is the hook route of the oh MCP servers served over HTTP by
// the oh daemon, under the proxy listeners (credproxy.HooksPrefix).
const MCPHookPath = "mcp/"

// remoteOhMCP declares the oh MCP servers of a bundle as remote servers
// served by the oh daemon on the machine (P4-T08), for a tool server that
// runs outside the machine (container): `oh` and the service tokens stay on
// the machine. The tool authenticates with the session token it already
// holds, read from its environment (`{env:…}`), never written in the config.
// Other local servers are left as is (they run in the runtime).
func remoteOhMCP(b sessionspec.BundleSpec, p sessionspec.ProviderSpec) sessionspec.BundleSpec {
	if len(b.MCP) == 0 {
		return b
	}
	base, env := mcpHookBase(p), providerTokenEnv(p)
	out := b
	out.MCP = make([]sessionspec.MCPServerDef, len(b.MCP))
	for i, m := range b.MCP {
		out.MCP[i] = m
		name, ok := sessionspec.OhMCPName(m)
		if !ok {
			if m.Type != "remote" {
				slog.Warn("opencodev2: local MCP server left in the runtime", "server", m.Name)
			}
			continue
		}
		if base == "" || env == "" {
			slog.Warn("opencodev2: oh MCP server unavailable outside the machine (no proxy)", "server", m.Name)
			continue
		}
		out.MCP[i] = sessionspec.MCPServerDef{Name: m.Name, Type: "remote", URL: base + url.PathEscape(name),
			Headers: map[string]string{"Authorization": "Bearer {env:" + env + "}"}}
	}
	return out
}

func mcpHookBase(p sessionspec.ProviderSpec) string {
	u, err := url.Parse(p.BaseURL)
	if p.BaseURL == "" || err != nil || u.Host == "" {
		return ""
	}
	u.Path, u.RawPath, u.RawQuery = strings.TrimSuffix(credproxy.HooksPrefix, "/")+"/"+MCPHookPath, "", ""
	return u.String()
}
