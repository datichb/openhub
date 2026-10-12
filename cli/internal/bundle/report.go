package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	// InlinedTokens is the part of Tokens taken by the inlined skills.
	InlinedTokens int `json:"inlined_tokens,omitempty"`
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

// InlinedSkill is a skill inlined in an agent prompt.
type InlinedSkill struct {
	ID     string `json:"id"`
	Tokens int    `json:"tokens"`
}

var inlinedSkillRe = regexp.MustCompile("(?m)^> Skill `([a-z0-9_-]+)` : incluse ci-dessous")

// inlinedTail are the generated blocks appended after the inlined skills.
var inlinedTail = []string{
	"\n\n> Skills absentes de ce paquet de session",
	"\n\n## Rôle dans cette session\n\n> Généré par oh",
	"\n\n---\n\n# Project instructions\n",
}

// InlinedSkills splits a composed agent prompt by its inlined-skill headings:
// each skill runs from its heading to the next one, the last one up to the
// generated blocks (absent skills, session role, project instructions).
func InlinedSkills(body string) []InlinedSkill {
	locs := inlinedSkillRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]InlinedSkill, 0, len(locs))
	for i, l := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			for _, m := range inlinedTail {
				if j := strings.Index(body[l[0]:], m); j >= 0 && l[0]+j < end {
					end = l[0] + j
				}
			}
		}
		section := strings.TrimSuffix(body[l[0]:end], "\n\n---\n\n")
		out = append(out, InlinedSkill{ID: body[l[2]:l[3]], Tokens: EstimateTokens(section)})
	}
	return out
}

// SkillCatalogTokens estimates the catalogue entry of an on-demand skill, as
// the tool lists it to the model (name and description).
func SkillCatalogTokens(sk sessionspec.SkillDef) int {
	return EstimateTokens(sk.ID + ": " + sk.Description)
}

// VisibleSkills returns the bundle skills an agent may load: every one its
// own skill rules do not deny (as the opencode adapter renders them).
func VisibleSkills(a sessionspec.AgentDef, skills []sessionspec.SkillDef) []sessionspec.SkillDef {
	out := make([]sessionspec.SkillDef, 0, len(skills))
	for _, sk := range skills {
		if ToolEffect(a.Permissions, sessionspec.ActionSkill, sk.ID) != sessionspec.EffectDeny {
			out = append(out, sk)
		}
	}
	return out
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
		for _, in := range InlinedSkills(a.Body) {
			ar.InlinedTokens += in.Tokens
		}
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
		r.Budget.SkillCatalog += SkillCatalogTokens(sk)
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
