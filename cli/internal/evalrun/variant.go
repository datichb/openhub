package evalrun

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Variants of the hub (oh eval --variant, level 5): the same cases run with
// several versions of the hub content, to compare them (a skill kept only if
// the passes with it do better than the passes without it).

// Variant is a version of the hub the passes of a run use.
type Variant struct {
	Name string `json:"name"`
	// Source is the hub directory the variant starts from (agents/,
	// skills/, permissions/, workflows/); "" is the hub embedded in oh.
	Source string `json:"source,omitempty"`
	// RemoveSkills are the skills taken out of the source.
	RemoveSkills []string `json:"remove_skills,omitempty"`
	// Removed lists what PrepareVariant took out (files, frontmatter
	// mentions), for the report.
	Removed []string `json:"removed,omitempty"`
	// Hub is the prepared hub directory installed in the home of each pass
	// (PrepareVariant).
	Hub string `json:"-"`
}

// Describe tells where the hub of the variant comes from.
func (v Variant) Describe() string {
	src := "hub embarqué"
	if v.Source != "" {
		src = "hub " + v.Source
	}
	if len(v.RemoveSkills) > 0 {
		src += " sans " + strings.Join(v.RemoveSkills, ", ")
	}
	return src
}

// Plain tells a variant that is the embedded hub unchanged (comparable
// with a baseline).
func (v Variant) Plain() bool { return v.Source == "" && len(v.RemoveSkills) == 0 }

var variantName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// removeSkillPrefix starts an item of a variant that removes a skill.
const removeSkillPrefix = "-skill:"

// ParseVariant reads a variant: `name` (the embedded hub), or
// `name=item,item…` where an item is a hub directory (at most one) or
// `-skill:<id>` (a skill taken out, by name or category/name).
func ParseVariant(s string) (Variant, error) {
	name, spec, hasSpec := strings.Cut(strings.TrimSpace(s), "=")
	name = strings.TrimSpace(name)
	if !variantName.MatchString(name) {
		return Variant{}, fmt.Errorf("variant %q: the name (before =) is letters, digits, '.', '_' or '-'", s)
	}
	v := Variant{Name: name}
	if !hasSpec {
		return v, nil
	}
	if strings.TrimSpace(spec) == "" {
		return Variant{}, fmt.Errorf("variant %q: nothing after =", s)
	}
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
			return Variant{}, fmt.Errorf("variant %q: empty item", s)
		case strings.HasPrefix(item, removeSkillPrefix):
			id := strings.Trim(strings.TrimPrefix(item, removeSkillPrefix), "/ ")
			if id == "" || strings.Contains(id, "..") {
				return Variant{}, fmt.Errorf("variant %q: -skill: needs a skill id", s)
			}
			v.RemoveSkills = append(v.RemoveSkills, id)
		case strings.HasPrefix(item, "-") || strings.HasPrefix(item, "+"):
			return Variant{}, fmt.Errorf("variant %q: unknown item %q (-skill:<id> or a hub directory)", s, item)
		case v.Source != "":
			return Variant{}, fmt.Errorf("variant %q: two hub directories", s)
		default:
			v.Source = item
		}
	}
	return v, nil
}

// ParseVariants reads the variants of a run; names are unique. The first
// one is the reference of the comparison.
func ParseVariants(specs []string) ([]Variant, error) {
	var out []Variant
	seen := map[string]bool{}
	for _, s := range specs {
		v, err := ParseVariant(s)
		if err != nil {
			return nil, err
		}
		if seen[v.Name] {
			return nil, fmt.Errorf("variant %s given twice", v.Name)
		}
		seen[v.Name] = true
		out = append(out, v)
	}
	return out, nil
}

// hubParts are the directories of a hub a variant copies.
var hubParts = []string{"agents", "skills", "permissions", "workflows"}

// PrepareVariant writes the hub of a variant under workRoot/variants/<name>:
// its source (embedded: the hub extracted from oh), less the skills it
// removes. It sets v.Hub and v.Removed.
func PrepareVariant(v *Variant, embedded, workRoot string) error {
	src := v.Source
	if src == "" {
		src = embedded
	}
	if src == "" {
		return fmt.Errorf("variant %s: no hub", v.Name)
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	for _, p := range []string{"agents", "skills", "workflows"} {
		if st, err := os.Stat(filepath.Join(abs, p)); err != nil || !st.IsDir() {
			return fmt.Errorf("variant %s: %s is not a hub (no %s/)", v.Name, src, p)
		}
	}
	dest := filepath.Join(workRoot, "variants", v.Name)
	if err := RemoveAll(dest); err != nil {
		return err
	}
	for _, p := range hubParts {
		if _, err := os.Stat(filepath.Join(abs, p)); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := copyTree(filepath.Join(abs, p), filepath.Join(dest, p)); err != nil {
			return fmt.Errorf("variant %s: %w", v.Name, err)
		}
	}
	v.Removed = nil
	for _, id := range v.RemoveSkills {
		removed, err := RemoveSkill(dest, id)
		if err != nil {
			return fmt.Errorf("variant %s: %w", v.Name, err)
		}
		v.Removed = append(v.Removed, removed...)
	}
	v.Hub = dest
	return nil
}

// skillListKeys are the frontmatter keys of an agent that name skills.
var skillListKeys = map[string]bool{"skills": true, "native_skills": true}

// RemoveSkill takes a skill out of a hub: its file, and its mentions in the
// skill lists of the agents' frontmatter (skills, native_skills). id is the
// name of the skill or its category/name. It returns what changed, relative
// to the hub. The other references (requires: of a skill, skills: of a
// workflow) are left: the check of the variant (CheckHub) reports them.
func RemoveSkill(hub, id string) ([]string, error) {
	root := filepath.Join(hub, "skills")
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "templates" {
				return filepath.SkipDir // annexes, not skills
			}
			return nil
		}
		if ref, ok := strings.CutSuffix(rel, ".md"); ok && (ref == id || path.Base(ref) == id) {
			found = append(found, ref)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("skill %s: not in the hub", id)
	case 1:
	default:
		sort.Strings(found)
		return nil, fmt.Errorf("skill %s: ambiguous (%s), give its category", id, strings.Join(found, ", "))
	}
	ref := found[0]
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(ref)+".md")); err != nil {
		return nil, err
	}
	changed := []string{"skills/" + ref + ".md"}
	agents := filepath.Join(hub, "agents")
	err = filepath.WalkDir(agents, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out, keys := dropSkillMentions(string(data), ref)
		if len(keys) == 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(out), info.Mode().Perm()); err != nil {
			return err
		}
		rel, _ := filepath.Rel(hub, p)
		changed = append(changed, filepath.ToSlash(rel)+" ("+strings.Join(keys, ", ")+")")
		return nil
	})
	return changed, err
}

// dropSkillMentions removes ref (or its bare name) from the skill lists of
// a frontmatter, flow ([a, b]) or block (- a) style. It returns the text
// and the keys changed.
func dropSkillMentions(doc, ref string) (text string, keys []string) {
	lines := strings.SplitAfter(doc, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return doc, nil
	}
	matches := func(item string) bool {
		item = strings.Trim(strings.TrimSpace(item), `"'`)
		return item == ref || item == path.Base(ref)
	}
	var out []string
	out = append(out, lines[0])
	block := "" // key of the block list being read
	i := 1
	for ; i < len(lines); i++ {
		l := lines[i]
		trim := strings.TrimRight(l, "\r\n")
		if trim == "---" {
			break
		}
		if block != "" {
			if item, ok := strings.CutPrefix(strings.TrimSpace(trim), "- "); ok && strings.HasPrefix(trim, " ") {
				if matches(item) {
					keys = appendOnce(keys, block)
					continue
				}
				out = append(out, l)
				continue
			}
			block = ""
		}
		key, val, ok := strings.Cut(trim, ":")
		if ok && skillListKeys[key] {
			val = strings.TrimSpace(val)
			switch {
			case val == "":
				block = key
			case strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]"):
				var kept []string
				dropped := false
				for _, it := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(val, "["), "]"), ",") {
					if strings.TrimSpace(it) == "" {
						continue
					}
					if matches(it) {
						dropped = true
						continue
					}
					kept = append(kept, strings.TrimSpace(it))
				}
				if dropped {
					keys = appendOnce(keys, key)
					l = key + ": [" + strings.Join(kept, ", ") + "]" + l[len(trim):]
				}
			}
		}
		out = append(out, l)
	}
	out = append(out, lines[i:]...)
	if len(keys) == 0 {
		return doc, nil
	}
	return strings.Join(out, ""), keys
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
