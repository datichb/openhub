package cmd

import (
	"context"
	"os"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › MCP services enabled without a token: a service the team or a
// project enables, with no token_key and no token variable, is left out of the
// sessions. The hub's own services are checked by the API keys check
// (mcpTokensReport); this one looks at the levels above it, which that check
// cannot see.

func init() { registerDoctorCheck(mcpDiagnosticDoctorChecks) }

func mcpDiagnosticDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil {
		return nil
	}
	return mcpDiagnosticChecks(ctx, a)
}

// mcpDiagnosticChecks returns one warning per service enabled by a team or a
// project and left without a token, for the default team and for each active
// project. A warning is not a failure: the service is left out, the session
// starts.
func mcpDiagnosticChecks(ctx context.Context, a *app.App) []doctor.Result {
	var out []doctor.Result
	seen := map[string]bool{}
	add := func(scope string, states []mcpState) {
		for _, st := range states {
			if st.Hub.Enabled { // reported by the API keys check
				continue
			}
			d := mcpresolve.DiagnoseToken(st.Service.Name, st.Eff.Enabled, st.Eff.TokenKey, os.Getenv)
			if d == nil || seen[d.Service+"\x00"+scope] {
				continue
			}
			seen[d.Service+"\x00"+scope] = true
			detail := mcpDiagnosticText(*d)
			if scope != "" {
				detail = i18n.Tf("v55m.mcp.diag.scope", scope, detail)
			}
			out = append(out, doctor.Result{
				Name: i18n.Tf("v55m.mcp.diag.name", d.Service), OK: true, Warn: true, Detail: detail,
			})
		}
	}

	hubTeam := defaultTeamConfig(a)
	add("", resolveMCPStates(ctx, a.Config, nil, hubTeam))
	if a.Projects == nil {
		return out
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return out
	}
	for i := range projects {
		p := &projects[i]
		add(p.Name, resolveMCPStates(ctx, a.Config, p.MCPConfig, resolvedTeamConfig(a, p)))
	}
	return out
}
