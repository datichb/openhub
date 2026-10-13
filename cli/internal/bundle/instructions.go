package bundle

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Project instructions by level (modernisation L7): the instruction files
// of the project reach each agent at the level of its frontmatter
// (project_instructions:): full, their whole text (default); index, their
// titles and the wiki index with its god nodes; none, nothing (the
// coordinators: their callees get them).

// instructionsHeading opens the project instructions in a composed prompt
// (report.go reads it as the end of the inlined skills).
const instructionsHeading = "\n\n---\n\n# Project instructions\n"

// wikiIndexFile is the index of the living wiki of the project
// (doc-wiki-protocol), read directly until the wiki service of LD-J.
const wikiIndexFile = "docs/wiki/index.md"

// indexMaxSections and indexMaxGodNodes cap the index of a file.
const (
	indexMaxSections = 12
	indexMaxGodNodes = 15
)

type instructionFile struct {
	Path     string // relative to the project
	Title    string // first level-1 heading
	Sections []string
	Body     string // trimmed text
}

type wikiIndex struct {
	Title    string
	Sections []string
	GodNodes []string // table lines of the god nodes section, header included
}

type projectInstructions struct {
	Files []instructionFile
	Wiki  *wikiIndex // nil without docs/wiki/index.md
}

func (p projectInstructions) empty() bool { return len(p.Files) == 0 && p.Wiki == nil }

func newInstructionFile(rel, text string) instructionFile {
	f := instructionFile{Path: rel, Body: strings.TrimSpace(text)}
	f.Title, f.Sections = headings(text)
	return f
}

// headings returns the first level-1 heading of a markdown text and its
// level-2 headings, outside the frontmatter and the code fences.
func headings(text string) (title string, sections []string) {
	eachLine(text, func(line string) {
		switch {
		case strings.HasPrefix(line, "# ") && title == "":
			title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			sections = append(sections, strings.TrimSpace(line[3:]))
		}
	})
	return title, sections
}

// eachLine calls fn on the lines of a markdown text, frontmatter and code
// fences aside.
func eachLine(text string, fn func(string)) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				lines = lines[i+1:]
				break
			}
		}
	}
	fence := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
			continue
		}
		if !fence {
			fn(line)
		}
	}
}

// parseWikiIndex reads the title, the sections and the god nodes table of
// the wiki index (section whose heading names the god nodes).
func parseWikiIndex(text string) *wikiIndex {
	w := &wikiIndex{}
	godNodes := false
	eachLine(text, func(line string) {
		switch {
		case strings.HasPrefix(line, "# ") && w.Title == "":
			w.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			s := strings.TrimSpace(line[3:])
			w.Sections = append(w.Sections, s)
			godNodes = strings.Contains(strings.ToLower(s), "god node")
		case godNodes && strings.HasPrefix(strings.TrimSpace(line), "|"):
			w.GodNodes = append(w.GodNodes, strings.TrimSpace(line))
		}
	})
	// A header and its separator without a row: no god node.
	if len(w.GodNodes) <= 2 {
		w.GodNodes = nil
	} else if len(w.GodNodes) > 2+indexMaxGodNodes {
		w.GodNodes = w.GodNodes[:2+indexMaxGodNodes]
	}
	return w
}

// render returns the project instructions of an agent at its level, perms
// being its rules (the index says which files it may read).
func (p projectInstructions) render(level string, perms []sessionspec.PermissionRule) string {
	if p.empty() {
		return ""
	}
	switch level {
	case bricks.InstructionsNone:
		return ""
	case bricks.InstructionsIndex:
		return p.renderIndex(perms)
	}
	if len(p.Files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(instructionsHeading)
	for _, f := range p.Files {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", f.Path, f.Body)
	}
	return b.String()
}

func (p projectInstructions) renderIndex(perms []sessionspec.PermissionRule) string {
	var b strings.Builder
	b.WriteString(instructionsHeading)
	b.WriteString("\n> Index généré par oh (`project_instructions: index`) : titres seulement. Lis un fichier avec l'outil `read` quand ta tâche en dépend.\n")
	if len(p.Files) > 0 {
		b.WriteString("\n")
	}
	for _, f := range p.Files {
		fmt.Fprintf(&b, "- `%s`", f.Path)
		if f.Title != "" {
			fmt.Fprintf(&b, " : %s", f.Title)
		}
		fmt.Fprintf(&b, " (≈ %d tokens)", EstimateTokens(f.Body))
		if len(f.Sections) > 0 {
			fmt.Fprintf(&b, " ; sections : %s", joinSections(f.Sections))
		}
		b.WriteString(readNote(perms, f.Path) + "\n")
	}
	if w := p.Wiki; w != nil {
		b.WriteString("\n## Wiki du projet\n\n")
		fmt.Fprintf(&b, "`%s`", wikiIndexFile)
		if w.Title != "" {
			fmt.Fprintf(&b, " : %s", w.Title)
		}
		if len(w.Sections) > 0 {
			fmt.Fprintf(&b, " ; sections : %s", joinSections(w.Sections))
		}
		b.WriteString(readNote(perms, wikiIndexFile) + "\n")
		if len(w.GodNodes) > 0 {
			b.WriteString("\nGod nodes (concepts les plus connectés, liens relatifs à `docs/wiki/`) :\n\n")
			b.WriteString(strings.Join(w.GodNodes, "\n") + "\n")
		}
	}
	return b.String()
}

func joinSections(ss []string) string {
	if len(ss) > indexMaxSections {
		return strings.Join(ss[:indexMaxSections], " · ") + " · …"
	}
	return strings.Join(ss, " · ")
}

// readNote tells the agent when its rules (guards included) do not let it
// read file; without a matching rule, the tool default applies (allow, no
// note).
func readNote(perms []sessionspec.PermissionRule, file string) string {
	matched := slices.ContainsFunc(perms, func(r sessionspec.PermissionRule) bool {
		return toolMatch(sessionspec.ActionRead, r.Action) && toolMatch(file, r.Resource)
	})
	if !matched {
		return ""
	}
	switch ToolEffect(perms, sessionspec.ActionRead, file) {
	case sessionspec.EffectDeny:
		return " — lecture non autorisée pour toi : si ta tâche en dépend, signale-le"
	case sessionspec.EffectAsk:
		return " — lecture soumise à confirmation"
	}
	return ""
}

// paths returns the files of the index (instruction files, wiki index).
func (p projectInstructions) paths() []string {
	out := make([]string, 0, len(p.Files)+1)
	for _, f := range p.Files {
		out = append(out, f.Path)
	}
	if p.Wiki != nil {
		out = append(out, wikiIndexFile)
	}
	return out
}

// instructionReadRules returns, for an agent at the level index, the read
// of each file of its index on its exact path, so that "read it on demand"
// holds: a read only, never a pattern, a path outside the project or a
// path that a rule of the agent other than "*" refuses or asks for (own:
// its own rules). They go after its own rules and before the guards,
// which always win.
func instructionReadRules(level string, own []sessionspec.PermissionRule, p projectInstructions) []sessionspec.PermissionRule {
	if level != bricks.InstructionsIndex {
		return nil
	}
	var out []sessionspec.PermissionRule
	for _, file := range p.paths() {
		rel := filepath.ToSlash(file)
		if rel != path.Clean(rel) || path.IsAbs(rel) || filepath.IsAbs(file) || rel == ".." || strings.HasPrefix(rel, "../") ||
			strings.ContainsAny(rel, "*?") {
			continue
		}
		if e, ok := explicitRead(own, rel); ok && e != sessionspec.EffectAllow {
			continue
		}
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionRead, Resource: rel, Effect: sessionspec.EffectAllow})
	}
	return out
}

// explicitRead returns the effect of the last rule of own on reading file
// whose resource is not "*" (the default of the agent is not explicit).
func explicitRead(own []sessionspec.PermissionRule, file string) (sessionspec.Effect, bool) {
	var eff sessionspec.Effect
	ok := false
	for _, r := range own {
		if r.Resource != "*" && toolMatch(sessionspec.ActionRead, r.Action) && toolMatch(file, r.Resource) {
			eff, ok = r.Effect, true
		}
	}
	return eff, ok
}

// instructionsContract is the session contract line of an agent at the
// level none when the project has instructions ("" otherwise).
func instructionsContract(level string, p projectInstructions) string {
	if level != bricks.InstructionsNone || p.empty() {
		return ""
	}
	return "- **Instructions du projet** : absentes de ton prompt (`project_instructions: none`) ; les agents que tu lances les reçoivent : ne les recopie pas dans leurs prompts.\n"
}
