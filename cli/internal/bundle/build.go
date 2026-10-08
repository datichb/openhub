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

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Request describes what to compile.
type Request struct {
	HubDir      string // ~/.oh/hub (agents/, skills/, permissions/)
	OutDir      string // ~/.oh/bundles
	ProjectPath string // for stack skills and project instructions (may be empty)
	EntryAgent  string

	// Model resolution (cascade levels, nil = none) and hub provider name ("bedrock"…).
	// WorkflowModels is the workflow level (O9), see WorkflowModels().
	Provider string
	// ToolProvider is the provider id of the tool for Provider (adapter,
	// adapters.ProviderMapper; "" = Provider).
	ToolProvider     string
	WorkflowModels   *bricks.ModelOverrides
	ProjectOverrides *bricks.ModelOverrides
	HubOverrides     *bricks.ModelOverrides
	TeamOverrides    *bricks.ModelOverrides

	// ExtraSkills / DenySkills apply the workflow `skills:` block (refs or
	// identifiers for DenySkills). Requirements of extra skills are added.
	ExtraSkills []string
	DenySkills  []string

	ExtraInstructionFiles []string
	MCP                   []sessionspec.MCPServerDef
	Plugins               []sessionspec.PluginDef
	WebsearchEnabled      bool
	CodeMode              bool
	// StrictIsolation mirrors `isolation: strict` (no "always" grant from
	// oh); set from Spec when it requires strict isolation.
	StrictIsolation bool

	// Spec is the resolved oh/v1 workflow (required). It selects the agents
	// and the delegation graph (P1-T06), generates the chain skills (P1-T11)
	// and fills EntryAgent, WorkflowModels, ExtraSkills, DenySkills and
	// Plugins when they are not set; its `code_mode` sets CodeMode (P1-T15).
	Spec *workflow.Spec
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
	if req.Spec == nil {
		return nil, fmt.Errorf("bundle: a workflow (Spec) is required")
	}
	if err := req.applySpec(); err != nil {
		return nil, err
	}
	if req.HubDir == "" || req.OutDir == "" || req.EntryAgent == "" {
		return nil, fmt.Errorf("bundle: HubDir, OutDir and EntryAgent are required")
	}
	generated, err := specSkills(req.HubDir, req.Spec)
	if err != nil {
		return nil, err
	}

	files, err := bricks.FindAgentFiles(req.HubDir)
	if err != nil {
		return nil, fmt.Errorf("listing hub agents: %w", err)
	}
	if _, ok := files[req.EntryAgent]; !ok {
		return nil, fmt.Errorf("bundle: unknown entry agent %q", req.EntryAgent)
	}

	graph, selected, err := selectAgents(req, files)
	if err != nil {
		return nil, err
	}

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

		StrictIsolation: req.StrictIsolation,
	}
	applyWorkflowRuntime(&spec, req.Spec)
	if req.WebsearchEnabled {
		spec.Permissions = append(spec.Permissions,
			sessionspec.PermissionRule{Action: sessionspec.ActionWebSrch, Resource: "*", Effect: sessionspec.EffectAllow},
			sessionspec.PermissionRule{Action: sessionspec.ActionWebFetch, Resource: "*", Effect: sessionspec.EffectAllow},
		)
	}

	skillRefs := map[string]bool{}
	loader := newSkillLoader(req.HubDir, generated)
	denied := denyList(req.DenySkills)
	mentionIndex := hubSkillIndex(req.HubDir, generated)
	ids := skillIndex{}
	for _, id := range selected {
		fm, err := bricks.ParseAgentFrontmatter(files[id])
		if err != nil {
			return nil, err
		}
		inline, err := loader.closure(fm.Skills, denied)
		if err != nil {
			return nil, fmt.Errorf("agent %s: %w", id, err)
		}
		inlined := map[string]bool{}
		bodies := make([][]byte, 0, len(inline))
		for _, d := range inline {
			if err := ids.add(d); err != nil {
				return nil, err
			}
			files, err := loader.annexes(d)
			if err != nil {
				return nil, err
			}
			if err := writeAnnexes(filepath.Join(tmp, skillsDir), d, files); err != nil {
				return nil, err
			}
			inlined[d.ID] = true
			bodies = append(bodies, append([]byte(inlinedSkillHeading(d.ID)), inlineAnnexRefs(d.body(), d, files)...))
		}
		a, err := bricks.AssembleAgentInline(req.HubDir, files[id], bodies)
		if err != nil {
			return nil, err
		}
		for _, ref := range mentionedSkills(a.Body, mentionIndex) {
			if d, err := loader.load(ref); err == nil && !inlined[d.ID] && deliverable(loader, ref, denied) {
				skillRefs[ref] = true
			}
		}
		def, err := agentDef(req, a, instructions)
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
		if err := os.WriteFile(filepath.Join(tmp, agentsDir, id+".md"), []byte(def.Body), bundleFileMode); err != nil {
			return nil, err
		}
	}
	if req.ProjectPath != "" {
		for _, ref := range bricks.ResolveStackSkills(req.ProjectPath) {
			skillRefs[ref] = true
		}
	}
	for _, ref := range req.ExtraSkills {
		skillRefs[ref] = true
	}
	native, err := loader.closure(sortedKeys(skillRefs), denied)
	if err != nil {
		return nil, err
	}
	for _, d := range native {
		if err := ids.add(d); err != nil {
			return nil, err
		}
		if err := checkName(d); err != nil {
			return nil, err
		}
		files, err := loader.annexes(d)
		if err != nil {
			return nil, err
		}
		if err := writeAnnexes(filepath.Join(tmp, skillsDir), d, files); err != nil {
			return nil, err
		}
	}
	skills, err := writeSkills(filepath.Join(tmp, skillsDir), native)
	if err != nil {
		return nil, err
	}
	spec.Skills = skills

	applyWorkflowModes(spec.Agents, req.Spec)
	spec.SubagentGraph = bundleGraph(graph, selected)
	spec.MaxDepth = workflow.MaxDepth(req.EntryAgent, spec.SubagentGraph)
	if def := findAgent(spec.Agents, req.EntryAgent); def != nil && def.Model != nil {
		m := *def.Model
		spec.DefaultModel = &m
	} else if m := req.ResolveModel(req.EntryAgent, "", fallbackModel); m != "" && req.Provider != "" {
		// Without a session model the tool picks its own default for the
		// provider (e.g. Bedrock: a non-Anthropic model).
		spec.DefaultModel = req.modelRef(m)
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
	if err := os.WriteFile(filepath.Join(tmp, specFile), data, bundleFileMode); err != nil {
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

func agentDef(req Request, a *bricks.AssembledAgent, instructions string) (sessionspec.AgentDef, error) {
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
	if def.Mode == "" {
		def.Mode = "primary"
	}
	if def.Description == "" {
		def.Description = fm.Label
	}

	if m := req.ResolveModel(fm.ID, a.Family, fm.Model); m != "" {
		def.Model = req.modelRef(m)
	}

	perms, err := bricks.ResolvePermissions(req.HubDir, fm)
	if err != nil {
		slog.Warn("bundle: permission base resolution failed, using inline permissions", "agent", fm.ID, "error", err)
		perms = fm.Permission
	}
	own := ConvertPermissions(perms)
	def.Permissions = append(append(append(own, GitShellGuard(own)...), BeadsShellGuard()...), HookGuard()...)
	return def, nil
}

// fallbackModel is the session model when neither the entry agent nor the
// overrides set one.
const fallbackModel = "claude-sonnet-4-6"

func findAgent(agents []sessionspec.AgentDef, id string) *sessionspec.AgentDef {
	for i := range agents {
		if agents[i].ID == id {
			return &agents[i]
		}
	}
	return nil
}

// readInstructions concatenates the project instruction files (ONBOARDING.md,
// CONVENTIONS.md, .claude/CLAUDE.md + configured extras). the tool ignores
// the `instructions` config key, so they are embedded in every agent body.
func readInstructions(projectPath string, extra []string) (string, error) {
	if projectPath == "" {
		return "", nil
	}
	var b strings.Builder
	for _, rel := range bricks.InstructionFiles(projectPath, extra) {
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
