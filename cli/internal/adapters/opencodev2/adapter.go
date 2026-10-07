package opencodev2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Name is the adapter identifier.
const Name = "opencode-v2"

// Adapter implements adapters.ToolAdapter for opencode V2.
type Adapter struct {
	Binary   string   // resolved opencode binary
	Ver      string   // detected version
	Natives  []string // native agents to disable (see LoadNatives)
	CacheDir string   // where discovery results are cached

	// DisablePlugin forces the fallback mode (agent bodies rendered as
	// `system` + tooling posture) instead of the oh plugin.
	DisablePlugin bool
	// PluginTrace, when set, makes the oh plugin trace injected prompts (tests).
	PluginTrace string
	// PluginTimeout bounds the wait for the oh plugin to become active (default 10s).
	PluginTimeout time.Duration
	// OhBinary runs the oh MCP servers of a bundle (`{{oh.bin}}`; empty =
	// the running executable).
	OhBinary string
}

var _ adapters.ToolAdapter = (*Adapter)(nil)

// New returns an adapter for the given binary (empty = PATH lookup).
func New(binary, cacheDir string) *Adapter {
	return &Adapter{Binary: binary, CacheDir: cacheDir}
}

// Name implements adapters.ToolAdapter.
func (a *Adapter) Name() string { return Name }

// Detect resolves the binary and version, and loads the native agent list.
func (a *Adapter) Detect(ctx context.Context) (adapters.ToolInfo, error) {
	if a.Binary == "" {
		bin, err := exec.LookPath(Command)
		if err != nil {
			return adapters.ToolInfo{Name: Name, DisplayName: DisplayName, Command: Command}, fmt.Errorf("%w: %w", ErrNotInstalled, err)
		}
		a.Binary = bin
	}
	v, err := Version(ctx, a.Binary)
	if err != nil {
		return adapters.ToolInfo{}, err
	}
	r := SupportedRange(buildinfo.Version)
	info := adapters.ToolInfo{Name: Name, DisplayName: DisplayName, Command: Command, Binary: a.Binary, Version: v,
		MinVersion: r.OpencodeMin, MaxVersion: r.OpencodeMax}
	if err := CheckVersion(buildinfo.Version, v); err != nil {
		return info, err
	}
	a.Ver = v
	if len(a.Natives) == 0 {
		a.Natives = LoadNatives(ctx, a.Binary, v, a.CacheDir, false)
	}
	return info, nil
}

// Info returns what the last Detect found (zero before Detect).
func (a *Adapter) Info() adapters.ToolInfo {
	r := SupportedRange(buildinfo.Version)
	return adapters.ToolInfo{Name: Name, DisplayName: DisplayName, Command: Command, Binary: a.Binary, Version: a.Ver,
		MinVersion: r.OpencodeMin, MaxVersion: r.OpencodeMax}
}

// RefreshNatives re-discovers native agents (after an Attest failure).
func (a *Adapter) RefreshNatives(ctx context.Context) []string {
	a.Natives = LoadNatives(ctx, a.Binary, a.Ver, a.CacheDir, true)
	return a.Natives
}

// Capabilities implements adapters.ToolAdapter.
func (a *Adapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{
		Isolation:         sessionspec.IsolationFull,
		Events:            true,
		HeadlessDecisions: true,
		MultiLocation:     true,
		PluginHooks:       true,
		Attach:            true,
		SessionContext:    true,
	}
}

// Render implements adapters.ToolAdapter.
func (a *Adapter) Render(b sessionspec.BundleSpec, p sessionspec.ProviderSpec) (adapters.RenderedConfig, error) {
	natives := a.Natives
	if len(natives) == 0 {
		natives = DefaultNatives
	}
	return Render(b, p, natives)
}

// StartServer implements adapters.ToolAdapter.
//
// The oh plugin is installed in the group data dir and agent bodies are
// injected by it (opencode's base prompt is kept). If the plugin does not
// become active, the server is restarted in fallback mode (bodies rendered as
// `system`, prefixed with the tooling posture).
func (a *Adapter) StartServer(ctx context.Context, g adapters.ServerGroup) (adapters.ServerHandle, error) {
	// Agent bodies reference the bundle as the server sees it (a mount point
	// in a container).
	g.Bundle = g.Bundle.WithBundleRoot(innerDir(g, g.Bundle.Root)).WithOhBinary(a.OhBinary)
	if !a.DisablePlugin {
		dir := filepath.Join(g.DataDir, PluginDirName)
		if err := installPlugin(dir, g.Bundle); err != nil {
			return adapters.ServerHandle{}, fmt.Errorf("installing oh plugin: %w", err)
		}
		h, err := a.start(ctx, g, withOhPlugin(g.Bundle, dir, a.PluginTrace, hookOptions(g.Provider)))
		if err != nil {
			return h, err
		}
		timeout := a.PluginTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		perr := waitPluginActive(ctx, client(h), innerDir(g, g.WorkDir), OhPluginID, timeout)
		if perr == nil {
			return h, nil
		}
		slog.Warn("oh plugin not active, restarting in fallback mode", "error", perr)
		_ = a.StopServer(context.Background(), h)
	}
	return a.start(ctx, g, withoutOhPlugin(g.Bundle))
}

// innerDir is a machine directory as seen by the server.
func innerDir(g adapters.ServerGroup, dir string) string {
	if g.Prepared == nil {
		return dir
	}
	if in, ok := g.Prepared.Paths.ToInner(dir); ok {
		return in
	}
	return dir
}

// innerBundle rewrites the machine paths of a bundle (root, skills, plugin
// directories and their path options) to the server runtime's view; npm
// plugin specs are left as is.
func innerBundle(b sessionspec.BundleSpec, m ohruntime.PathMap) (sessionspec.BundleSpec, error) {
	tr := func(p string) (string, error) {
		if p == "" {
			return "", nil
		}
		in, ok := m.ToInner(p)
		if !ok {
			return "", fmt.Errorf("%s is not visible in the server runtime", p)
		}
		return in, nil
	}
	var err error
	out := b
	if out.Root, err = tr(b.Root); err != nil {
		return out, err
	}
	if out.SkillsDir, err = tr(b.SkillsDir); err != nil {
		return out, err
	}
	out.Skills = make([]sessionspec.SkillDef, len(b.Skills))
	for i, sk := range b.Skills {
		out.Skills[i] = sk
		if out.Skills[i].Dir, err = tr(sk.Dir); err != nil {
			return out, err
		}
	}
	out.Plugins = make([]sessionspec.PluginDef, len(b.Plugins))
	for i, pl := range b.Plugins {
		out.Plugins[i] = pl
		// A workflow plugin is an npm spec, installed by the tool itself.
		if filepath.IsAbs(pl.Dir) {
			if out.Plugins[i].Dir, err = tr(pl.Dir); err != nil {
				return out, err
			}
		}
		if len(pl.Options) > 0 {
			opts := make(map[string]any, len(pl.Options))
			for k, v := range pl.Options {
				if s, ok := v.(string); ok && filepath.IsAbs(s) {
					if in, ok := m.ToInner(s); ok {
						v = in
					}
				}
				opts[k] = v
			}
			out.Plugins[i].Options = opts
		}
	}
	return out, nil
}

func (a *Adapter) start(ctx context.Context, g adapters.ServerGroup, b sessionspec.BundleSpec) (adapters.ServerHandle, error) {
	opts := ServerOptions{
		Binary:     a.Binary,
		WorkDir:    g.WorkDir,
		DataDir:    g.DataDir,
		ReadyAgent: b.EntryAgent,
		LogPath:    filepath.Join(g.DataDir, "oh-server.log"),
	}
	if g.Runtime != nil && g.Prepared != nil {
		var err error
		if b, err = innerBundle(b, g.Prepared.Paths); err != nil {
			return adapters.ServerHandle{}, err
		}
		b = remoteOhMCP(b, g.Provider)
		rt, pg := g.Runtime, g.Prepared
		opts.Paths = pg.Paths
		opts.Run = func(argv []string, env map[string]string, dir string, port int) (*exec.Cmd, error) {
			return rt.Command(ctx, pg, ohruntime.Proc{Argv: argv, Env: env, Dir: dir, Ports: []int{port}})
		}
	} else if g.IsolateUserConfig {
		home, err := isolatedConfigHome(filepath.Join(filepath.Dir(g.DataDir), "config-home"))
		if err != nil {
			return adapters.ServerHandle{}, fmt.Errorf("strict isolation: %w", err)
		}
		opts.ConfigHome = home
	}
	rc, err := a.Render(b, g.Provider)
	if err != nil {
		return adapters.ServerHandle{}, err
	}
	env := map[string]string{}
	for k, v := range rc.Env {
		if k != "OPENCODE_CONFIG_CONTENT" {
			env[k] = v
		}
	}
	for k, v := range g.Env {
		env[k] = v
	}
	opts.ConfigContent = rc.Env["OPENCODE_CONFIG_CONTENT"]
	opts.Env = env
	srv, err := StartServer(ctx, opts)
	if err != nil {
		return adapters.ServerHandle{}, err
	}
	return adapters.ServerHandle{Key: g.Key, URL: srv.URL, Password: srv.Password, PID: srv.PID, Started: srv.Started}, nil
}

// StopServer implements adapters.ToolAdapter. The process is signalled only
// after an authenticated call proves it is the tool server: a PID reused by
// another program (after a reboot) is never killed.
func (a *Adapter) StopServer(ctx context.Context, h adapters.ServerHandle) error {
	srv := AttachServer(h.URL, h.Password, h.PID)
	if !srv.Healthy(ctx) {
		if srv.Alive() {
			slog.Warn("opencode server not answering with its credentials; not signalling PID", "pid", h.PID, "url", h.URL)
		}
		return nil
	}
	return srv.Stop(ctx, 5*time.Second)
}

func client(h adapters.ServerHandle) *Client { return NewClient(h.URL, h.Password) }

// Attest implements adapters.ToolAdapter.
func (a *Adapter) Attest(ctx context.Context, h adapters.ServerHandle, b sessionspec.BundleSpec, location string) (adapters.VisibilityReport, error) {
	return Attest(ctx, client(h), b, location)
}

// CreateSession implements adapters.ToolAdapter.
func (a *Adapter) CreateSession(ctx context.Context, h adapters.ServerHandle, s sessionspec.SessionSpec) error {
	req := CreateSessionRequest{
		ID:          s.SessionID,
		Title:       s.Title,
		Agent:       s.EntryAgent,
		Location:    &Location{Directory: s.Location},
		Permissions: toRules(s.SessionRules),
	}
	if s.Model != nil && s.Model.Provider != "" && s.Model.Model != "" {
		id := strings.TrimPrefix(ModelID(*s.Model, s.Provider.Region), s.Model.Provider+"/")
		if i := strings.LastIndex(id, "#"); i >= 0 {
			id = id[:i]
		}
		req.Model = &ModelRef{ProviderID: s.Model.Provider, ID: id, Variant: s.Model.Variant}
	}
	c := client(h)
	if _, err := c.CreateSession(ctx, req); err != nil {
		return err
	}
	if len(s.SessionEnv) > 0 {
		if err := c.SetEnvironment(ctx, s.SessionID, s.SessionEnv); err != nil {
			return fmt.Errorf("setting session environment: %w", err)
		}
	}
	return nil
}

// SendPrompt implements adapters.ToolAdapter.
func (a *Adapter) SendPrompt(ctx context.Context, h adapters.ServerHandle, sessionID, text string) error {
	return client(h).Prompt(ctx, sessionID, text)
}

var _ adapters.SessionEnvSetter = (*Adapter)(nil)

// SetSessionEnv implements adapters.SessionEnvSetter. opencode keeps the
// session environment in memory only (lost when the server restarts).
func (a *Adapter) SetSessionEnv(ctx context.Context, h adapters.ServerHandle, sessionID string, env map[string]string) error {
	if env == nil {
		env = map[string]string{}
	}
	return client(h).SetEnvironment(ctx, sessionID, env)
}

// AttachCommand implements adapters.ToolAdapter: the interactive client
// connects to the server and opens the session.
func (a *Adapter) AttachCommand(h adapters.ServerHandle, sessionID string) (argv, env []string) {
	bin := a.Binary
	if bin == "" {
		bin = "opencode"
	}
	return []string{bin, "--server", h.URL, "-s", sessionID}, []string{"OPENCODE_SERVER_PASSWORD=" + h.Password}
}

// Events implements adapters.ToolAdapter.
func (a *Adapter) Events(ctx context.Context, h adapters.ServerHandle) (<-chan adapters.ToolEvent, error) {
	evs, _, err := client(h).Events(ctx)
	if err != nil {
		return nil, err
	}
	out := make(chan adapters.ToolEvent, 64)
	go func() {
		defer close(out)
		feed := newFeedDecoder()
		for e := range evs {
			te := adapters.ToolEvent{ID: e.ID, Type: e.Type, SessionID: e.SessionID(), Time: e.Time()}
			te.Kind, te.Outcome = EventKind(e.Type)
			te.Call = feed.callOf(e)
			te.Feed, te.ParentID = feed.decode(e)
			if e.Location != nil {
				te.Location = e.Location.Directory
			}
			if len(e.Data) > 0 {
				_ = json.Unmarshal(e.Data, &te.Data)
			}
			select {
			case out <- te:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// ActiveSessions implements adapters.ToolAdapter.
func (a *Adapter) ActiveSessions(ctx context.Context, h adapters.ServerHandle) ([]string, error) {
	return client(h).ActiveSessions(ctx)
}

// Pending implements adapters.ToolAdapter.
func (a *Adapter) Pending(ctx context.Context, h adapters.ServerHandle, sessionID string) ([]adapters.PendingDecision, error) {
	c := client(h)
	perms, err := c.Permissions(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	forms, err := c.Forms(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]adapters.PendingDecision, 0, len(perms)+len(forms))
	for _, p := range perms {
		d := adapters.PendingDecision{
			ID: p.ID, SessionID: p.SessionID, Kind: adapters.DecisionPermission,
			Action: p.Action, Resources: p.Resources, Message: p.Message,
		}
		if p.Source != nil && p.Source.ID != "" {
			d.Call = &adapters.ToolCall{ID: p.Source.ID, Action: NeutralAction(p.Action), Status: adapters.CallCalled}
			// The request does not carry the call input; the oh workflow
			// tools need it (which checkpoint is reached).
			if _, ours := workflowTools[p.Action]; ours {
				if in, err := c.ToolCallInput(ctx, p.SessionID, p.Source.ID); err == nil {
					d.Call.Input = in
				}
			}
		}
		out = append(out, d)
	}
	for _, f := range forms {
		d := adapters.PendingDecision{ID: f.ID, SessionID: f.SessionID, Kind: adapters.DecisionQuestion, Title: f.Title}
		for _, fl := range f.Fields {
			ff := adapters.FormField{Key: fl.Key, Title: fl.Title, Description: fl.Description, Type: fl.Type, Custom: fl.Custom, Required: fl.Required}
			for _, o := range fl.Options {
				ff.Options = append(ff.Options, adapters.FormOption{Value: o.Value, Label: o.Label, Description: o.Description})
			}
			d.Fields = append(d.Fields, ff)
		}
		out = append(out, d)
	}
	return out, nil
}

// Reply implements adapters.ToolAdapter.
func (a *Adapter) Reply(ctx context.Context, h adapters.ServerHandle, d adapters.DecisionReply) error {
	c := client(h)
	var err error
	switch d.Kind {
	case adapters.DecisionPermission:
		switch d.Decision {
		case "once", "always", "reject":
		default:
			return fmt.Errorf("invalid permission decision %q", d.Decision)
		}
		err = c.ReplyPermission(ctx, d.SessionID, d.ID, d.Decision, d.Message)
	case adapters.DecisionQuestion:
		err = c.ReplyForm(ctx, d.SessionID, d.ID, d.Answer)
	default:
		return fmt.Errorf("unknown decision kind %q", d.Kind)
	}
	switch {
	case err == nil:
		return nil
	case IsSettled(err):
		return fmt.Errorf("%w: %v", adapters.ErrRequestGone, err)
	case IsInvalidAnswer(err):
		return fmt.Errorf("%w: %v", adapters.ErrInvalidAnswer, err)
	}
	return err
}

// Control implements adapters.ToolAdapter.
func (a *Adapter) Control(ctx context.Context, h adapters.ServerHandle, sessionID string, op adapters.ControlOp) error {
	c := client(h)
	switch op.Kind {
	case adapters.ControlInterrupt:
		return c.Interrupt(ctx, sessionID)
	case adapters.ControlSynthetic:
		return c.SyntheticWith(ctx, sessionID, op.Text, string(op.Delivery))
	case adapters.ControlPrompt:
		return c.PromptWith(ctx, sessionID, op.Text, string(op.Delivery))
	case adapters.ControlCompact:
		return c.Compact(ctx, sessionID)
	case adapters.ControlSwitchModel:
		if op.Model == nil || op.Model.Provider == "" || op.Model.Model == "" {
			return errors.New("switch_model requires a model")
		}
		region := op.Region
		ref := ModelRef{ProviderID: op.Model.Provider, Variant: op.Model.Variant}
		ref.ID = strings.TrimPrefix(ModelID(*op.Model, region), op.Model.Provider+"/")
		if i := strings.LastIndex(ref.ID, "#"); i >= 0 {
			ref.ID = ref.ID[:i]
		}
		if region == "" {
			// Keep the inference profile prefix of the current model (eu., us.…).
			if cur, err := c.GetSession(ctx, sessionID); err == nil && cur.Model != nil {
				ref.ID = withGeoOf(ref.ID, cur.Model.ID, op.Model.Provider)
			}
		}
		return c.SwitchModel(ctx, sessionID, ref)
	}
	return fmt.Errorf("unsupported control %q", op.Kind)
}

var _ adapters.ActionNamer = (*Adapter)(nil)

// NeutralAction implements adapters.ActionNamer.
func (a *Adapter) NeutralAction(action string) string { return NeutralAction(action) }

var _ adapters.SessionRulesSetter = (*Adapter)(nil)

// SetSessionRules implements adapters.SessionRulesSetter.
func (a *Adapter) SetSessionRules(ctx context.Context, h adapters.ServerHandle, sessionID string, rules []sessionspec.PermissionRule) error {
	return client(h).SetSessionPermissions(ctx, sessionID, toRules(rules))
}

var _ adapters.Forker = (*Adapter)(nil)

var _ adapters.ChildLister = (*Adapter)(nil)

// Children implements adapters.ChildLister.
func (a *Adapter) Children(ctx context.Context, h adapters.ServerHandle) (map[string]string, error) {
	list, err := client(h).ListSessions(ctx, "")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, s := range list {
		if s.ParentID != "" {
			out[s.ID] = s.ParentID
		}
	}
	return out, nil
}

// Fork implements adapters.Forker.
func (a *Adapter) Fork(ctx context.Context, h adapters.ServerHandle, sessionID string) (string, error) {
	s, err := client(h).Fork(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// withGeoOf adds to a Bedrock Anthropic model id the inference profile
// prefix ("eu.", "us."…) of the current model of the session.
func withGeoOf(id, current, provider string) string {
	if provider != "amazon-bedrock" || !strings.HasPrefix(id, "anthropic.") {
		return id
	}
	if i := strings.Index(current, ".anthropic."); i > 0 {
		return current[:i] + "." + id
	}
	return id
}

// Usage returns the cost and token usage of a session (no diff).
func (a *Adapter) Usage(ctx context.Context, h adapters.ServerHandle, sessionID string) (adapters.SessionResult, error) {
	s, err := client(h).GetSession(ctx, sessionID)
	if err != nil {
		return adapters.SessionResult{}, err
	}
	return adapters.SessionResult{
		SessionID: s.ID, Title: s.Title, Agent: s.Agent, Cost: s.Cost,
		TokensIn: s.Tokens.Input, TokensOut: s.Tokens.Output, TokensReasoning: s.Tokens.Reasoning,
		TokensCacheRead: s.Tokens.Cache.Read, TokensCacheWrite: s.Tokens.Cache.Write,
	}, nil
}

// Results implements adapters.ToolAdapter.
func (a *Adapter) Results(ctx context.Context, h adapters.ServerHandle, sessionID string) (adapters.SessionResult, error) {
	c := client(h)
	s, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return adapters.SessionResult{}, err
	}
	res := adapters.SessionResult{
		SessionID: s.ID, Title: s.Title, Agent: s.Agent, Cost: s.Cost,
		TokensIn: s.Tokens.Input, TokensOut: s.Tokens.Output, TokensReasoning: s.Tokens.Reasoning,
		TokensCacheRead: s.Tokens.Cache.Read, TokensCacheWrite: s.Tokens.Cache.Write,
	}
	if s.Model != nil {
		res.Model = s.Model.ProviderID + "/" + s.Model.ID
	}
	if s.Location.Directory != "" {
		if vcs, err := c.VCS(ctx, s.Location.Directory); err == nil {
			res.Branch = vcs.Branch.Current
		}
	}
	diff, err := c.Diff(ctx, sessionID)
	if err != nil {
		return res, err
	}
	for _, d := range diff {
		res.Changes = append(res.Changes, adapters.FileChange{File: d.File, Status: d.Status, Additions: d.Additions, Deletions: d.Deletions, Patch: d.Patch})
	}
	return res, nil
}
