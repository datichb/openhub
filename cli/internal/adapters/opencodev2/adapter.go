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
		bin, err := exec.LookPath("opencode")
		if err != nil {
			return adapters.ToolInfo{}, fmt.Errorf("opencode binary not found: %w", err)
		}
		a.Binary = bin
	}
	v, err := Version(ctx, a.Binary)
	if err != nil {
		return adapters.ToolInfo{}, err
	}
	if !strings.HasPrefix(v, "2.") {
		return adapters.ToolInfo{}, fmt.Errorf("opencode %s is not a V2 release", v)
	}
	a.Ver = v
	if len(a.Natives) == 0 {
		a.Natives = LoadNatives(ctx, a.Binary, v, a.CacheDir, false)
	}
	return adapters.ToolInfo{Name: Name, Binary: a.Binary, Version: v}, nil
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
	if !a.DisablePlugin {
		dir := filepath.Join(g.DataDir, PluginDirName)
		if err := installPlugin(dir, g.Bundle); err != nil {
			return adapters.ServerHandle{}, fmt.Errorf("installing oh plugin: %w", err)
		}
		h, err := a.start(ctx, g, withOhPlugin(g.Bundle, dir, a.PluginTrace))
		if err != nil {
			return h, err
		}
		timeout := a.PluginTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		perr := waitPluginActive(ctx, client(h), g.WorkDir, OhPluginID, timeout)
		if perr == nil {
			return h, nil
		}
		slog.Warn("oh plugin not active, restarting in fallback mode", "error", perr)
		_ = a.StopServer(context.Background(), h)
	}
	return a.start(ctx, g, withoutOhPlugin(g.Bundle))
}

func (a *Adapter) start(ctx context.Context, g adapters.ServerGroup, b sessionspec.BundleSpec) (adapters.ServerHandle, error) {
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
	srv, err := StartServer(ctx, ServerOptions{
		Binary:        a.Binary,
		WorkDir:       g.WorkDir,
		DataDir:       g.DataDir,
		ConfigContent: rc.Env["OPENCODE_CONFIG_CONTENT"],
		Env:           env,
		ReadyAgent:    b.EntryAgent,
		LogPath:       filepath.Join(g.DataDir, "oh-server.log"),
	})
	if err != nil {
		return adapters.ServerHandle{}, err
	}
	return adapters.ServerHandle{Key: g.Key, URL: srv.URL, Password: srv.Password, PID: srv.PID, Started: srv.Started}, nil
}

// StopServer implements adapters.ToolAdapter.
func (a *Adapter) StopServer(ctx context.Context, h adapters.ServerHandle) error {
	return AttachServer(h.URL, h.Password, h.PID).Stop(ctx, 5*time.Second)
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
		for e := range evs {
			te := adapters.ToolEvent{ID: e.ID, Type: e.Type, SessionID: e.SessionID(), Time: e.Time()}
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
		out = append(out, adapters.PendingDecision{
			ID: p.ID, SessionID: p.SessionID, Kind: adapters.DecisionPermission,
			Action: p.Action, Resources: p.Resources,
		})
	}
	for _, f := range forms {
		d := adapters.PendingDecision{ID: f.ID, SessionID: f.SessionID, Kind: adapters.DecisionQuestion, Title: f.Title}
		for _, fl := range f.Fields {
			ff := adapters.FormField{Key: fl.Key, Title: fl.Title, Description: fl.Description, Type: fl.Type, Custom: fl.Custom}
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
	switch d.Kind {
	case adapters.DecisionPermission:
		switch d.Decision {
		case "once", "always", "reject":
		default:
			return fmt.Errorf("invalid permission decision %q", d.Decision)
		}
		return c.ReplyPermission(ctx, d.SessionID, d.ID, d.Decision, d.Message)
	case adapters.DecisionQuestion:
		return c.ReplyForm(ctx, d.SessionID, d.ID, d.Answer)
	}
	return fmt.Errorf("unknown decision kind %q", d.Kind)
}

// Control implements adapters.ToolAdapter.
func (a *Adapter) Control(ctx context.Context, h adapters.ServerHandle, sessionID string, op adapters.ControlOp) error {
	c := client(h)
	switch op.Kind {
	case "interrupt":
		return c.Interrupt(ctx, sessionID)
	case "synthetic":
		return c.Synthetic(ctx, sessionID, op.Text)
	case "prompt":
		return c.Prompt(ctx, sessionID, op.Text)
	case "switch_model":
		if op.Model == nil {
			return errors.New("switch_model requires a model")
		}
		return c.SwitchModel(ctx, sessionID, ModelRef{ProviderID: op.Model.Provider, ID: op.Model.Model, Variant: op.Model.Variant})
	}
	return fmt.Errorf("unsupported control %q", op.Kind)
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
	diff, err := c.Diff(ctx, sessionID)
	if err != nil {
		return res, err
	}
	for _, d := range diff {
		res.Changes = append(res.Changes, adapters.FileChange{File: d.File, Status: d.Status, Additions: d.Additions, Deletions: d.Deletions, Patch: d.Patch})
	}
	return res, nil
}
