package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/mcp/github"
	"github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/jira"
	"github.com/datichb/openhub/cli/internal/mcp/linear"
	"github.com/datichb/openhub/cli/internal/mcp/team"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

// projectProvider returns the provider, explicit credential key and provider
// config (AWS profile / region) of a project session.
func projectProvider(a *app.App, project *domain.Project, providerFlag string) (prov, tokenKey string, cfg provider.Config) {
	prov = provider.ResolveProvider(providerFlag, project.Provider, a.Config.LLM.DefaultProvider)
	var projProv *provider.ProviderConfig
	if project.ProviderConfig != nil {
		projProv = &provider.ProviderConfig{AWSProfile: project.ProviderConfig.AWSProfile, AWSRegion: project.ProviderConfig.AWSRegion}
		tokenKey = project.ProviderConfig.TokenKey
	}
	return prov, tokenKey, provider.ResolveProviderConfig(projProv, hubProviderCfg(a, prov))
}

// v5Request returns the provider, team and attach settings of a project
// session (bundle, location and prompt are set by the caller).
func v5Request(a *app.App, project *domain.Project, providerFlag string) runsvc.StartRequest {
	prov, tokenKey, provCfg := projectProvider(a, project, providerFlag)
	req := runsvc.StartRequest{
		ProjectID: project.ID, ProjectTokenKey: tokenKey, Provider: prov,
		ProviderCfg: provCfg,
		Attach:      sessionspec.AttachPref(attachPreference(a)), ITermStyle: termlaunch.ITermStyle(a.Config.Session.ITermStyle),
	}
	applyProjectExec(&req, project)
	req.IsolateUserConfig = a.Config.Execution.StrictIsolation
	req.Limits = sessionLimits(context.Background(), a, project, nil)
	if tc := config.ResolveTeamForProject(a.Config, project); tc.Enabled {
		req.TeamID = tc.TeamID
		if tc.MemberID != "" {
			mid := tc.MemberID
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

// buildWorkflowBundle compiles the bundle of a resolved oh/v1 workflow: its
// agents and delegation graph, generated chain skills, `models:` (workflow
// level of the cascade), `skills:`, `isolation:`, `plugins:`, `code_mode:`
// (bundle.Request.Spec) and `mcp:` (selection among the project MCP
// servers). project may be nil (hub-only bundle). missing lists the MCP
// servers the workflow asks for but the project does not provide.
func buildWorkflowBundle(a *app.App, project *domain.Project, res *workflowsvc.Resolution, prov string) (b *bundle.Bundle, missing []string, err error) {
	return buildWorkflowBundleIn(a, project, res, prov, "")
}

// buildWorkflowBundleIn is buildWorkflowBundle with another bundles
// directory ("" = ~/.oh/bundles; the editor preview builds in a temporary
// one).
func buildWorkflowBundleIn(a *app.App, project *domain.Project, res *workflowsvc.Resolution, prov, outDir string) (b *bundle.Bundle, missing []string, err error) {
	var tc config.ResolvedTeamConfig
	if project != nil {
		tc = config.ResolveTeamForProject(a.Config, project)
	}
	req := sessionBundleRequest(a, project, tc, prov)
	if outDir != "" {
		req.OutDir = outDir
	}
	req.Spec = res.Spec
	if dir := res.BricksDir(); dir != "" {
		req.HubDir = dir // team catalogue bricks (v5 phase 2)
	}
	if ids, set := res.MCPSelection(); set {
		req.MCP, missing = selectMCP(req.MCP, ids)
	}
	b, err = bundle.Build(req)
	return b, missing, err
}

// workflowMCPServer is the oh MCP server of the workflow runtime (piste G),
// added to every workflow bundle: listing it is never « missing ».
const workflowMCPServer = "workflow"

// selectMCP keeps the servers listed by the workflow, in the project order,
// and returns the listed ones the project does not provide.
func selectMCP(available []sessionspec.MCPServerDef, ids []string) (kept []sessionspec.MCPServerDef, missing []string) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	have := map[string]bool{}
	for _, s := range available {
		if want[s.Name] {
			kept = append(kept, s)
			have[s.Name] = true
		}
	}
	for _, id := range ids {
		if !have[id] && id != workflowMCPServer {
			missing = append(missing, id)
		}
	}
	return kept, missing
}

// sessionBundleRequest is the part of a bundle request shared by every
// launch: hub, project instructions, model cascade and MCP servers.
func sessionBundleRequest(a *app.App, project *domain.Project, tc config.ResolvedTeamConfig, prov string) bundle.Request {
	req := bundle.Request{
		HubDir: hubcontent.HubContentDir(), OutDir: ohBundlesDir(), Provider: prov, ToolProvider: toolProviderID(prov),
		ExtraInstructionFiles: a.Config.Deploy.InstructionFiles,
		WebsearchEnabled:      a.Config.Websearch.Enabled,
	}
	req.HubOverrides, req.ProjectOverrides = modelOverridesFor(a, project)
	req.TeamOverrides = teamModelOverrides(tc)
	if project != nil {
		req.ProjectPath = project.Path
		req.MCP = sessionMCP(a, project, tc)
	}
	return req
}

func ohBundlesDir() string { return filepath.Join(config.HubDir(), "bundles") }

// modelOverridesFor returns the hub and project model cascade levels.
func modelOverridesFor(a *app.App, project *domain.Project) (hub, proj *bricks.ModelOverrides) {
	if a.Config.Models.Default != "" || len(a.Config.Models.Families) > 0 || len(a.Config.Models.Agents) > 0 {
		hub = &bricks.ModelOverrides{Default: a.Config.Models.Default, Families: a.Config.Models.Families, Agents: a.Config.Models.Agents}
	}
	if project != nil && (project.Model != "" || project.ModelOverrides != nil) {
		proj = &bricks.ModelOverrides{Default: project.Model}
		if project.ModelOverrides != nil {
			proj.Families, proj.Agents = project.ModelOverrides.Families, project.ModelOverrides.Agents
		}
	}
	return hub, proj
}

// teamModelOverrides returns the team recommendations of the model cascade
// (`[models]` of the team-state config.toml, ADR-030; below hub and project).
func teamModelOverrides(tc config.ResolvedTeamConfig) *bricks.ModelOverrides {
	if !tc.Enabled || tc.StatePath == "" {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	cfg, err := repo.LoadConfig()
	if err != nil {
		slog.Warn("team model recommendations not loaded", "team", tc.TeamID, "error", err)
		return nil
	}
	m := cfg.Models
	if m.Default == "" && len(m.Families) == 0 && len(m.Agents) == 0 {
		return nil
	}
	return &bricks.ModelOverrides{Default: m.Default, Families: m.Families, Agents: m.Agents}
}

// mcpWriteEnv is the variable that enables the write tools of each oh MCP
// server (servers without write tools have none).
var mcpWriteEnv = map[string]string{
	"gitlab": gitlab.EnvWriteEnabled,
	"github": github.EnvWriteEnabled,
	"jira":   jira.EnvWriteEnabled,
	"linear": linear.EnvWriteEnabled,
}

// sessionMCPEnv returns the environment of an oh MCP server: its write
// switch (only for a server that has one), its URL and its own variables.
func sessionMCPEnv(s bricks.MCPServerDef) map[string]string {
	env := map[string]string{}
	if v := mcpWriteEnv[s.Name]; v != "" && s.WriteEnabled {
		env[v] = "true"
	}
	if s.URL != "" {
		env[strings.ToUpper(s.Name)+"_URL"] = s.URL
	}
	for k, v := range s.Environment {
		env[k] = v
	}
	return env
}

// sessionMCP converts the project MCP cascade into bundle MCP servers
// (`oh mcp serve <name>` reads its token from the host secret store).
func sessionMCP(a *app.App, project *domain.Project, teamCfg config.ResolvedTeamConfig) []sessionspec.MCPServerDef {
	exe, err := os.Executable()
	if err != nil {
		exe = "oh"
	}
	var out []sessionspec.MCPServerDef
	for _, s := range buildMCPServersForProject(a, project.MCPConfig, teamCfg) {
		if !s.Enabled || !bricks.MCPServerUsable(s) {
			continue
		}
		cmd := []string{exe, "mcp", "serve", s.Name}
		if s.TokenKey != "" {
			cmd = append(cmd, "--token-key", s.TokenKey)
		}
		env := sessionMCPEnv(s)
		if s.Name == "team" {
			// The team server reads the team of the session project from its
			// environment (P3-T29; formerly a file written by `oh deploy`).
			env[team.EnvTeamID], env[team.EnvProjectID] = teamCfg.TeamID, project.ID
		}
		def := sessionspec.MCPServerDef{Name: s.Name, Type: "local", Command: cmd}
		if len(env) > 0 {
			def.Environment = env
		}
		out = append(out, def)
	}
	return out
}
