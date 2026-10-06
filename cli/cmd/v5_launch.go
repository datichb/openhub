package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
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
	"github.com/datichb/openhub/cli/internal/worktree"
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
	v5Once.Do(func() {
		v5Adapter, v5Err = detectV2Adapter(ctx)
		worktree.SessionBundles = v5Err == nil
	})
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
	if runtime.GOOS == "windows" {
		// The background daemon (credential proxy host) does not exist on
		// Windows yet; the legacy pipeline does not work with opencode V2.
		return true, errors.New(i18n.T("cmd.v5.windows_unsupported"))
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

	req := v5Request(a, project, opts.Provider)
	ui.Notify(i18n.Tf("cmd.v5.preparing", entry), launcher.LevelInfo)
	b, err := buildSessionBundle(a, project, config.ResolveTeamForProject(a.Config, project), entry, req.Provider)
	if err != nil {
		return true, fmt.Errorf("building session bundle: %w", err)
	}

	svc, err := newRunService(ctx, a)
	if err != nil {
		return true, err
	}
	req.Location, req.Bundle, req.EntryAgent = location, b, entry
	req.Title, req.Prompt, req.WorkflowID = sessionTitle(project, entry), opts.Prompt, entry
	res, err := svc.StartSession(ctx, req)
	if err != nil {
		return true, err
	}
	for _, w := range res.Report.Warnings {
		ui.Notify(i18n.Tf("cmd.v5.isolation_warning", w), launcher.LevelWarning)
	}

	return true, afterStart(ctx, a, svc, ui, req.Attach, res, true)
}

// v5Request returns the provider, team and attach settings of a project
// session (bundle, location and prompt are set by the caller).
func v5Request(a *app.App, project *domain.Project, providerFlag string) runsvc.StartRequest {
	prov := provider.ResolveProvider(providerFlag, project.Provider, a.Config.Opencode.DefaultProvider)
	var projProv *provider.ProviderConfig
	tokenKey := ""
	if project.ProviderConfig != nil {
		projProv = &provider.ProviderConfig{AWSProfile: project.ProviderConfig.AWSProfile, AWSRegion: project.ProviderConfig.AWSRegion}
		tokenKey = project.ProviderConfig.TokenKey
	}
	req := runsvc.StartRequest{
		ProjectID: project.ID, ProjectTokenKey: tokenKey, Provider: prov,
		ProviderCfg: provider.ResolveProviderConfig(projProv, hubProviderCfg(a, prov)),
		Attach:      sessionspec.AttachPref(attachPreference(a)), ITermStyle: termlaunch.ITermStyle(a.Config.Session.ITermStyle),
	}
	if team := config.ResolveTeamForProject(a.Config, project); team.Enabled {
		req.TeamID = team.TeamID
		if team.MemberID != "" {
			mid := team.MemberID
			req.MemberID = &mid
		}
	}
	return req
}

// resumeV5Session wakes the server group of a sleeping session.
func resumeV5Session(ctx context.Context, a *app.App, svc *runsvc.Service, sessionID string) error {
	sess, err := svc.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	project, err := a.Projects.Get(ctx, sess.ProjectID)
	if err != nil {
		return err
	}
	req := v5Request(a, project, sess.Provider)
	return svc.ResumeSession(ctx, sessionID, req)
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
func runAttachInline(ctx context.Context, a *app.App, svc *runsvc.Service, ui launcher.LaunchUI, sessionID string) error {
	run := func() error { return runAttachChild(ctx, a, svc, sessionID) }
	if suspend := ui.SuspendAndExec(); suspend != nil {
		return suspend(run)
	}
	return run()
}

// runAttachChild runs the tool client attached to a session in the current
// terminal and declares it to the daemon while it runs (an attached session
// is never put to sleep). A sleeping session is resumed first.
func runAttachChild(ctx context.Context, a *app.App, svc *runsvc.Service, sessionID string) error {
	argv, env, err := svc.AttachCommand(ctx, sessionID)
	if errors.Is(err, runsvc.ErrServerNotRunning) {
		if rerr := resumeV5Session(ctx, a, svc, sessionID); rerr != nil {
			return rerr
		}
		argv, env, err = svc.AttachCommand(ctx, sessionID)
	}
	if err != nil {
		return err
	}
	sess, _ := svc.Session(ctx, sessionID)
	hbCtx, stop := context.WithCancel(ctx)
	defer stop()
	if dc, _, derr := ensureDaemon(ctx); derr == nil && sess != nil {
		go dc.KeepAlive(hbCtx, daemon.HeartbeatRequest{
			ClientID: "attach-" + sessionspec.NewSessionID(), Kind: daemon.ClientAttach, Group: sess.GroupKey, SessionID: sessionID,
		}, 30*time.Second)
	}
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = append(os.Environ(), env...)
	// The client handles Ctrl+C itself: absorb it here without removing the
	// handlers other parts of oh registered (parallel monitor).
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	defer signal.Stop(sigs)
	return c.Run()
}

// buildSessionBundle compiles the phase 0 bundle for an entry agent.
func buildSessionBundle(a *app.App, project *domain.Project, team config.ResolvedTeamConfig, entry, prov string) (*bundle.Bundle, error) {
	wf, err := deploy.ResolveAndPrepareWorkflow(workflow.BaseWorkflow(), collectWorkflowOverrides(a, project, team)...)
	if err != nil {
		slog.Warn("workflow resolution failed, using base workflow", "error", err)
		wf, _ = deploy.ResolveAndPrepareWorkflow(workflow.BaseWorkflow())
	}
	req := sessionBundleRequest(a, project, team, prov)
	req.EntryAgent, req.Workflow = entry, wf
	return bundle.Build(req)
}

// buildWorkflowBundle compiles the bundle of a resolved oh/v1 workflow: its
// agents and delegation graph, generated chain skills, `models:` (workflow
// level of the cascade), `skills:` and `isolation:` (bundle.Request.Spec).
// project may be nil (hub-only bundle).
func buildWorkflowBundle(a *app.App, project *domain.Project, spec *workflow.Spec, prov string) (*bundle.Bundle, error) {
	var team config.ResolvedTeamConfig
	if project != nil {
		team = config.ResolveTeamForProject(a.Config, project)
	}
	req := sessionBundleRequest(a, project, team, prov)
	req.Spec = spec
	return bundle.Build(req)
}

// sessionBundleRequest is the part of a bundle request shared by every
// launch: hub, project instructions, model cascade and MCP servers.
func sessionBundleRequest(a *app.App, project *domain.Project, team config.ResolvedTeamConfig, prov string) bundle.Request {
	req := bundle.Request{
		HubDir: hubcontent.HubContentDir(), OutDir: ohBundlesDir(), Provider: prov,
		ExtraInstructionFiles: a.Config.Deploy.InstructionFiles,
		WebsearchEnabled:      a.Config.Websearch.Enabled,
	}
	req.HubOverrides, req.ProjectOverrides = modelOverridesFor(a, project)
	if project != nil {
		req.ProjectPath = project.Path
		req.MCP = sessionMCP(a, project, team)
	}
	return req
}

func ohBundlesDir() string { return filepath.Join(config.HubDir(), "bundles") }

// modelOverridesFor returns the hub and project model cascade levels.
func modelOverridesFor(a *app.App, project *domain.Project) (hub, proj *deploy.ModelOverrides) {
	if a.Config.Models.Default != "" || len(a.Config.Models.Families) > 0 || len(a.Config.Models.Agents) > 0 {
		hub = &deploy.ModelOverrides{Default: a.Config.Models.Default, Families: a.Config.Models.Families, Agents: a.Config.Models.Agents}
	}
	if project != nil && (project.Model != "" || project.ModelOverrides != nil) {
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
