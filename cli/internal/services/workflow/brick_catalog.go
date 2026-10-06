package workflow

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
)

// Brick catalogue (P3-T27, read-only view `project.agents`): the agents and
// skills a session may receive, with their origin (hub or team catalogue),
// their estimated cost and the workflows that use them. It replaces the
// per-project agent selection of the former deployment.

// BrickKind is the kind of a brick.
type BrickKind string

const (
	BrickAgent BrickKind = "agent"
	BrickSkill BrickKind = "skill"
)

// BrickOrigin is where a brick comes from.
type BrickOrigin string

const (
	OriginHub  BrickOrigin = "hub"
	OriginTeam BrickOrigin = "team"
)

// BrickEntry is an agent or a skill of the catalogue.
type BrickEntry struct {
	Kind BrickKind `json:"kind"`
	// ID is the agent id or the skill ref (path under skills/, without .md).
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"` // skill name (identifier of the tool)
	Label       string      `json:"label,omitempty"`
	Description string      `json:"description,omitempty"`
	Family      string      `json:"family,omitempty"` // agent family (folder)
	Mode        string      `json:"mode,omitempty"`   // agent mode
	Origin      BrickOrigin `json:"origin"`
	// Skills are the skills an agent loads (inlined, then on demand).
	Skills []string `json:"skills,omitempty"`
	// Requires are the skills a skill depends on.
	Requires []string `json:"requires,omitempty"`
	// Tokens estimates the cost of the brick (body, ~4 characters/token).
	Tokens int `json:"tokens"`
	// Agents lists the agents that load a skill.
	Agents []string `json:"agents,omitempty"`
	// Workflows lists the catalogue workflows that ship the brick.
	Workflows []string `json:"workflows,omitempty"`
}

type skillFront struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
}

// BrickCatalog lists the bricks of context c (hub, merged with the team
// catalogue when the context has one), agents first, then skills, by id.
func (s *Service) BrickCatalog(ctx context.Context, c Context) ([]BrickEntry, error) {
	b, err := s.Bricks(ctx, c)
	if err != nil {
		return nil, err
	}
	team := map[string]bool{}
	for _, t := range b.Team {
		team[t] = true
	}
	origin := func(kind BrickKind, id string) BrickOrigin {
		if team[string(kind)+":"+id] {
			return OriginTeam
		}
		return OriginHub
	}

	files, err := bricks.FindAgentFiles(b.Dir)
	if err != nil {
		return nil, err
	}
	agents := map[string]*BrickEntry{}
	for id, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fm, err := bricks.ParseAgentFrontmatterFromBytes(data)
		if err != nil {
			continue
		}
		_, body := bricks.SplitFrontmatter(data)
		rel, _ := filepath.Rel(filepath.Join(b.Dir, "agents"), path)
		agents[id] = &BrickEntry{Kind: BrickAgent, ID: id, Label: fm.Label, Description: fm.Description,
			Family: bricks.AgentFamily(filepath.ToSlash(rel)), Mode: modeOr(fm.Mode), Origin: origin(BrickAgent, id),
			Skills: append(append([]string(nil), fm.Skills...), fm.NativeSkills...), Tokens: bundle.EstimateTokens(string(body))}
	}

	skills := map[string]*BrickEntry{}
	root := filepath.Join(b.Dir, "skills")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "templates" {
				return fs.SkipDir // annexes, not skills
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // unreadable skills are skipped
		}
		fmBytes, body := bricks.SplitFrontmatter(data)
		var fm skillFront
		if len(fmBytes) > 0 {
			_ = yaml.Unmarshal(trimFences(fmBytes), &fm)
		}
		ref := strings.TrimSuffix(rel, ".md")
		skills[ref] = &BrickEntry{Kind: BrickSkill, ID: ref, Name: fm.Name, Description: fm.Description,
			Requires: fm.Requires, Origin: origin(BrickSkill, ref), Tokens: bundle.EstimateTokens(string(body))}
		return nil
	})

	for id, a := range agents {
		for _, ref := range a.Skills {
			if sk, ok := skills[ref]; ok {
				sk.Agents = append(sk.Agents, id)
			}
		}
	}
	s.markWorkflowUses(ctx, c, b.Dir, agents, skills)

	out := make([]BrickEntry, 0, len(agents)+len(skills))
	for _, m := range []map[string]*BrickEntry{agents, skills} {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			e := m[k]
			sort.Strings(e.Agents)
			sort.Strings(e.Workflows)
			out = append(out, *e)
		}
	}
	return out, nil
}

// markWorkflowUses fills Workflows: the members of each valid catalogue
// workflow and the skill closure of their agents (with `skills.extra`).
func (s *Service) markWorkflowUses(ctx context.Context, c Context, dir string, agents, skills map[string]*BrickEntry) {
	list, err := s.Catalog(ctx, c)
	if err != nil {
		return
	}
	closure := bundle.NewSkillCatalog(dir)
	for _, sum := range list {
		if !sum.Valid {
			continue
		}
		res, err := s.Resolve(ctx, c, sum.Ref, ResolveOpts{})
		if err != nil || res == nil {
			continue
		}
		var roots []string
		for _, id := range res.Spec.Members() {
			a, ok := agents[id]
			if !ok {
				continue
			}
			a.Workflows = appendOnce(a.Workflows, sum.ID)
			roots = append(roots, a.Skills...)
		}
		if res.Spec.Skills != nil {
			roots = append(roots, res.Spec.Skills.Extra...)
		}
		refs, _ := closure.Closure(roots)
		for _, ref := range refs {
			if sk, ok := skills[ref]; ok {
				sk.Workflows = appendOnce(sk.Workflows, sum.ID)
			}
		}
	}
}

func appendOnce(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func modeOr(m string) string {
	if m == "" {
		return "primary"
	}
	return m
}

// trimFences removes the "---" delimiters of a frontmatter block.
func trimFences(fm []byte) []byte {
	s := strings.TrimSpace(string(fm))
	s = strings.TrimPrefix(s, "---")
	s = strings.TrimSuffix(s, "---")
	return []byte(s)
}
