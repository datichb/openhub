package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// Team brick catalogue (P2-T07): agents and skills of the team-state
// catalog/ folder, used by the workflows like the hub bricks.
//
//	catalog/agents/<family>/<id>.md     same format as the hub agents
//	catalog/skills/<path>.md            same format as the hub skills
//	catalog/skills/templates/…          their annexes
//
// An identifier already used by the hub (agent id, skill ref or skill name)
// is refused unless the brick declares `extends: hub:<id|ref>` in its
// frontmatter: it then replaces the hub brick. A refused brick is skipped
// with a warning. The hub and the team bricks are merged into a content
// directory cached under BricksCacheDir, used for validation and bundles.

// brickDirs are the hub folders the merged directory holds.
var brickDirs = []string{"agents", "skills", "permissions"}

// Brick diagnostic codes (warnings, messages "teamstate.workflow.brick.<code>").
const (
	DiagBrickCollision     = "brick_collision"
	DiagBrickBadExtends    = "brick_bad_extends"
	DiagBrickDuplicate     = "brick_duplicate"
	DiagBrickAnnexConflict = "brick_annex_conflict"
)

// Bricks is the brick catalogue of a context.
type Bricks struct {
	// Dir is the content directory (hub alone, or hub merged with the team
	// catalogue).
	Dir string `json:"dir"`
	// Team lists the team bricks in use ("agent:<id>", "skill:<ref>").
	Team []string `json:"team,omitempty"`
	// Diagnostics are the refused team bricks (warnings).
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
}

func brickDiag(code, source string, args ...any) wf.Diagnostic {
	return wf.Diagnostic{Severity: wf.SeverityWarning, Code: code, Source: source,
		Message: i18n.Tf("teamstate.workflow.brick."+code, args...)}
}

// BricksDir returns the hub merged with the team catalogue when the
// resolution used team bricks ("" otherwise: the hub): bundles must be
// built from it.
func (r *Resolution) BricksDir() string {
	if t, ok := r.env.Agents.(teamAgents); ok {
		return t.Dir()
	}
	return ""
}

// teamAgents marks an agent catalogue merged with team bricks.
type teamAgents struct{ *hubcat.Catalog }

// Bricks returns the brick catalogue of c.
func (s *Service) Bricks(ctx context.Context, c Context) (*Bricks, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	if cat.bricks != nil {
		return cat.bricks, nil
	}
	return &Bricks{Dir: s.HubDir}, nil
}

// useTeamBricks merges the team catalogue of ts into the validation
// environment of cat (no-op without team bricks).
func (s *Service) useTeamBricks(cat *catalog, ts *TeamState) error {
	team := filepath.Join(ts.Repo.Path(), teamstate.CatalogDirName)
	if s.HubDir == "" || !hasFiles(team) {
		return nil
	}
	b, err := mergeBricks(s.HubDir, team, s.bricksCacheDir())
	if err != nil {
		return err
	}
	cat.bricks = b
	cat.diags = append(cat.diags, b.Diagnostics...)
	hc, err := hubcat.New(b.Dir)
	if err != nil {
		return err
	}
	prompts := cat.env.Prompts
	cat.env.Agents = teamAgents{hc}
	cat.env.Skills = bundle.NewSkillCatalog(b.Dir)
	cat.env.Prompts = prompts
	return nil
}

func (s *Service) bricksCacheDir() string {
	if s.BricksCacheDir != "" {
		return s.BricksCacheDir
	}
	return filepath.Join(filepath.Dir(s.HubDir), "cache", "bricks")
}

func hasFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() != ".gitkeep" {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// teamBrick is a file of the team catalogue.
type teamBrick struct {
	kind    string // agent | skill | annex
	id      string // agent id, skill ref, annex path (relative to skills/)
	src     string // absolute path
	extends string // frontmatter extends ("" = none)
	data    []byte
}

// mergeBricks builds (or reuses) the hub + team content directory.
func mergeBricks(hubDir, teamDir, cacheRoot string) (*Bricks, error) {
	bricks, err := readTeamBricks(teamDir)
	if err != nil {
		return nil, err
	}
	hubAgents, err := listFiles(filepath.Join(hubDir, "agents"), ".md")
	if err != nil {
		return nil, err
	}
	hubSkills, err := listFiles(filepath.Join(hubDir, "skills"), "")
	if err != nil {
		return nil, err
	}
	agentPath := map[string]string{} // agent id → rel path in agents/
	for _, rel := range hubAgents {
		agentPath[strings.TrimSuffix(filepath.Base(rel), ".md")] = rel
	}
	skillName := map[string]string{} // skill name (basename) → ref
	for _, rel := range hubSkills {
		if strings.HasSuffix(rel, ".md") && !strings.HasPrefix(rel, "templates"+string(filepath.Separator)) {
			ref := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
			skillName[filepath.Base(ref)] = ref
		}
	}

	out := &Bricks{}
	type placed struct {
		dst  string
		data []byte
	}
	var files []placed
	seen := map[string]string{} // brick key → source (duplicates in the team catalogue)
	for _, b := range bricks {
		switch b.kind {
		case "agent":
			key := "agent:" + b.id
			if prev, dup := seen[key]; dup {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickDuplicate, b.src, key, prev))
				continue
			}
			seen[key] = b.src
			dst := filepath.Join("agents", "team", b.id+".md")
			if hubRel, clash := agentPath[b.id]; clash {
				if b.extends != "hub:"+b.id {
					out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickCollision, b.src, key, "hub:"+b.id))
					continue
				}
				dst = filepath.Join("agents", hubRel)
			} else if b.extends != "" {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickBadExtends, b.src, key, b.extends))
				continue
			}
			files = append(files, placed{dst, stripExtends(b.data)})
			out.Team = append(out.Team, key)
		case "skill":
			name := filepath.Base(b.id)
			key := "skill:" + b.id
			if prev, dup := seen["skill-name:"+name]; dup {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickDuplicate, b.src, key, prev))
				continue
			}
			seen["skill-name:"+name] = b.src
			dst := filepath.Join("skills", filepath.FromSlash(b.id)+".md")
			if hubRef, clash := skillName[name]; clash {
				if b.extends != "hub:"+hubRef {
					out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickCollision, b.src, key, "hub:"+hubRef))
					continue
				}
				dst = filepath.Join("skills", filepath.FromSlash(hubRef)+".md")
				key = "skill:" + hubRef
			} else if b.extends != "" {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickBadExtends, b.src, key, b.extends))
				continue
			}
			files = append(files, placed{dst, stripExtends(b.data)})
			out.Team = append(out.Team, key)
		case "annex":
			dst := filepath.Join("skills", b.id)
			if _, err := os.Stat(filepath.Join(hubDir, dst)); err == nil {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickAnnexConflict, b.src, filepath.ToSlash(b.id)))
				continue
			}
			files = append(files, placed{dst, b.data})
		}
	}
	sort.Strings(out.Team)

	// Cache key: the hub tree (paths, sizes, dates) and the placed files.
	h := sha256.New()
	for _, d := range brickDirs {
		_ = filepath.WalkDir(filepath.Join(hubDir, d), func(p string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				if info, ierr := e.Info(); ierr == nil {
					fmt.Fprintf(h, "%s\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano())
				}
			}
			return nil
		})
	}
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00", f.dst)
		h.Write(f.data)
	}
	key := hex.EncodeToString(h.Sum(nil))[:24]
	out.Dir = filepath.Join(cacheRoot, key)
	if st, err := os.Stat(out.Dir); err == nil && st.IsDir() {
		return out, nil
	}

	tmp, err := os.MkdirTemp(cacheRoot0(cacheRoot), ".bricks-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	for _, d := range brickDirs {
		if err := copyTree(filepath.Join(hubDir, d), filepath.Join(tmp, d)); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmp, f.dst)), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(tmp, f.dst), f.data, 0o644); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(tmp, out.Dir); err != nil && !os.IsExist(err) {
		if st, serr := os.Stat(out.Dir); serr != nil || !st.IsDir() { // not built concurrently
			return nil, err
		}
	}
	return out, nil
}

func cacheRoot0(root string) string {
	_ = os.MkdirAll(root, 0o755)
	return root
}

// readTeamBricks lists the agents, skills and annexes of the team catalogue.
func readTeamBricks(teamDir string) ([]teamBrick, error) {
	var out []teamBrick
	agents, err := listFiles(filepath.Join(teamDir, "agents"), ".md")
	if err != nil {
		return nil, err
	}
	for _, rel := range agents {
		src := filepath.Join(teamDir, "agents", rel)
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		out = append(out, teamBrick{kind: "agent", id: strings.TrimSuffix(filepath.Base(rel), ".md"), src: src, data: data, extends: frontmatterExtends(data)})
	}
	skills, err := listFiles(filepath.Join(teamDir, "skills"), "")
	if err != nil {
		return nil, err
	}
	for _, rel := range skills {
		src := filepath.Join(teamDir, "skills", rel)
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(rel, ".md") && !strings.HasPrefix(rel, "templates"+string(filepath.Separator)) {
			out = append(out, teamBrick{kind: "skill", id: filepath.ToSlash(strings.TrimSuffix(rel, ".md")), src: src, data: data, extends: frontmatterExtends(data)})
		} else {
			out = append(out, teamBrick{kind: "annex", id: rel, src: src, data: data})
		}
	}
	return out, nil
}

// listFiles returns the files under dir (relative paths, sorted), with ext
// when not empty; .gitkeep and hidden files are skipped.
func listFiles(dir, ext string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") || (ext != "" && filepath.Ext(p) != ext) {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// frontmatterExtends reads `extends:` from a markdown frontmatter.
func frontmatterExtends(data []byte) string {
	fm, _ := splitFrontmatter(data)
	if fm == nil {
		return ""
	}
	var v struct {
		Extends string `yaml:"extends"`
	}
	_ = yaml.Unmarshal(fm, &v)
	return strings.TrimSpace(v.Extends)
}

// stripExtends removes the top-level `extends:` line of the frontmatter.
func stripExtends(data []byte) []byte {
	fm, body := splitFrontmatter(data)
	if fm == nil {
		return data
	}
	var kept []string
	for _, l := range strings.Split(string(fm), "\n") {
		if !strings.HasPrefix(l, "extends:") {
			kept = append(kept, l)
		}
	}
	return []byte("---\n" + strings.Join(kept, "\n") + "---\n" + string(body))
}

// splitFrontmatter returns the YAML between the leading "---" lines (nil
// without frontmatter) and the rest.
func splitFrontmatter(data []byte) (fm, body []byte) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, data
	}
	rest := data[4:]
	i := bytes.Index(rest, []byte("\n---"))
	if i < 0 {
		return nil, data
	}
	fm = rest[:i+1]
	body = rest[i+4:]
	body = bytes.TrimPrefix(body, []byte("\n"))
	return fm, body
}

// teamBricksUsed returns the team bricks a workflow uses (its members and
// their skills, plus skills.extra).
func teamBricksUsed(sp *wf.Spec, agents wf.AgentCatalog, team []string) []string {
	if sp == nil || len(team) == 0 {
		return nil
	}
	used := map[string]bool{}
	for _, id := range sp.Members() {
		used["agent:"+id] = true
		if agents != nil {
			if a, ok := agents.Agent(id); ok {
				for _, s := range a.Skills {
					used["skill:"+s] = true
				}
			}
		}
	}
	for _, s := range skillExtra(sp) {
		used["skill:"+s] = true
	}
	var out []string
	for _, b := range team {
		if used[b] {
			out = append(out, b)
		}
	}
	return out
}
