package bundle

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
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
	return skillMentions(body, index, true)
}

// selfMentionedSkills returns the skills body invites its reader to load:
// the code spans, not the [SKILL:…] tags (written by an agent in the
// delegation prompt of another one, they invite the callee).
func selfMentionedSkills(body string, index map[string]string) []string {
	return skillMentions(body, index, false)
}

func skillMentions(body string, index map[string]string, tags bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range skillMentionRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if name == "" {
			if !tags {
				continue
			}
			name = m[2]
		}
		if ref, ok := index[name]; ok && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out
}

// headingRe matches a markdown heading: its level and its title.
var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

// titleWordRe splits a heading title into words (agent ids keep their dashes).
var titleWordRe = regexp.MustCompile(`[a-z0-9][a-z0-9-]*`)

// ignoredSection reports whether the mentions of a section do not invite
// agent to load a skill (L2): the « Relation avec … » and « Injection … »
// notes, and the sections addressed to other agents than this one: rules
// for the consumer or the producer of a handoff, « Pour le <agent> » (a
// title naming no agent, « agent appelant », is kept).
func ignoredSection(title, agent string, agents map[string]bool) bool {
	t := strings.ToLower(title)
	if strings.HasPrefix(t, "relation") || strings.HasPrefix(t, "injection") {
		return true
	}
	addressed := strings.HasPrefix(t, "pour ")
	for _, w := range []string{"consommateur", "consumer", "producteur", "producer"} {
		addressed = addressed || strings.Contains(t, w)
	}
	if !addressed {
		return false
	}
	named := false
	for _, w := range titleWordRe.FindAllString(t, -1) {
		if w == agent {
			return false
		}
		named = named || agents[w]
	}
	return named
}

// invitingText returns body without the sections that are ignored for agent
// (ignoredSection): a section runs to the next heading of the same or a
// higher level, or to the next inlined skill. Headings inside code fences
// are text.
func invitingText(body, agent string, agents map[string]bool) string {
	var b strings.Builder
	fence := false
	skip := 0 // level of the ignored section, 0 when none
	for _, ln := range strings.SplitAfter(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fence = !fence
		} else if !fence {
			if inlinedSkillRe.MatchString(ln) {
				skip = 0
			} else if m := headingRe.FindStringSubmatch(strings.TrimRight(ln, "\n")); m != nil {
				if skip > 0 && len(m[1]) <= skip {
					skip = 0
				}
				if skip == 0 && ignoredSection(m[2], agent, agents) {
					skip = len(m[1])
				}
			}
		}
		if skip == 0 {
			b.WriteString(ln)
		}
	}
	return b.String()
}

// absentSkillsNote tells the agent which skills it names are not in this
// session bundle (plugin not loaded, frontend skills in a project without
// frontend, A8): it does not try to load them ("" when none). It closes an
// agent prompt, or an on-demand skill for the skills that one names
// (absentFromSkills).
func absentSkillsNote(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	ids = slices.Clone(ids)
	sort.Strings(ids)
	ids = slices.Compact(ids)
	return "\n\n> Skills absentes de ce paquet de session (elles ne s'appliquent pas à ce projet ou à ce workflow) : `" + strings.Join(ids, "`, `") + "`. Ne les charge pas avec l'outil `skill`.\n"
}

// inlinedSkillHeading opens an inlined skill in an agent body.
func inlinedSkillHeading(id string) string {
	return "> Skill `" + id + "` : incluse ci-dessous dans ton prompt, ne la charge pas avec l'outil `skill`.\n\n"
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
