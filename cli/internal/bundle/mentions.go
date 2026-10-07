package bundle

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

// Skills named in an agent body are made loadable (v5 finalisation, Q3-1).
//
// The hub content names skills in code spans (« voir skill `x` »), written
// when every hub skill was deployed. In a closed-world bundle such a skill is
// either inlined in the agent body (the model must not load it: it gets a
// heading naming it) or shipped on demand when the agent names it.

// skillMentionRe matches a code span that may hold a skill reference, or a
// skill tag of a delegation prompt ([SKILL:cat/name], QB3).
var skillMentionRe = regexp.MustCompile("`([a-z0-9][a-z0-9/_-]*)`|\\[SKILL:([a-z0-9][a-z0-9/_-]*)\\]")

// hubSkillIndex maps the reference forms of the hub and generated skills
// ("cat/name" and "name") to their reference. A name shared by two skills is
// left out (ambiguous).
func hubSkillIndex(hubDir string, generated map[string]string) map[string]string {
	index := map[string]string{}
	ambiguous := map[string]bool{}
	addRef := func(ref string) {
		index[ref] = ref
		name := skillID(ref)
		if prev, ok := index[name]; ok && prev != ref {
			ambiguous[name] = true
			return
		}
		index[name] = ref
	}
	root := filepath.Join(hubDir, "skills")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable hub only indexes fewer skills
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "templates" {
				return filepath.SkipDir // annexes, not skills
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") && strings.Contains(rel, "/") {
			addRef(strings.TrimSuffix(rel, ".md"))
		}
		return nil
	})
	for ref := range generated {
		addRef(ref)
	}
	for name := range ambiguous {
		if index[name] != name {
			delete(index, name)
		}
	}
	return index
}

// mentionedSkills returns the references of the skills named in body, in
// order of first mention.
func mentionedSkills(body string, index map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range skillMentionRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if ref, ok := index[name]; ok && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out
}

// inlinedSkillHeading opens an inlined skill in an agent body.
func inlinedSkillHeading(id string) string {
	return fmt.Sprintf("> Skill `%s` : incluse ci-dessous dans ton prompt, ne la charge pas avec l'outil `skill`.\n\n", id)
}

// deliverable reports whether a mentioned skill can be shipped on demand:
// neither it nor a requirement is denied, all are found and well named. A
// mention never makes the build fail.
func deliverable(l *skillLoader, ref string, denied skillDenied) bool {
	d, err := l.load(ref)
	if err != nil || denied(d) {
		return false
	}
	docs, err := l.closure([]string{ref}, denied)
	if err != nil || len(docs) == 0 {
		return false
	}
	for _, d := range docs {
		if checkName(d) != nil {
			return false
		}
	}
	return true
}
