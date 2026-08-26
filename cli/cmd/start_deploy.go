package cmd

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// autoDeployIfNeeded checks whether the project's deployed configuration is
// absent or stale, and auto-deploys if necessary. It always prints a status
// line so the user knows what happened. On error it prints a warning and waits
// for the user to press Enter (unless skipPrompt is true).
func autoDeployIfNeeded(a *app.App, project *domain.Project, hubDir, provider, model string, skipPrompt bool) {
	if hubDir == "" {
		return // hub content not available — nothing to deploy
	}

	out := a.IO.Out

	// CASE 1: Never deployed (no .deploy-state)
	if deploy.ReadDeployState(project.Path) == nil {
		var results []deploy.PhaseResult
		err := progress.Run(
			i18n.T("cmd.start.autodeploy_first"),
			func() error {
				plan := buildDeployPlan(a, project.Path, project.ID, hubDir, provider, model,
					project.Agents, project.ModelOverrides, project.MCPConfig, project.TeamConfig)
				var e error
				results, e = deploy.Execute(plan)
				return e
			},
		)
		if err != nil {
			fmt.Fprintf(out, "  %s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.start.autodeploy_failed", err))
			if !skipPrompt {
				fmt.Fprintf(out, "  %s", i18n.T("cmd.start.autodeploy_continue"))
				fmt.Scanln()
			}
			return
		}
		agentCount, skillCount, mcpCount := countDeployResults(results)
		fmt.Fprintf(out, "  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.autodeploy_done_first", agentCount, skillCount, mcpCount))
		return
	}

	// CASE 2: Already deployed — check staleness
	report, err := deploy.ComputeDiff(hubDir, project.Path, project.Agents)
	if err != nil || !report.HasChanges() {
		// CASE 3: Up to date (or diff error — conservative, no redeploy)
		fmt.Fprintf(out, "  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.T("cmd.start.autodeploy_uptodate"))

		// Context freshness check (informational warning)
		if fr, fErr := deploy.CheckContextFreshness(hubDir, project.Path); fErr == nil && fr != nil && !fr.Fresh {
			staleCount := len(fr.Stale) + len(fr.New)
			fmt.Fprintf(out, "  %s %d fichier(s) hub modifié(s) depuis le dernier deploy — lance %s pour mettre à jour\n",
				theme.WarningStyle.Render(theme.IconWarning),
				staleCount,
				theme.Bold.Render("oh deploy"))
		}
		return
	}

	// CASE 4: Stale — incremental deploy
	added, modified, removed, _ := report.Summary()
	err = progress.Run(
		i18n.Tf("cmd.start.autodeploy_updating", added, modified, removed),
		func() error {
			plan := buildDeployPlan(a, project.Path, project.ID, hubDir, provider, model,
				project.Agents, project.ModelOverrides, project.MCPConfig, project.TeamConfig)
			_, e := deploy.Execute(plan)
			return e
		},
	)
	if err != nil {
		fmt.Fprintf(out, "  %s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.start.autodeploy_failed", err))
		if !skipPrompt {
			fmt.Fprintf(out, "  %s", i18n.T("cmd.start.autodeploy_continue"))
			fmt.Scanln()
		}
		return
	}
	fmt.Fprintf(out, "  %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.start.autodeploy_done_update"))
}

// countDeployResults extracts agent, skill and MCP counts from phase results.
func countDeployResults(results []deploy.PhaseResult) (agents, skills, mcp int) {
	for _, r := range results {
		if !r.Success {
			continue
		}
		switch r.Name {
		case "agents":
			agents = parseCount(r.Message)
		case "skills":
			skills = parseCount(r.Message)
		case "mcp":
			mcp = parseCount(r.Message)
		}
	}
	return
}

// parseCount extracts the first integer from a string like "3 deployed".
func parseCount(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if n > 0 {
			break
		}
	}
	return n
}
