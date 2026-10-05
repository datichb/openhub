// Package hubcat exposes the hub content (agents, skills, workflows and
// prompt templates extracted under ~/.oh/hub) to the workflow engine.
//
// It lives outside package workflow because it reads agent frontmatter
// through package deploy, which itself depends on workflow.
package hubcat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowsDir is the hub directory holding the shipped workflows.
const WorkflowsDir = "workflows"

// Catalog reads the hub content directory. It implements
// workflow.AgentCatalog, workflow.SkillCatalog and workflow.PromptSource
// (hub layer).
type Catalog struct {
	hubDir string
	files  map[string]string // agent id → file

	mu     sync.Mutex
	agents map[string]agentEntry
}

type agentEntry struct {
	info workflow.AgentInfo
	ok   bool
}

// New indexes the agents of hubDir.
func New(hubDir string) (*Catalog, error) {
	files, err := deploy.FindAgentFiles(hubDir)
	if err != nil {
		return nil, fmt.Errorf("listing hub agents: %w", err)
	}
	return &Catalog{hubDir: hubDir, files: files, agents: map[string]agentEntry{}}, nil
}

// Env returns a validation environment backed by the hub.
func (c *Catalog) Env() workflow.Env {
	return workflow.Env{Agents: c, Skills: c, Prompts: c}
}

// AgentIDs returns the ids of every hub agent.
func (c *Catalog) AgentIDs() []string {
	out := make([]string, 0, len(c.files))
	for id := range c.files {
		out = append(out, id)
	}
	return out
}

// Agent implements workflow.AgentCatalog.
func (c *Catalog) Agent(id string) (workflow.AgentInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.agents[id]; ok {
		return e.info, e.ok
	}
	info, ok := c.load(id)
	c.agents[id] = agentEntry{info: info, ok: ok}
	return info, ok
}

func (c *Catalog) load(id string) (workflow.AgentInfo, bool) {
	file, ok := c.files[id]
	if !ok {
		return workflow.AgentInfo{}, false
	}
	fm, err := deploy.ParseAgentFrontmatter(file)
	if err != nil {
		// The agent exists; an unreadable frontmatter is reported elsewhere
		// (bundle build). Assume the most permissive shape.
		return workflow.AgentInfo{ID: id, Mode: workflow.ModePrimary, Edits: true, Shell: true}, true
	}
	perms, err := deploy.ResolvePermissions(c.hubDir, fm)
	if err != nil {
		perms = fm.Permission
	}
	return AgentInfoFrom(id, fm, perms), true
}

// AgentInfoFrom derives the workflow view of an agent from its frontmatter
// and resolved permissions (V1 shape). A permission that is not written is
// allowed by the tool, so it counts as granted, except `task`: only the
// delegations an agent declares are taken into account.
func AgentInfoFrom(id string, fm *deploy.AgentFrontmatter, perms map[string]any) workflow.AgentInfo {
	info := workflow.AgentInfo{ID: id, Mode: workflow.ModePrimary}
	if fm.Mode == string(workflow.ModeSubagent) {
		info.Mode = workflow.ModeSubagent
	}
	written := false
	for _, k := range []string{"edit", "write", "patch"} {
		if v, ok := perms[k]; ok {
			written = true
			if grantsAny(v) {
				info.Edits = true
			}
		}
	}
	if !written {
		info.Edits = true
	}
	info.Shell = true
	if v, ok := perms["bash"]; ok {
		info.Shell = grantsWildcard(v)
	}
	switch v := perms["task"].(type) {
	case map[string]any:
		for pattern, eff := range v {
			if pattern != id && effect(eff) != "deny" {
				info.Tasks = append(info.Tasks, pattern)
			}
		}
		sort.Strings(info.Tasks)
	case nil:
	default:
		if effect(v) != "deny" {
			info.Tasks = []string{"*"}
		}
	}
	info.Skills = append(append([]string(nil), fm.Skills...), fm.NativeSkills...)
	return info
}

// grantsAny reports whether a permission value allows (or asks for) anything.
func grantsAny(v any) bool {
	if m, ok := v.(map[string]any); ok {
		for _, e := range m {
			if effect(e) != "deny" {
				return true
			}
		}
		return false
	}
	return effect(v) != "deny"
}

// grantsWildcard reports whether a permission value allows any command:
// a plain non-deny value, or a pattern map whose "*" is not denied.
func grantsWildcard(v any) bool {
	if m, ok := v.(map[string]any); ok {
		e, ok := m["*"]
		return !ok || effect(e) != "deny"
	}
	return effect(v) != "deny"
}

func effect(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "allow"
		}
		return "deny"
	case string:
		switch strings.ToLower(x) {
		case "deny", "false":
			return "deny"
		case "ask":
			return "ask"
		}
		return "allow"
	}
	return "allow"
}

// HasSkill implements workflow.SkillCatalog (hub skills, then community).
func (c *Catalog) HasSkill(ref string) bool {
	_, err := deploy.SkillSourcePath(c.hubDir, ref)
	return err == nil
}

// Closure implements workflow.SkillCatalog. Skill dependencies (`requires:`)
// are not declared yet (P1-T08): the closure is the deduplicated roots, and
// unknown roots are reported as missing.
func (c *Catalog) Closure(roots []string) ([]string, []workflow.SkillIssue) {
	var out []string
	var issues []workflow.SkillIssue
	seen := map[string]bool{}
	for _, r := range roots {
		if seen[r] {
			continue
		}
		seen[r] = true
		if !c.HasSkill(r) {
			issues = append(issues, workflow.SkillIssue{Skill: r, Kind: "missing", Detail: r})
			continue
		}
		out = append(out, r)
	}
	return out, issues
}

// ReadPrompt implements workflow.PromptSource. Templates are read relative
// to the directory of the document that sets them (<hub>/workflows for the
// shipped workflows), or to <hub>/workflows for in-memory documents.
func (c *Catalog) ReadPrompt(origin workflow.Origin, path string) ([]byte, error) {
	dir := filepath.Join(c.hubDir, WorkflowsDir)
	if origin.Source != "" {
		dir = filepath.Dir(origin.Source)
	}
	return readUnder(dir, path)
}

// readUnder reads rel inside dir, refusing paths that escape it.
func readUnder(dir, rel string) ([]byte, error) {
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if r, err := filepath.Rel(dir, full); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("prompt path %q escapes %s", rel, dir)
	}
	return os.ReadFile(full)
}

// LoadWorkflows parses <hub>/workflows/*.yaml into a catalog (hub layer).
// A missing directory gives an empty catalog.
func LoadWorkflows(hubDir string) (*workflow.MemCatalog, workflow.Diagnostics) {
	cat := workflow.NewMemCatalog()
	dir := filepath.Join(hubDir, WorkflowsDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return cat, nil
	}
	diags := cat.LoadDir(dir, workflow.LayerHub)
	return cat, diags
}
