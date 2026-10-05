package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/termlaunch"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func init() {
	launcher.V5Launch = v5Launch
}

var (
	v5Once    sync.Once
	v5Adapter *opencodev2.Adapter
	v5Err     error
)

// v5Available reports whether the v5 runtime can be used (opencode V2
// installed). OH_V5=0 forces the legacy pipeline.
func v5Available(ctx context.Context) bool {
	if os.Getenv("OH_V5") == "0" {
		return false
	}
	v5Once.Do(func() { v5Adapter, v5Err = detectV2Adapter(ctx) })
	if v5Err != nil {
		slog.Debug("v5 runtime unavailable, using legacy launch", "reason", v5Err)
	}
	return v5Err == nil
}

// v5Launch runs a session on the v5 runtime: compile the session bundle,
// start (or join) the tool server through the oh daemon, create the session
// and open its client in a new terminal tab/window.
func v5Launch(ctx context.Context, a *app.App, ui launcher.LaunchUI, opts launcher.LaunchOpts) (bool, error) {
	if !v5Available(ctx) {
		return false, nil
	}
	if opts.ProjectID == "" || a.Projects == nil {
		return false, nil
	}
	project, err := a.Projects.Get(ctx, opts.ProjectID)
	if err != nil {
		return true, fmt.Errorf("loading project: %w", err)
	}
	location := opts.ProjectPath
	if location == "" {
		location = project.Path
	}
	entry := opts.Agent
	if entry == "" {
		entry = "orchestrator"
	}

	prov := provider.ResolveProvider(opts.Provider, project.Provider, a.Config.Opencode.DefaultProvider)
	var projProv *provider.ProviderConfig
	tokenKey := ""
	if project.ProviderConfig != nil {
		projProv = &provider.ProviderConfig{AWSProfile: project.ProviderConfig.AWSProfile, AWSRegion: project.ProviderConfig.AWSRegion}
		tokenKey = project.ProviderConfig.TokenKey
	}
	provCfg := provider.ResolveProviderConfig(projProv, hubProviderCfg(a, prov))
	team := config.ResolveTeamForProject(a.Config, project)

	ui.Notify(i18n.Tf("cmd.v5.preparing", entry), launcher.LevelInfo)
	b, err := buildSessionBundle(a, project, team, entry, prov)
	if err != nil {
		return true, fmt.Errorf("building session bundle: %w", err)
	}

	svc, err := newRunService(ctx, a)
	if err != nil {
		return true, err
	}

	req := runsvc.StartRequest{
		ProjectID: project.ID, ProjectTokenKey: tokenKey, Location: location, Bundle: b, EntryAgent: entry,
		Title: sessionTitle(project, entry), Prompt: opts.Prompt, WorkflowID: entry,
		Provider: prov, ProviderCfg: provCfg,
		Attach: sessionspec.AttachPref(attachPreference(a)), ITermStyle: termlaunch.ITermStyle(a.Config.Session.ITermStyle),
	}
	if team.Enabled {
		req.TeamID = team.TeamID
		if team.MemberID != "" {
			mid := team.MemberID
			req.MemberID = &mid
		}
	}
	res, err := svc.StartSession(ctx, req)
	if err != nil {
		return true, err
	}
	for _, w := range res.Report.Warnings {
		ui.Notify(i18n.Tf("cmd.v5.isolation_warning", w), launcher.LevelWarning)
	}

	switch {
	case req.Attach == sessionspec.AttachBrowser:
		url, err := svc.PairURL(ctx, res.SessionID, pairURL)
		if err != nil {
			return true, err
		}
		ui.Notify(i18n.Tf("cmd.v5.opened_browser", url), launcher.LevelSuccess)
		return true, openURL(url)
	case req.Attach == sessionspec.AttachSuspend || res.AttachErr != nil:
		if res.AttachErr != nil {
			ui.Notify(i18n.T("cmd.session.no_terminal"), launcher.LevelWarning)
		}
		return true, runAttachInline(ctx, svc, ui, res.SessionID)
	}
	ui.Notify(i18n.Tf("cmd.v5.opened_in", string(res.AttachMethod), res.SessionID), launcher.LevelSuccess)
	return true, nil
}

func attachPreference(a *app.App) string {
	if v := os.Getenv("OH_SESSION_ATTACH"); v != "" {
		return v
	}
	if a.Config.Session.Attach != "" {
		return a.Config.Session.Attach
	}
	return string(sessionspec.AttachAuto)
}

func sessionTitle(p *domain.Project, entry string) string {
	return p.Name + " · " + entry
}

// runAttachInline runs the tool client in the current terminal (TUI suspended).
func runAttachInline(ctx context.Context, svc *runsvc.Service, ui launcher.LaunchUI, sessionID string) error {
	argv, env, err := svc.AttachCommand(ctx, sessionID)
	if err != nil {
		return err
	}
	run := func() error {
		c := exec.CommandContext(ctx, argv[0], argv[1:]...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		c.Env = append(os.Environ(), env...)
		return c.Run()
	}
	if suspend := ui.SuspendAndExec(); suspend != nil {
		return suspend(run)
	}
	return run()
}

// buildSessionBundle compiles the phase 0 bundle for an entry agent.
func buildSessionBundle(a *app.App, project *domain.Project, team config.ResolvedTeamConfig, entry, prov string) (*bundle.Bundle, error) {
	wf, err := deploy.ResolveAndPrepareWorkflow(workflow.BaseWorkflow(), collectWorkflowOverrides(a, project, team)...)
	if err != nil {
		slog.Warn("workflow resolution failed, using base workflow", "error", err)
		wf, _ = deploy.ResolveAndPrepareWorkflow(workflow.BaseWorkflow())
	}
	hubOv, projOv := modelOverridesFor(a, project)
	return bundle.Build(bundle.Request{
		HubDir: hubcontent.HubContentDir(), OutDir: ohBundlesDir(), ProjectPath: project.Path, EntryAgent: entry,
		Workflow: wf, Provider: prov, ProjectOverrides: projOv, HubOverrides: hubOv,
		ExtraInstructionFiles: a.Config.Deploy.InstructionFiles,
		MCP:                   sessionMCP(a, project, team),
		WebsearchEnabled:      a.Config.Websearch.Enabled,
	})
}

func ohBundlesDir() string { return filepath.Join(config.HubDir(), "bundles") }

// modelOverridesFor returns the hub and project model cascade levels.
func modelOverridesFor(a *app.App, project *domain.Project) (hub, proj *deploy.ModelOverrides) {
	if a.Config.Models.Default != "" || len(a.Config.Models.Families) > 0 || len(a.Config.Models.Agents) > 0 {
		hub = &deploy.ModelOverrides{Default: a.Config.Models.Default, Families: a.Config.Models.Families, Agents: a.Config.Models.Agents}
	}
	if project.Model != "" || project.ModelOverrides != nil {
		proj = &deploy.ModelOverrides{Default: project.Model}
		if project.ModelOverrides != nil {
			proj.Families, proj.Agents = project.ModelOverrides.Families, project.ModelOverrides.Agents
		}
	}
	return hub, proj
}

// sessionMCP converts the project MCP cascade into bundle MCP servers
// (`oh mcp serve <name>` reads its token from the host secret store).
func sessionMCP(a *app.App, project *domain.Project, team config.ResolvedTeamConfig) []sessionspec.MCPServerDef {
	exe, err := os.Executable()
	if err != nil {
		exe = "oh"
	}
	var out []sessionspec.MCPServerDef
	for _, s := range buildMCPServersForProject(a, project.MCPConfig, team) {
		if !s.Enabled || !deploy.MCPServerUsable(s) {
			continue
		}
		cmd := []string{exe, "mcp", "serve", s.Name}
		if s.TokenKey != "" {
			cmd = append(cmd, "--token-key", s.TokenKey)
		}
		env := map[string]string{}
		if s.WriteEnabled {
			env["GITLAB_WRITE_ENABLED"] = "true"
		}
		if s.URL != "" {
			env[strings.ToUpper(s.Name)+"_URL"] = s.URL
		}
		for k, v := range s.Environment {
			env[k] = v
		}
		def := sessionspec.MCPServerDef{Name: s.Name, Type: "local", Command: cmd}
		if len(env) > 0 {
			def.Environment = env
		}
		out = append(out, def)
	}
	return out
}
