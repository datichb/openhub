package opencodev2

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// SystemAgents are internal opencode agents (hidden, not selectable) that
// must stay enabled: they perform compaction, titling and summaries.
var SystemAgents = map[string]bool{"compaction": true, "title": true, "summary": true}

// DefaultNatives is the fallback list of selectable built-in agents in
// opencode 2.0.x, used when dynamic discovery is unavailable.
var DefaultNatives = []string{"build", "plan", "general", "explore"}

// OhPluginID is the identifier of the oh plugin (prompt injection, closed world).
// When the bundle ships it, agent bodies are injected by the plugin instead of
// replacing opencode's base prompt with `system`.
const OhPluginID = "oh"

// ConfigFileName is the rendered config file name inside the bundle.
const ConfigFileName = "config.json"

// Render produces the opencode V2 configuration for a bundle.
//
// Closed world rules (D13):
//   - every native agent (natives) is disabled;
//   - skills: deny "*" then allow the bundle skills (hides built-in skills);
//   - subagents: per agent, deny "*" then allow the targets of SubagentGraph;
//   - project config is excluded by the server environment (StartServer).
func Render(b sessionspec.BundleSpec, p sessionspec.ProviderSpec, natives []string) (adapters.RenderedConfig, error) {
	cfg, err := BuildConfig(b, p, natives)
	if err != nil {
		return adapters.RenderedConfig{}, err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return adapters.RenderedConfig{}, fmt.Errorf("encoding config: %w", err)
	}
	env := map[string]string{"OPENCODE_CONFIG_CONTENT": string(data)}
	if p.SessionToken != "" {
		if tokenEnv := providerTokenEnv(p); tokenEnv != "" {
			env[tokenEnv] = p.SessionToken
		}
	}
	return adapters.RenderedConfig{
		Env:   env,
		Files: map[string][]byte{ConfigFileName: data},
	}, nil
}

// BuildConfig returns the config as a JSON-ready map (exported for tests/inspection).
func BuildConfig(b sessionspec.BundleSpec, p sessionspec.ProviderSpec, natives []string) (map[string]any, error) {
	if b.EntryAgent == "" {
		return nil, fmt.Errorf("bundle has no entry agent")
	}
	if !b.HasAgent(b.EntryAgent) {
		return nil, fmt.Errorf("entry agent %q is not part of the bundle", b.EntryAgent)
	}
	usePlugin := hasPlugin(b, OhPluginID)

	agents := map[string]any{}
	for _, n := range natives {
		if SystemAgents[n] {
			continue
		}
		agents[n] = map[string]any{"disabled": true}
	}
	skillIDs := b.SkillIDs()
	for _, a := range b.Agents {
		agents[a.ID] = renderAgent(a, b, skillIDs, usePlugin)
	}

	cfg := map[string]any{
		"$schema":       "https://opencode.ai/config.json",
		"default_agent": b.EntryAgent,
		"agents":        agents,
		"permissions":   toRules(globalRules(b, skillIDs)),
		"experimental":  map[string]any{"subagent_depth": maxInt(b.MaxDepth, 1)},
		"snapshots":     true,
	}
	if b.DefaultModel != nil && b.DefaultModel.String() != "" {
		cfg["model"] = b.DefaultModel.String()
	}
	if b.SkillsDir != "" {
		cfg["skills"] = []string{b.SkillsDir}
	} else if len(b.Skills) > 0 {
		dirs := []string{}
		seen := map[string]bool{}
		for _, s := range b.Skills {
			d := path.Dir(s.Dir)
			if !seen[d] {
				seen[d] = true
				dirs = append(dirs, d)
			}
		}
		cfg["skills"] = dirs
	}
	if len(b.Plugins) > 0 {
		plugins := make([]any, 0, len(b.Plugins))
		for _, pl := range b.Plugins {
			if len(pl.Options) > 0 {
				plugins = append(plugins, map[string]any{"package": pl.Dir, "options": pl.Options})
			} else {
				plugins = append(plugins, pl.Dir)
			}
		}
		cfg["plugins"] = plugins
	}
	if servers := renderMCP(b); len(servers) > 0 {
		cfg["mcp"] = map[string]any{"servers": servers}
	}
	if prov := renderProvider(p); prov != nil {
		cfg["providers"] = map[string]any{p.ID: prov}
	}
	return cfg, nil
}

func renderAgent(a sessionspec.AgentDef, b sessionspec.BundleSpec, bundleSkills []string, usePlugin bool) map[string]any {
	mode := a.Mode
	if mode == "" {
		mode = "primary"
	}
	out := map[string]any{
		"description": a.Description,
		"mode":        mode,
	}
	if a.Hidden {
		out["hidden"] = true
	}
	if a.Model != nil && a.Model.String() != "" {
		out["model"] = a.Model.String()
	}
	if !usePlugin && a.Body != "" {
		out["system"] = a.Body
	}

	rules := agentRules(a, b, bundleSkills)
	out["permissions"] = toRules(rules)
	return out
}

// agentRules returns the rendered permission rules of an agent: its own rules
// except skill/subagent ones, then the skill filter and the subagent graph.
func agentRules(a sessionspec.AgentDef, b sessionspec.BundleSpec, bundleSkills []string) []sessionspec.PermissionRule {
	rules := make([]sessionspec.PermissionRule, 0, len(a.Permissions)+len(bundleSkills)+4)
	for _, r := range a.Permissions {
		// skill and subagent visibility is decided below from the bundle/graph.
		if r.Action == sessionspec.ActionSkill || r.Action == sessionspec.ActionSubagent {
			continue
		}
		rules = append(rules, r)
	}
	// Skills: the agent's own skill rules filter the bundle skills.
	rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSkill, Resource: "*", Effect: sessionspec.EffectDeny})
	for _, id := range bundleSkills {
		if effect := evaluate(a.Permissions, sessionspec.ActionSkill, id, sessionspec.EffectAllow); effect != sessionspec.EffectDeny {
			rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSkill, Resource: id, Effect: effect})
		}
	}
	// Subagents: only the graph targets.
	rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "*", Effect: sessionspec.EffectDeny})
	targets := append([]string(nil), b.SubagentGraph[a.ID]...)
	sort.Strings(targets)
	for _, tgt := range targets {
		if b.HasAgent(tgt) {
			rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: tgt, Effect: sessionspec.EffectAllow})
		}
	}
	return rules
}

func globalRules(b sessionspec.BundleSpec, skills []string) []sessionspec.PermissionRule {
	rules := append([]sessionspec.PermissionRule(nil), b.Permissions...)
	rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSkill, Resource: "*", Effect: sessionspec.EffectDeny})
	for _, id := range skills {
		rules = append(rules, sessionspec.PermissionRule{Action: sessionspec.ActionSkill, Resource: id, Effect: sessionspec.EffectAllow})
	}
	if !b.CodeMode {
		rules = append(rules, sessionspec.PermissionRule{Action: "execute", Resource: "*", Effect: sessionspec.EffectDeny})
	}
	return rules
}

func renderMCP(b sessionspec.BundleSpec) map[string]any {
	servers := map[string]any{}
	for _, m := range b.MCP {
		s := map[string]any{"type": m.Type}
		switch m.Type {
		case "remote":
			s["url"] = m.URL
			s["oauth"] = false
			if len(m.Headers) > 0 {
				s["headers"] = m.Headers
			}
		default:
			s["type"] = "local"
			s["command"] = m.Command
			if len(m.Environment) > 0 {
				s["environment"] = m.Environment
			}
		}
		if !b.CodeMode {
			s["codemode"] = false
		}
		servers[m.Name] = s
	}
	return servers
}

func renderProvider(p sessionspec.ProviderSpec) map[string]any {
	if p.ID == "" {
		return nil
	}
	settings := map[string]any{}
	if p.Region != "" {
		settings["region"] = p.Region
	}
	if p.BaseURL != "" {
		settings["baseURL"] = p.BaseURL
	}
	if len(settings) == 0 {
		return nil
	}
	return map[string]any{"settings": settings}
}

// providerTokenEnv returns the environment variable from which opencode reads
// the provider credential (the oh proxy session token is placed there).
func providerTokenEnv(p sessionspec.ProviderSpec) string {
	if p.TokenEnv != "" {
		return p.TokenEnv
	}
	switch p.ID {
	case "amazon-bedrock":
		return "AWS_BEARER_TOKEN_BEDROCK"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	}
	return ""
}

// evaluate applies ordered rules (last match wins) for action/resource.
func evaluate(rules []sessionspec.PermissionRule, action, resource string, def sessionspec.Effect) sessionspec.Effect {
	effect := def
	for _, r := range rules {
		if wildcardMatch(r.Action, action) && wildcardMatch(r.Resource, resource) {
			effect = r.Effect
		}
	}
	return effect
}

// wildcardMatch implements opencode whole-value wildcards: '*' (any run,
// including '/') and '?' (one character).
func wildcardMatch(pattern, s string) bool {
	p, v := []rune(pattern), []rune(s)
	pi, si, star, mark := 0, 0, -1, 0
	for si < len(v) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == v[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func toRules(rules []sessionspec.PermissionRule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		out = append(out, Rule{Action: r.Action, Resource: r.Resource, Effect: string(r.Effect)})
	}
	return out
}

func hasPlugin(b sessionspec.BundleSpec, id string) bool {
	for _, p := range b.Plugins {
		if p.ID == id {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
