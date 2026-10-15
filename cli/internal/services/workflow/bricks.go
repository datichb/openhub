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
	// Integrity of the sealed catalogue (catalog.lock, ADR-054).
	DiagBrickUnlocked     = "brick_unlocked"
	DiagBrickHashMismatch = "brick_hash_mismatch"
	DiagBrickLockOrphan   = "brick_lock_orphan"
	DiagBrickLockInvalid  = "brick_lock_invalid"
	DiagBrickUnsafePath   = "brick_unsafe_path"
)

// brickDiagCodes are the codes of the refused team bricks.
var brickDiagCodes = []string{
	DiagBrickCollision, DiagBrickBadExtends, DiagBrickDuplicate, DiagBrickAnnexConflict,
	DiagBrickUnlocked, DiagBrickHashMismatch, DiagBrickLockOrphan, DiagBrickLockInvalid, DiagBrickUnsafePath,
}

// Bricks is the brick catalogue of a context.
type Bricks struct {
	// Dir is the content directory (hub alone, or hub merged with the team
	// catalogue).
	Dir string `json:"dir"`
	// Team lists the team bricks in use ("agent:<id>", "skill:<ref>").
	Team []string `json:"team,omitempty"`
	// Diagnostics are the refused team bricks (warnings).
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	// Shadows maps the team bricks that replace a hub brick (extends) to
	// that brick ("skill:shared/x" → "hub:shared/x").
	Shadows map[string]string `json:"shadows,omitempty"`
	// Sources maps the team bricks in use to their file in the team-state.
	Sources map[string]string `json:"sources,omitempty"`
	// Root is the team-state of the catalogue ("" = hub only).
	Root string `json:"-"`
	// Sealed: catalog.lock exists, only sealed bricks are loaded.
	Sealed bool `json:"sealed,omitempty"`
	// Unsealed lists the team bricks loaded without a lock entry (catalogue
	// committed by hand, never sealed: catalog.lock absent).
	Unsealed []string `json:"unsealed,omitempty"`
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
	b, err := mergeBricks(s.HubDir, team, s.bricksCacheDir(), nil)
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
	rel     string // path relative to the team-state root (catalog/…)
	extends string // frontmatter extends ("" = none)
	data    []byte
}

// mergeBricks builds (or reuses) the hub + team content directory. teamDir
// is the catalog/ folder of a team-state; its catalog.lock (next to it)
// decides which bricks are loaded. candidate, when not nil, is a brick
// being validated (draft): it replaces the published brick of the same key
// and is not checked against the lock.
func mergeBricks(hubDir, teamDir, cacheRoot string, candidate *teamBrick) (*Bricks, error) {
	bricks, unsafe, err := readTeamBricks(teamDir)
	if err != nil {
		return nil, err
	}
	out := &Bricks{Root: filepath.Dir(teamDir)}
	for _, u := range unsafe {
		out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickUnsafePath, u, u))
	}
	bricks, out.Diagnostics = checkBricksLock(filepath.Dir(teamDir), bricks, candidate, out, out.Diagnostics)
	if candidate != nil {
		kept := make([]teamBrick, 0, len(bricks)+1)
		for _, b := range bricks {
			if b.kind != candidate.kind || b.id != candidate.id {
				kept = append(kept, b)
			}
		}
		kept = append(kept, *candidate)
		bricks = kept
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
				out.shadow(key, "hub:"+b.id)
			} else if b.extends != "" {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickBadExtends, b.src, key, b.extends))
				continue
			}
			files = append(files, placed{dst, stripExtends(b.data)})
			out.Team = append(out.Team, key)
			out.source(key, b.src)
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
				out.shadow(key, "hub:"+hubRef)
			} else if b.extends != "" {
				out.Diagnostics = append(out.Diagnostics, brickDiag(DiagBrickBadExtends, b.src, key, b.extends))
				continue
			}
			files = append(files, placed{dst, stripExtends(b.data)})
			out.Team = append(out.Team, key)
			out.source(key, b.src)
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

// readTeamBricks lists the agents, skills and annexes of the team catalogue
// (drafts and history aside). Links are refused: nothing outside catalog/
// reaches a bundle (ADR-051, ADR-054); unsafe lists them.
func readTeamBricks(teamDir string) (out []teamBrick, unsafe []string, err error) {
	root := filepath.Dir(teamDir)
	read := func(dir, rel string) ([]byte, bool, error) {
		src := filepath.Join(teamDir, dir, rel)
		if st, err := os.Lstat(src); err != nil {
			return nil, false, err
		} else if !st.Mode().IsRegular() {
			unsafe = append(unsafe, src)
			return nil, false, nil
		}
		data, err := os.ReadFile(src)
		return data, err == nil, err
	}
	agents, err := listFiles(filepath.Join(teamDir, "agents"), ".md")
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range agents {
		data, ok, err := read("agents", rel)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		src := filepath.Join(teamDir, "agents", rel)
		r, _ := filepath.Rel(root, src)
		out = append(out, teamBrick{kind: "agent", id: strings.TrimSuffix(filepath.Base(rel), ".md"), src: src, rel: r, data: data, extends: frontmatterExtends(data)})
	}
	skills, err := listFiles(filepath.Join(teamDir, "skills"), "")
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range skills {
		data, ok, err := read("skills", rel)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		src := filepath.Join(teamDir, "skills", rel)
		r, _ := filepath.Rel(root, src)
		if strings.HasSuffix(rel, ".md") && !strings.HasPrefix(rel, "templates"+string(filepath.Separator)) {
			out = append(out, teamBrick{kind: "skill", id: filepath.ToSlash(strings.TrimSuffix(rel, ".md")), src: src, rel: r, data: data, extends: frontmatterExtends(data)})
		} else {
			out = append(out, teamBrick{kind: "annex", id: rel, src: src, rel: r, data: data})
		}
	}
	return out, unsafe, nil
}

// checkBricksLock keeps the bricks catalog.lock allows. Without the lock
// (catalogue never sealed), every brick is kept and listed in Unsealed.
// With it, a brick without entry, at another path or whose content changed
// is refused; an entry without file is reported. Annexes are not sealed.
func checkBricksLock(root string, bricks []teamBrick, candidate *teamBrick, out *Bricks, diags wf.Diagnostics) ([]teamBrick, wf.Diagnostics) {
	lock, sealed, err := teamstate.ReadBricksLock(root)
	lockFile := filepath.Join(root, teamstate.BricksLockFile)
	if err != nil {
		var kept []teamBrick
		for _, b := range bricks {
			if b.kind == "annex" {
				kept = append(kept, b)
			}
		}
		return kept, append(diags, brickDiag(DiagBrickLockInvalid, lockFile, err))
	}
	out.Sealed = sealed
	var kept []teamBrick
	seen := map[string]bool{}
	for _, b := range bricks {
		if b.kind == "annex" {
			kept = append(kept, b)
			continue
		}
		key := teamstate.BrickKey(b.kind, b.id)
		if teamstate.ValidBrickID(b.kind, b.id) != nil {
			diags = append(diags, brickDiag(DiagBrickUnsafePath, b.src, b.src))
			continue
		}
		if candidate != nil && candidate.kind == b.kind && candidate.id == b.id {
			seen[key] = true
			continue // replaced by the candidate
		}
		if !sealed {
			out.Unsealed = append(out.Unsealed, key)
			kept = append(kept, b)
			continue
		}
		e, ok := lock.Get(b.kind, b.id)
		if ok {
			seen[key] = true
		}
		switch {
		case !ok || e.CheckPath(b.kind, b.id) != nil || filepath.ToSlash(e.Path) != filepath.ToSlash(b.rel):
			diags = append(diags, brickDiag(DiagBrickUnlocked, b.src, key))
			continue
		case teamstate.HashContent(b.data) != e.Hash:
			diags = append(diags, brickDiag(DiagBrickHashMismatch, b.src, key, e.Version))
			continue
		}
		kept = append(kept, b)
	}
	for _, key := range lock.Keys() {
		if !seen[key] {
			diags = append(diags, brickDiag(DiagBrickLockOrphan, lockFile, key))
		}
	}
	return kept, diags
}

func (b *Bricks) shadow(key, hub string) {
	if b.Shadows == nil {
		b.Shadows = map[string]string{}
	}
	b.Shadows[key] = hub
}

func (b *Bricks) source(key, src string) {
	if b.Sources == nil {
		b.Sources = map[string]string{}
	}
	b.Sources[key] = src
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
