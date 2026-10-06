package bundle

import (
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Report describes a compiled bundle (BundleService.Show): what the session
// sees and an estimate of its prompt budget.
type Report struct {
	Hash            string                       `json:"hash"`
	Dir             string                       `json:"dir"`
	EntryAgent      string                       `json:"entry_agent"`
	DefaultModel    string                       `json:"default_model,omitempty"`
	Agents          []AgentReport                `json:"agents"`
	Skills          []SkillReport                `json:"skills"`
	MCP             []string                     `json:"mcp"`
	Plugins         []string                     `json:"plugins"`
	Permissions     []sessionspec.PermissionRule `json:"permissions,omitempty"`
	MaxDepth        int                          `json:"max_depth"`
	CodeMode        bool                         `json:"code_mode"`
	Isolation       sessionspec.IsolationLevel   `json:"isolation"`
	StrictIsolation bool                         `json:"strict_isolation"`
	Budget          Budget                       `json:"budget"`
}

// AgentReport is an agent of the bundle.
type AgentReport struct {
	ID          string   `json:"id"`
	Entry       bool     `json:"entry,omitempty"`
	Mode        string   `json:"mode"`
	Description string   `json:"description,omitempty"`
	Model       string   `json:"model,omitempty"`
	Calls       []string `json:"calls,omitempty"`
	// Tokens estimates the agent prompt (body with inlined skills).
	Tokens int `json:"tokens"`
}

// SkillReport is an on-demand skill of the bundle.
type SkillReport struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	// Tokens estimates SKILL.md, loaded when the agent opens the skill.
	Tokens int `json:"tokens"`
}

// Budget estimates the prompt size of a session, in tokens (≈ 4 characters
// per token). The real system prompt adds the tool's own blocks.
type Budget struct {
	// EntryAgent is the prompt of the entry agent (always loaded).
	EntryAgent int `json:"entry_agent"`
	// Agents sums every agent prompt (each one loaded when the agent runs).
	Agents int `json:"agents"`
	// SkillCatalog is the list of skill names and descriptions shown to the
	// model at every turn.
	SkillCatalog int `json:"skill_catalog"`
	// Skills sums the on-demand skill bodies (loaded only when opened).
	Skills int `json:"skills"`
	// Initial is what the first turn costs: entry agent + skill catalogue.
	Initial int `json:"initial"`
}

// EstimateTokens is the token estimate of a text (≈ 4 characters per token).
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	return (n + 3) / 4
}

// Show reports the content and the estimated budget of a bundle.
func Show(b *Bundle) *Report {
	s := b.Spec
	r := &Report{Hash: s.Hash, Dir: b.Dir, EntryAgent: s.EntryAgent, Permissions: s.Permissions,
		MaxDepth: s.MaxDepth, CodeMode: s.CodeMode, Isolation: s.Isolation, StrictIsolation: s.StrictIsolation,
		MCP: []string{}, Plugins: []string{}, Agents: []AgentReport{}, Skills: []SkillReport{}}
	if s.DefaultModel != nil {
		r.DefaultModel = s.DefaultModel.String()
	}
	for _, a := range s.Agents {
		ar := AgentReport{ID: a.ID, Entry: a.ID == s.EntryAgent, Mode: a.Mode, Description: a.Description,
			Calls: s.SubagentGraph[a.ID], Tokens: EstimateTokens(a.Body)}
		if a.Model != nil {
			ar.Model = a.Model.String()
		}
		r.Agents = append(r.Agents, ar)
		r.Budget.Agents += ar.Tokens
		if ar.Entry {
			r.Budget.EntryAgent = ar.Tokens
		}
	}
	sort.SliceStable(r.Agents, func(i, j int) bool { return r.Agents[i].Entry && !r.Agents[j].Entry })
	for _, sk := range s.Skills {
		sr := SkillReport{ID: sk.ID, Description: sk.Description}
		if data, err := os.ReadFile(filepath.Join(b.Dir, skillsDir, sk.ID, "SKILL.md")); err == nil {
			_, body := bricks.SplitFrontmatter(data)
			sr.Tokens = EstimateTokens(string(body))
		}
		r.Skills = append(r.Skills, sr)
		r.Budget.Skills += sr.Tokens
		r.Budget.SkillCatalog += EstimateTokens(sk.ID + ": " + sk.Description)
	}
	for _, m := range s.MCP {
		r.MCP = append(r.MCP, m.Name)
	}
	for _, p := range s.Plugins {
		r.Plugins = append(r.Plugins, p.ID)
	}
	r.Budget.Initial = r.Budget.EntryAgent + r.Budget.SkillCatalog
	return r
}
