// Package bundle compiles the content of a session (agents, skills,
// permissions, graph) into an immutable, content-addressed directory under
// ~/.oh/bundles/<hash>, independent of the project directory.
//
// Phase 0 builder: agents are selected from the current (single) hub
// workflow, starting at the entry agent and following the workflow
// delegation graph. Declarative workflows (phase 1) will replace the
// selection step; the assembly and layout stay.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Request describes what to compile.
type Request struct {
	HubDir      string // ~/.oh/hub (agents/, skills/, permissions/)
	OutDir      string // ~/.oh/bundles
	ProjectPath string // for stack skills and project instructions (may be empty)
	EntryAgent  string

	// Workflow is the resolved hub workflow (graph + generated skills). Nil = base workflow.
	Workflow *deploy.WorkflowDeployResult

	// Model resolution (cascade levels, nil = none) and hub provider name ("bedrock"…).
	Provider         string
	ProjectOverrides *deploy.ModelOverrides
	HubOverrides     *deploy.ModelOverrides
	TeamOverrides    *deploy.ModelOverrides

	ExtraInstructionFiles []string
	MCP                   []sessionspec.MCPServerDef
	Plugins               []sessionspec.PluginDef
	WebsearchEnabled      bool
	CodeMode              bool
}

// Bundle is a compiled bundle on disk.
type Bundle struct {
	Spec sessionspec.BundleSpec
	Dir  string
}

const (
	specFile  = "bundle.json"
	agentsDir = "agents"
	skillsDir = "skills"
)

// Build compiles the bundle. It is idempotent: identical inputs produce the
// same hash and reuse the existing directory.
func Build(req Request) (*Bundle, error) {
	if req.HubDir == "" || req.OutDir == "" || req.EntryAgent == "" {
		return nil, fmt.Errorf("bundle: HubDir, OutDir and EntryAgent are required")
	}
	wf := req.Workflow
	if wf == nil {
		var err error
		if wf, err = deploy.ResolveAndPrepareWorkflow(workflow.BaseWorkflow()); err != nil {
			return nil, err
		}
	}

	files, err := deploy.FindAgentFiles(req.HubDir)
	if err != nil {
		return nil, fmt.Errorf("listing hub agents: %w", err)
	}
	if _, ok := files[req.EntryAgent]; !ok {
		return nil, fmt.Errorf("bundle: unknown entry agent %q", req.EntryAgent)
	}

	graph := deriveGraph(req.HubDir, &wf.Resolved, files)
	selected := reachable(req.EntryAgent, graph)

	instructions, err := readInstructions(req.ProjectPath, req.ExtraInstructionFiles)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(req.OutDir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(req.OutDir, ".build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	spec := sessionspec.BundleSpec{
		EntryAgent: req.EntryAgent,
		MCP:        req.MCP,
		Plugins:    req.Plugins,
		CodeMode:   req.CodeMode,
		Isolation:  sessionspec.IsolationFull,
	}
	if req.WebsearchEnabled {
		spec.Permissions = append(spec.Permissions,
			sessionspec.PermissionRule{Action: sessionspec.ActionWebSrch, Resource: "*", Effect: sessionspec.EffectAllow},
			sessionspec.PermissionRule{Action: sessionspec.ActionWebFetch, Resource: "*", Effect: sessionspec.EffectAllow},
		)
	}

	skillRefs := map[string]bool{}
	slots := slotIndex(&wf.Resolved)
	for _, id := range selected {
		a, err := deploy.AssembleAgent(req.HubDir, files[id], wf.GeneratedSkills)
		if err != nil {
			return nil, err
		}
		def, err := agentDef(req, a, slots[id], instructions)
		if err != nil {
			return nil, err
		}
		spec.Agents = append(spec.Agents, def)
		for _, ref := range a.Frontmatter.NativeSkills {
			skillRefs[ref] = true
		}
		if err := os.MkdirAll(filepath.Join(tmp, agentsDir), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(tmp, agentsDir, id+".md"), []byte(def.Body), 0o644); err != nil {
			return nil, err
		}
	}
	if req.ProjectPath != "" {
		for _, ref := range deploy.ResolveStackSkills(req.ProjectPath) {
			skillRefs[ref] = true
		}
	}
	skills, err := writeSkills(req.HubDir, filepath.Join(tmp, skillsDir), skillRefs, wf.GeneratedSkills)
	if err != nil {
		return nil, err
	}
	spec.Skills = skills

	spec.SubagentGraph = map[string][]string{}
	inBundle := map[string]bool{}
	for _, id := range selected {
		inBundle[id] = true
	}
	for _, id := range selected {
		var targets []string
		for _, t := range graph[id] {
			if inBundle[t] && t != id {
				targets = append(targets, t)
			}
		}
		if len(targets) > 0 {
			sort.Strings(targets)
			spec.SubagentGraph[id] = targets
		}
	}
	spec.MaxDepth = maxDepth(req.EntryAgent, spec.SubagentGraph)
	if def := findAgent(spec.Agents, req.EntryAgent); def != nil && def.Model != nil {
		m := *def.Model
		spec.DefaultModel = &m
	}

	hash, err := hashBundle(tmp, spec)
	if err != nil {
		return nil, err
	}
	final := filepath.Join(req.OutDir, hash)
	spec.Hash = hash
	spec.Root = final
	spec.SkillsDir = filepath.Join(final, skillsDir)
	for i := range spec.Skills {
		spec.Skills[i].Dir = filepath.Join(final, skillsDir, spec.Skills[i].ID)
	}

	if _, err := os.Stat(filepath.Join(final, specFile)); err == nil {
		return &Bundle{Spec: spec, Dir: final}, nil // already compiled
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(tmp, specFile), data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, final); err != nil {
		if _, statErr := os.Stat(filepath.Join(final, specFile)); statErr == nil {
			return &Bundle{Spec: spec, Dir: final}, nil // concurrent build won
		}
		return nil, fmt.Errorf("finalizing bundle: %w", err)
	}
	return &Bundle{Spec: spec, Dir: final}, nil
}

// Load reads a compiled bundle by hash.
func Load(outDir, hash string) (*Bundle, error) {
	dir := filepath.Join(outDir, hash)
	data, err := os.ReadFile(filepath.Join(dir, specFile))
	if err != nil {
		return nil, err
	}
	var spec sessionspec.BundleSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &Bundle{Spec: spec, Dir: dir}, nil
}

func agentDef(req Request, a *deploy.AssembledAgent, slot *workflow.AgentSlot, instructions string) (sessionspec.AgentDef, error) {
	fm := a.Frontmatter
	def := sessionspec.AgentDef{
		ID:          fm.ID,
		Description: fm.Description,
		Mode:        fm.Mode,
		Body:        a.Body + instructions,
	}
	if def.ID == "" {
		def.ID = strings.TrimSuffix(filepath.Base(a.Path), ".md")
	}
	if slot != nil {
		if slot.Mode == workflow.ModeSubagent {
			def.Mode = "subagent"
		} else if slot.Mode != "" {
			def.Mode = "primary"
		}
	}
	if def.Mode == "" {
		def.Mode = "primary"
	}
	if def.Description == "" {
		def.Description = fm.Label
	}

	if m := deploy.ResolveAgentModel(fm.ID, a.Family, req.ProjectOverrides, req.HubOverrides, req.TeamOverrides, fm.Model, req.Provider); m != "" {
		ref := sessionspec.ParseModelRef(m)
		if ref.Provider == "" {
			ref.Provider = deploy.OpencodeProviderID(req.Provider)
		}
		def.Model = &ref
	}

	perms, err := deploy.ResolvePermissions(req.HubDir, fm)
	if err != nil {
		slog.Warn("bundle: permission base resolution failed, using inline permissions", "agent", fm.ID, "error", err)
		perms = fm.Permission
	}
	def.Permissions = ConvertPermissions(perms)
	return def, nil
}

func findAgent(agents []sessionspec.AgentDef, id string) *sessionspec.AgentDef {
	for i := range agents {
		if agents[i].ID == id {
			return &agents[i]
		}
	}
	return nil
}

func slotIndex(wf *workflow.WorkflowDefinition) map[string]*workflow.AgentSlot {
	out := map[string]*workflow.AgentSlot{}
	for i := range wf.Agents {
		out[wf.Agents[i].AgentID] = &wf.Agents[i]
	}
	return out
}

// readInstructions concatenates the project instruction files (ONBOARDING.md,
// CONVENTIONS.md, .claude/CLAUDE.md + configured extras). opencode V2 ignores
// the `instructions` config key, so they are embedded in every agent body.
func readInstructions(projectPath string, extra []string) (string, error) {
	if projectPath == "" {
		return "", nil
	}
	var b strings.Builder
	for _, rel := range deploy.InstructionFiles(projectPath, extra) {
		data, err := os.ReadFile(filepath.Join(projectPath, rel))
		if err != nil {
			return "", fmt.Errorf("reading instruction file %s: %w", rel, err)
		}
		if b.Len() == 0 {
			b.WriteString("\n\n---\n\n# Project instructions\n")
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", rel, strings.TrimSpace(string(data)))
	}
	return b.String(), nil
}

// writeSkills copies Bucket B skills to <dst>/<id>/SKILL.md (generated
// workflow skills override static ones). Skill IDs are the ref base names.
func writeSkills(hubDir, dst string, refs map[string]bool, generated map[string]string) ([]sessionspec.SkillDef, error) {
	ordered := make([]string, 0, len(refs))
	for r := range refs {
		ordered = append(ordered, r)
	}
	sort.Strings(ordered)

	var out []sessionspec.SkillDef
	owner := map[string]string{}
	for _, ref := range ordered {
		id := filepath.Base(ref)
		if prev, dup := owner[id]; dup {
			slog.Warn("bundle: duplicate skill id, keeping first", "id", id, "kept", prev, "skipped", ref)
			continue
		}
		var content []byte
		if gen, ok := generated[ref]; ok {
			content = []byte(gen)
		} else {
			src, err := deploy.SkillSourcePath(hubDir, ref)
			if err != nil {
				slog.Warn("bundle: skill not found, skipped", "ref", ref, "error", err)
				continue
			}
			if content, err = os.ReadFile(src); err != nil {
				return nil, err
			}
		}
		dir := filepath.Join(dst, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), content, 0o644); err != nil {
			return nil, err
		}
		owner[id] = ref
		out = append(out, sessionspec.SkillDef{ID: id, Description: skillDescription(content), Dir: dir})
	}
	return out, nil
}

func skillDescription(content []byte) string {
	text := string(content)
	if !strings.HasPrefix(text, "---") {
		return ""
	}
	end := strings.Index(text[3:], "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(text[3:3+end], "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "description:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// hashBundle hashes the spec (without absolute paths) and every file of dir.
func hashBundle(dir string, spec sessionspec.BundleSpec) (string, error) {
	h := sha256.New()
	canon := spec
	canon.Root, canon.SkillsDir, canon.Hash = "", "", ""
	canon.Skills = append([]sessionspec.SkillDef(nil), spec.Skills...)
	for i := range canon.Skills {
		canon.Skills[i].Dir = ""
	}
	data, err := json.Marshal(canon)
	if err != nil {
		return "", err
	}
	h.Write(data)
	var paths []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if rel != specFile {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "\x00%s\x00%d\x00", filepath.ToSlash(rel), len(content))
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
