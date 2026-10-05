package opencodev2

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Attest checks the closed world invariant on a running server: everything
// the model can see at location must come from the bundle.
//
//   - agents: non-hidden agents ⊆ bundle agents; hidden agents ⊆ system agents ∪ bundle;
//   - skills: listed skills (the list honours global permission rules) ⊆ bundle skills;
//   - MCP: configured servers ⊆ bundle MCP servers.
func Attest(ctx context.Context, c *Client, b sessionspec.BundleSpec, location string) (adapters.VisibilityReport, error) {
	rep := adapters.VisibilityReport{Level: sessionspec.IsolationFull}

	agents, err := c.Agents(ctx, location)
	if err != nil {
		return rep, fmt.Errorf("listing agents: %w", err)
	}
	if len(agents) == 0 {
		return rep, fmt.Errorf("no agent listed at %s (server not ready?)", location)
	}
	inBundle := set(b.AgentIDs())
	for _, a := range agents {
		if a.Hidden && SystemAgents[a.ID] {
			continue
		}
		rep.Agents = append(rep.Agents, a.ID)
		if !inBundle[a.ID] {
			rep.Unexpected = append(rep.Unexpected, "agent:"+a.ID)
		}
	}
	for _, id := range b.AgentIDs() {
		if !hasAgent(agents, id) {
			return rep, fmt.Errorf("bundle agent %q is not loaded by the server", id)
		}
	}

	// The skill registry lists every discovered skill regardless of
	// permissions; visibility to the model is enforced by the rendered rules
	// (deny "*" then allow bundle skills). A skill outside the bundle is only
	// acceptable if every bundle agent denies it.
	skills, err := waitSkills(ctx, c, location, b.SkillIDs())
	if err != nil {
		return rep, err
	}
	skillSet := set(b.SkillIDs())
	for _, s := range skills {
		if skillSet[s.ID] {
			rep.Skills = append(rep.Skills, s.ID)
			continue
		}
		if visibleTo := skillVisibleTo(b, s.ID); len(visibleTo) > 0 {
			rep.Skills = append(rep.Skills, s.ID)
			rep.Unexpected = append(rep.Unexpected, "skill:"+s.ID)
		}
	}

	servers, err := c.MCP(ctx, location)
	if err != nil {
		return rep, fmt.Errorf("listing mcp servers: %w", err)
	}
	mcpSet := map[string]bool{}
	for _, m := range b.MCP {
		mcpSet[m.Name] = true
	}
	for _, m := range servers {
		rep.MCP = append(rep.MCP, m.Name)
		if !mcpSet[m.Name] {
			rep.Unexpected = append(rep.Unexpected, "mcp:"+m.Name)
		}
	}

	// Plugins loaded from the user's global config also run in oh sessions.
	// They are reported as warnings: they do not add agents or skills (that is
	// checked above) but may alter prompts or tools.
	if plugins, err := c.Plugins(ctx, location); err == nil {
		allowed := map[string]bool{OhPluginID: true}
		for _, p := range b.Plugins {
			allowed[p.ID] = true
		}
		for _, p := range plugins {
			if p.Source.Type == "builtin" || allowed[p.ID] {
				continue
			}
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("plugin:%s (%s)", p.ID, p.Source.Path))
		}
	}

	sort.Strings(rep.Unexpected)
	if !rep.OK() {
		rep.Level = sessionspec.IsolationNone
	}
	return rep, nil
}

// waitSkills polls the skill registry until every bundle skill is listed
// (skills load asynchronously after the agents).
func waitSkills(ctx context.Context, c *Client, location string, want []string) ([]Skill, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		skills, err := c.Skills(ctx, location)
		if err != nil {
			return nil, fmt.Errorf("listing skills: %w", err)
		}
		got := map[string]bool{}
		for _, s := range skills {
			got[s.ID] = true
		}
		missing := ""
		for _, id := range want {
			if !got[id] {
				missing = id
				break
			}
		}
		if missing == "" {
			return skills, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("bundle skill %q is not loaded by the server", missing)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// skillVisibleTo returns the bundle agents whose rendered rules allow the skill.
func skillVisibleTo(b sessionspec.BundleSpec, skill string) []string {
	global := globalRules(b, b.SkillIDs())
	var out []string
	for _, a := range b.Agents {
		rules := append(append([]sessionspec.PermissionRule(nil), global...), agentRules(a, b, b.SkillIDs())...)
		if evaluate(rules, sessionspec.ActionSkill, skill, sessionspec.EffectAllow) != sessionspec.EffectDeny {
			out = append(out, a.ID)
		}
	}
	return out
}

// UnexpectedAgents returns the agent IDs reported as unexpected.
func UnexpectedAgents(rep adapters.VisibilityReport) []string {
	var out []string
	for _, u := range rep.Unexpected {
		if len(u) > 6 && u[:6] == "agent:" {
			out = append(out, u[6:])
		}
	}
	return out
}

func set(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}
