package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/skillregistry"
)

// Skills are referenced by "<category>/<name>" (hub file skills/<ref>.md) or
// by a bare community skill name (~/.oh/skills/<name>/). Their identifier in
// a bundle is the last path component, which becomes the folder name
// skills/<id>/SKILL.md: it must be unique in the bundle and equal to the
// frontmatter `name:` (opencode identifies skills by name).
//
// Hub-only frontmatter fields (stripped from the delivered SKILL.md):
//
//	requires: [<ref>, …]   skills that must be present with this one
//	annexes:  [<path>, …]  files shipped next to the skill (see annexes.go)

// hubSkillKeys are frontmatter keys consumed by oh, never delivered to the tool.
var hubSkillKeys = []string{"requires", "annexes"}

type skillFront struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
	Annexes     []string `yaml:"annexes"`
	Bucket      string   `yaml:"bucket"` // legacy (B17), reported by CheckSkills
}

type skillDoc struct {
	Ref     string
	ID      string
	Source  string // file path, "" for a generated skill
	Content []byte // whole file
	Front   skillFront
}

func (d *skillDoc) body() []byte {
	_, b := bricks.SplitFrontmatter(d.Content)
	return b
}

// delivered returns the SKILL.md content handed to the tool.
func (d *skillDoc) delivered() []byte {
	return stripFrontmatterKeys(d.Content, hubSkillKeys...)
}

var errSkillNotFound = errors.New("skill not found")

// bundleFileMode makes bundle files read-only: a bundle is immutable and its
// skill directories are readable by the agents (external_directory rule).
const bundleFileMode = 0o444

func skillID(ref string) string { return filepath.Base(ref) }

// skillLoader reads skills from the hub (generated workflow skills first).
type skillLoader struct {
	hubDir    string
	generated map[string]string
	cache     map[string]*skillDoc
}

func newSkillLoader(hubDir string, generated map[string]string) *skillLoader {
	return &skillLoader{hubDir: hubDir, generated: generated, cache: map[string]*skillDoc{}}
}

func (l *skillLoader) load(ref string) (*skillDoc, error) {
	if d, ok := l.cache[ref]; ok {
		return d, nil
	}
	var content []byte
	src := ""
	if gen, ok := l.generated[ref]; ok {
		content = []byte(gen)
	} else {
		p, err := bricks.SkillSourcePath(l.hubDir, ref)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errSkillNotFound, ref)
		}
		if content, err = os.ReadFile(p); err != nil {
			return nil, err
		}
		src = p
	}
	d, err := parseSkill(ref, src, content)
	if err != nil {
		return nil, err
	}
	l.cache[ref] = d
	return d, nil
}

func parseSkill(ref, src string, content []byte) (*skillDoc, error) {
	d := &skillDoc{Ref: ref, ID: skillID(ref), Source: src, Content: content}
	if fm, _ := bricks.SplitFrontmatter(content); fm != nil {
		if err := yaml.Unmarshal(frontmatterYAML(fm), &d.Front); err != nil {
			return nil, fmt.Errorf("skill %s: invalid frontmatter: %w", ref, err)
		}
	}
	return d, nil
}

// skillDenied reports whether a skill is excluded (deny list of refs or ids).
type skillDenied func(d *skillDoc) bool

func denyList(entries []string) skillDenied {
	if len(entries) == 0 {
		return func(*skillDoc) bool { return false }
	}
	set := map[string]bool{}
	for _, e := range entries {
		set[e] = true
	}
	return func(d *skillDoc) bool { return set[d.Ref] || set[d.ID] }
}

// closure returns the roots and their transitive requirements, each
// requirement before the skill that requires it, roots in the given order.
// A missing or denied root is skipped (missing: warning); a missing or
// denied requirement and a requirement cycle are errors.
func (l *skillLoader) closure(roots []string, denied skillDenied) ([]*skillDoc, error) {
	var out []*skillDoc
	done := map[string]bool{}
	onStack := map[string]bool{}
	var visit func(ref string, stack []string) error
	visit = func(ref string, stack []string) error {
		if done[ref] {
			return nil
		}
		if onStack[ref] {
			return &cycleError{path: append(append([]string(nil), stack...), ref)}
		}
		d, err := l.load(ref)
		if err != nil {
			if !errors.Is(err, errSkillNotFound) {
				return err
			}
			if len(stack) == 0 {
				slog.Warn("bundle: skill not found, skipped", "ref", ref)
				done[ref] = true
				return nil
			}
			return fmt.Errorf("bundle: skill %s requires %s, which was not found", stack[len(stack)-1], ref)
		}
		if denied(d) {
			if len(stack) == 0 {
				done[ref] = true
				return nil
			}
			return fmt.Errorf("bundle: skill %s requires %s, which is denied by the workflow", stack[len(stack)-1], ref)
		}
		onStack[ref] = true
		for _, req := range d.Front.Requires {
			if err := visit(req, append(stack, ref)); err != nil {
				return err
			}
		}
		onStack[ref] = false
		done[ref] = true
		out = append(out, d)
		return nil
	}
	for _, r := range roots {
		if err := visit(r, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// cycleError is a requirement cycle; path ends with the repeated skill.
type cycleError struct{ path []string }

func (e *cycleError) Error() string {
	return "bundle: skill requirement cycle: " + strings.Join(e.cycle(), " → ")
}

// cycle returns the members of the cycle, first one repeated at the end.
func (e *cycleError) cycle() []string {
	last := e.path[len(e.path)-1]
	for i, r := range e.path {
		if r == last {
			return e.path[i:]
		}
	}
	return e.path
}

// skillIndex enforces one source per skill identifier in a bundle.
type skillIndex map[string]string // id → ref

func (ix skillIndex) add(d *skillDoc) error {
	if prev, ok := ix[d.ID]; ok && prev != d.Ref {
		return fmt.Errorf("bundle: duplicate skill id %q (%s and %s)", d.ID, prev, d.Ref)
	}
	ix[d.ID] = d.Ref
	return nil
}

// checkName requires the frontmatter name to match the identifier of a
// delivered skill (the tool identifies skills by name).
func checkName(d *skillDoc) error {
	if d.Front.Name != d.ID {
		return fmt.Errorf("bundle: skill %s: frontmatter name %q must equal its identifier %q", d.Ref, d.Front.Name, d.ID)
	}
	return nil
}

// writeSkills writes the on-demand skills to <dst>/<id>/SKILL.md.
func writeSkills(dst string, docs []*skillDoc) ([]sessionspec.SkillDef, error) {
	sorted := append([]*skillDoc(nil), docs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	out := make([]sessionspec.SkillDef, 0, len(sorted))
	for _, d := range sorted {
		dir := filepath.Join(dst, d.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), d.delivered(), bundleFileMode); err != nil {
			return nil, err
		}
		out = append(out, sessionspec.SkillDef{ID: d.ID, Description: d.Front.Description, Dir: dir})
	}
	return out, nil
}

// frontmatterYAML strips the "---" delimiters returned by SplitFrontmatter.
func frontmatterYAML(fm []byte) []byte {
	s := strings.TrimPrefix(string(fm), "---\n")
	s = strings.TrimSuffix(s, "---\n")
	return []byte(s)
}

// stripFrontmatterKeys removes top-level keys (and their block values) from
// the frontmatter of a markdown file.
func stripFrontmatterKeys(content []byte, keys ...string) []byte {
	fm, body := bricks.SplitFrontmatter(content)
	if fm == nil {
		return content
	}
	var out bytes.Buffer
	skipping := false
	for _, line := range strings.SplitAfter(string(fm), "\n") {
		if line == "" {
			continue
		}
		top := line[0] != ' ' && line[0] != '\t'
		if skipping && (!top || strings.HasPrefix(line, "- ")) {
			continue
		}
		skipping = false
		if top {
			hit := false
			for _, k := range keys {
				if strings.HasPrefix(line, k+":") {
					hit = true
					break
				}
			}
			if hit {
				skipping = true
				continue
			}
		}
		out.WriteString(line)
	}
	out.Write(body)
	return out.Bytes()
}

// ─── Catalogue check (oh skill check, workflow validation) ──────────────────

// SkillProblemKind identifies a catalogue problem.
type SkillProblemKind string

const (
	SkillDuplicateID        SkillProblemKind = "duplicate_id"        // error
	SkillShadowed           SkillProblemKind = "shadowed"            // warning: community skill hidden by a hub skill
	SkillMissingRequire     SkillProblemKind = "missing_require"     // error
	SkillRequireCycle       SkillProblemKind = "require_cycle"       // error
	SkillInvalidFrontmatter SkillProblemKind = "invalid_frontmatter" // error
	SkillNameMismatch       SkillProblemKind = "name_mismatch"       // error
	SkillNoDescription      SkillProblemKind = "no_description"      // warning
	SkillMissingAgentSkill  SkillProblemKind = "missing_agent_skill" // warning
	SkillLegacyBucket       SkillProblemKind = "legacy_bucket"       // warning (B17)
	SkillMissingAnnex       SkillProblemKind = "missing_annex"       // error
	SkillUndeclaredAnnex    SkillProblemKind = "undeclared_annex"    // warning: referenced in the text, not in annexes:
	SkillOrphanAnnex        SkillProblemKind = "orphan_annex"        // warning: file of skills/templates/ used by no skill
)

// SkillProblem is one finding of CheckSkills.
type SkillProblem struct {
	Kind  SkillProblemKind `json:"kind"`
	Error bool             `json:"error"` // false = warning
	Ref   string           `json:"ref"`
	// Detail is the other party: duplicate ref, missing requirement, cycle
	// path, agent id, parse error.
	Detail string `json:"detail,omitempty"`
}

func problem(kind SkillProblemKind, ref, detail string) SkillProblem {
	isErr := true
	switch kind {
	case SkillShadowed, SkillNoDescription, SkillMissingAgentSkill, SkillLegacyBucket, SkillUndeclaredAnnex, SkillOrphanAnnex:
		isErr = false
	}
	return SkillProblem{Kind: kind, Error: isErr, Ref: ref, Detail: detail}
}

// CheckSkills inspects the whole skill catalogue (hub skills, installed
// community skills) and the skill references of the hub agents.
func CheckSkills(hubDir string) ([]SkillProblem, error) {
	var problems []SkillProblem
	docs := map[string]*skillDoc{} // ref → doc
	byID := map[string][]string{}

	root := filepath.Join(hubDir, "skills")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "templates" && filepath.Dir(path) == root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		ref := filepath.ToSlash(strings.TrimSuffix(rel, ".md"))
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		doc, perr := parseSkill(ref, path, content)
		if perr != nil {
			problems = append(problems, problem(SkillInvalidFrontmatter, ref, perr.Error()))
			return nil
		}
		docs[ref] = doc
		byID[doc.ID] = append(byID[doc.ID], ref)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking skills: %w", err)
	}

	community, _ := skillregistry.NewRegistry().ListInstalled()
	for _, c := range community {
		name := c.Manifest.Name
		if refs, ok := byID[name]; ok {
			problems = append(problems, problem(SkillShadowed, name, refs[0]))
			continue
		}
		skillFile, err := skillregistry.NewRegistry().SkillMDPath(name)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		doc, perr := parseSkill(name, skillFile, content)
		if perr != nil {
			problems = append(problems, problem(SkillInvalidFrontmatter, name, perr.Error()))
			continue
		}
		docs[name] = doc
		byID[name] = append(byID[name], name)
	}

	for _, id := range sortedKeys(byID) {
		if refs := byID[id]; len(refs) > 1 {
			sort.Strings(refs)
			for _, other := range refs[1:] {
				problems = append(problems, problem(SkillDuplicateID, refs[0], other))
			}
		}
	}

	loader := newSkillLoader(hubDir, nil)
	for _, ref := range sortedKeys(docs) {
		d := docs[ref]
		loader.cache[ref] = d
		if d.Front.Name != d.ID {
			problems = append(problems, problem(SkillNameMismatch, ref, d.Front.Name))
		}
		if strings.TrimSpace(d.Front.Description) == "" {
			problems = append(problems, problem(SkillNoDescription, ref, ""))
		}
		if d.Front.Bucket != "" {
			problems = append(problems, problem(SkillLegacyBucket, ref, d.Front.Bucket))
		}
	}
	for _, ref := range sortedKeys(docs) {
		for _, req := range docs[ref].Front.Requires {
			if _, err := loader.load(req); err != nil {
				problems = append(problems, problem(SkillMissingRequire, ref, req))
			}
		}
		var cyc *cycleError
		if _, err := loader.closure([]string{ref}, denyList(nil)); errors.As(err, &cyc) {
			members := cyc.cycle()
			lowest := members[0]
			for _, m := range members {
				if m < lowest {
					lowest = m
				}
			}
			if lowest == ref { // report each cycle once
				problems = append(problems, problem(SkillRequireCycle, ref, strings.Join(members, " → ")))
			}
		}
	}

	problems = append(problems, checkAnnexes(root, docs)...)

	agents, err := bricks.FindAgentFiles(hubDir)
	if err == nil {
		for _, id := range sortedKeys(agents) {
			fm, err := bricks.ParseAgentFrontmatter(agents[id])
			if err != nil {
				continue
			}
			for _, ref := range append(append([]string(nil), fm.Skills...), fm.NativeSkills...) {
				if _, err := loader.load(ref); errors.Is(err, errSkillNotFound) {
					problems = append(problems, problem(SkillMissingAgentSkill, ref, id))
				}
			}
		}
	}
	return problems, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkAnnexes reports missing, undeclared and unused annexes of hub skills.
func checkAnnexes(skillsRoot string, docs map[string]*skillDoc) []SkillProblem {
	var problems []SkillProblem
	used := map[string]bool{}
	for _, ref := range sortedKeys(docs) {
		d := docs[ref]
		if !isUnder(d.Source, skillsRoot) {
			continue // community skill: the whole package is shipped
		}
		declared := map[string]bool{}
		for _, rel := range d.Front.Annexes {
			clean := path.Clean(filepath.ToSlash(rel))
			declared[clean] = true
			used[clean] = true
			st, err := os.Stat(filepath.Join(skillsRoot, filepath.FromSlash(clean)))
			if err != nil || st.IsDir() || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
				problems = append(problems, problem(SkillMissingAnnex, ref, rel))
			}
		}
		seen := map[string]bool{}
		for _, rel := range annexRef.FindAllString(string(d.body()), -1) {
			if !declared[rel] && !seen[rel] {
				seen[rel] = true
				problems = append(problems, problem(SkillUndeclaredAnnex, ref, rel))
			}
		}
	}
	entries, _ := os.ReadDir(filepath.Join(skillsRoot, "templates"))
	for _, e := range entries {
		if rel := "templates/" + e.Name(); !e.IsDir() && !used[rel] {
			problems = append(problems, problem(SkillOrphanAnnex, rel, ""))
		}
	}
	return problems
}
