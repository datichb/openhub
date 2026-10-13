package wiki

import (
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/services/report"
)

// God nodes computed by oh (doc-wiki-protocol, wiki-navigation): a concept
// cited by at least two distinct pages of the wiki (the index aside).
//
// The concepts are those the index declares in its god nodes table, and
// those the pages put forward: a bold term (one to five words, not a
// label ending with a colon) or an identifier in code (`AuthService`,
// `useCart`: letters, digits and underscores, with an upper and a lower
// case letter). A page cites a concept when it names it as a whole word
// (case insensitive), or, for a declared one, when the table links to it.
//
// Criticality: Critique at four pages or more, or when the index names the
// concept in its critical points, or when the index declares it Critique;
// Haute at three pages (or declared Haute); Normale at two. A declared row
// whose pages and criticality oh finds again is kept as written (its
// anchors included).

// Criticalities of a god node.
const (
	CriticalityCritical = "Critique"
	CriticalityHigh     = "Haute"
	CriticalityNormal   = "Normale"
)

// MinGodNodePages is the number of distinct pages that makes a god node.
const MinGodNodePages = 2

// GodNode is a concept cited by several pages.
type GodNode struct {
	Concept     string   `json:"concept"`
	Pages       []string `json:"pages"` // relative to docs/wiki/, sorted
	Criticality string   `json:"criticality"`
	// Declared: the index declares it in its god nodes table.
	Declared bool   `json:"declared,omitempty"`
	row      string // the declared row, kept when oh finds the same data
	links    map[string]string
}

// Index is the index of the wiki with the god nodes computed by oh.
type Index struct {
	// Present: docs/wiki/index.md exists.
	Present  bool
	Title    string
	Sections []string
	GodNodes []GodNode
	// Declared are the god nodes of the table of index.md, as written.
	Declared []GodNode
	// HasTable: index.md has a god nodes section.
	HasTable bool
	header   []string
}

// GodNodeTable returns the lines of the god nodes table (header and
// separator first) with at most limit rows (0: all); nil without god node.
func (ix Index) GodNodeTable(limit int) []string {
	if len(ix.GodNodes) == 0 {
		return nil
	}
	out := append([]string(nil), ix.header...)
	if len(out) != 2 {
		out = []string{"| Concept | Pages liées | Criticité |", "|---------|-------------|-----------|"}
	}
	for i, g := range ix.GodNodes {
		if limit > 0 && i == limit {
			break
		}
		out = append(out, g.Row())
	}
	return out
}

// Row renders the god node as a table row.
func (g GodNode) Row() string {
	if g.row != "" {
		return g.row
	}
	links := make([]string, 0, len(g.Pages))
	for _, p := range g.Pages {
		if l, ok := g.links[p]; ok {
			links = append(links, l)
			continue
		}
		links = append(links, "["+p+"]("+p+")")
	}
	return "| " + escapeCell(g.Concept) + " | " + strings.Join(links, ", ") + " | " + g.Criticality + " |"
}

func escapeCell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// isGodNodeHeading reports whether a section heading is the god nodes one.
func isGodNodeHeading(h string) bool { return strings.Contains(strings.ToLower(h), "god node") }

// isCriticalHeading reports whether a section heading is the critical points one.
func isCriticalHeading(h string) bool {
	return strings.Contains(strings.ToLower(h), "points critiques")
}

var (
	linkRe  = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)
	boldRe  = regexp.MustCompile(`\*\*([^*\n]{2,80})\*\*`)
	codeRe  = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]{2,59})`")
	fmtRe   = regexp.MustCompile("[`*_]")
	sepCell = regexp.MustCompile(`^:?-{3,}:?$`)
)

// cells splits a table line into its cells (escaped pipes kept).
func cells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var out []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case line[i] == '|':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	return append(out, strings.TrimSpace(cur.String()))
}

// plain returns the text of a Markdown cell: links become their text,
// emphasis and code marks go.
func plain(s string) string {
	s = linkRe.ReplaceAllString(s, "$1")
	s = fmtRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, `\|`, "|")
	return strings.Join(strings.Fields(s), " ")
}

func isSeparator(line string) bool {
	cs := cells(line)
	for _, c := range cs {
		if !sepCell.MatchString(strings.ReplaceAll(c, " ", "")) {
			return false
		}
	}
	return len(cs) > 0
}

// linkTarget returns the page a link of the wiki points to, relative to
// docs/wiki/ (from a page at dir), without its anchor ("" outside the wiki).
func linkTarget(dir, target string) string {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
		return ""
	}
	target, _, _ = strings.Cut(target, "#")
	target, _, _ = strings.Cut(target, "?")
	if target == "" || path.IsAbs(target) {
		return ""
	}
	p := path.Clean(path.Join(dir, target))
	if p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

// parseIndex reads the title, sections, god nodes table and critical
// points of the index page.
func parseIndex(p *Page) (ix Index, critical string) {
	ix.Present = true
	ix.Title, ix.Sections = p.Title, p.Sections
	inTable, inCritical := false, false
	var crit strings.Builder
	EachLine(p.Text, func(_ int, line string) {
		if strings.HasPrefix(line, "## ") {
			h := strings.TrimSpace(line[3:])
			inTable, inCritical = isGodNodeHeading(h), isCriticalHeading(h)
			ix.HasTable = ix.HasTable || inTable
			return
		}
		if inCritical {
			crit.WriteString(line + "\n")
		}
		t := strings.TrimSpace(line)
		if !inTable || !strings.HasPrefix(t, "|") {
			return
		}
		if len(ix.header) < 2 {
			if len(ix.header) == 0 || isSeparator(t) {
				ix.header = append(ix.header, t)
				return
			}
		}
		cs := cells(t)
		if len(cs) == 0 || plain(cs[0]) == "" {
			return
		}
		g := GodNode{Concept: plain(cs[0]), Declared: true, row: t, links: map[string]string{}}
		if len(cs) > 1 {
			for _, m := range linkRe.FindAllStringSubmatch(cs[1], -1) {
				if tgt := linkTarget(path.Dir(p.Path), m[2]); tgt != "" {
					g.Pages = append(g.Pages, tgt)
					if _, ok := g.links[tgt]; !ok {
						g.links[tgt] = m[0]
					}
				}
			}
		}
		if len(cs) > 2 {
			g.Criticality = plain(cs[2])
		}
		ix.Declared = append(ix.Declared, g)
	})
	if len(ix.header) != 2 {
		ix.header = nil
	}
	return ix, crit.String()
}

// Index returns the index of the wiki and its god nodes computed by oh.
func (w *Wiki) Index() Index {
	idx, ok := w.Page(IndexPage)
	if !ok {
		return Index{}
	}
	ix, critical := parseIndex(idx)
	ix.GodNodes = w.godNodes(ix, critical)
	return ix
}

// godNodes computes the god nodes: the declared ones first (in their
// order), then the others by number of pages.
func (w *Wiki) godNodes(ix Index, critical string) []GodNode {
	texts := map[string]string{} // page → text without frontmatter and code fences, lower case
	exists := map[string]bool{}
	var pages []string
	for _, p := range w.Pages {
		exists[p.Path] = true
		if p.Path == IndexPage {
			continue
		}
		var b strings.Builder
		EachLine(p.Text, func(_ int, line string) { b.WriteString(line + "\n") })
		texts[p.Path] = strings.ToLower(b.String())
		pages = append(pages, p.Path)
	}
	criticalLower := strings.ToLower(critical)
	cites := func(concept string) []string {
		var out []string
		c := strings.ToLower(concept)
		for _, p := range pages {
			if containsWord(texts[p], c) {
				out = append(out, p)
			}
		}
		return out
	}
	var out []GodNode
	seen := map[string]bool{}
	for _, d := range ix.Declared {
		key := strings.ToLower(d.Concept)
		if seen[key] {
			continue
		}
		seen[key] = true
		set := map[string]bool{}
		for _, p := range d.Pages {
			if exists[p] && p != IndexPage {
				set[p] = true
			}
		}
		for _, p := range cites(d.Concept) {
			set[p] = true
		}
		if len(set) < MinGodNodePages {
			continue
		}
		g := GodNode{Concept: d.Concept, Declared: true, links: d.links}
		for p := range set {
			g.Pages = append(g.Pages, p)
		}
		sort.Strings(g.Pages)
		g.Criticality = criticality(len(g.Pages), containsWord(criticalLower, key), d.Criticality)
		if strings.EqualFold(strings.TrimSpace(d.Criticality), g.Criticality) && sameSet(d.Pages, g.Pages) {
			g.row = d.row
		}
		out = append(out, g)
	}
	var found []GodNode
	for _, c := range w.candidates() {
		key := strings.ToLower(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		ps := cites(c)
		if len(ps) < MinGodNodePages {
			continue
		}
		found = append(found, GodNode{Concept: c, Pages: ps, Criticality: criticality(len(ps), containsWord(criticalLower, key), "")})
	}
	sort.SliceStable(found, func(i, j int) bool {
		if len(found[i].Pages) != len(found[j].Pages) {
			return len(found[i].Pages) > len(found[j].Pages)
		}
		return strings.ToLower(found[i].Concept) < strings.ToLower(found[j].Concept)
	})
	return append(out, found...)
}

// criticality returns the criticality of a god node cited by n pages.
func criticality(n int, critical bool, declared string) string {
	declared = strings.ToLower(strings.TrimSpace(declared))
	switch {
	case n >= 4 || critical || declared == strings.ToLower(CriticalityCritical):
		return CriticalityCritical
	case n == 3 || declared == strings.ToLower(CriticalityHigh):
		return CriticalityHigh
	}
	return CriticalityNormal
}

func sameSet(declared, pages []string) bool {
	set := map[string]bool{}
	for _, p := range declared {
		set[p] = true
	}
	if len(set) != len(pages) {
		return false
	}
	for _, p := range pages {
		if !set[p] {
			return false
		}
	}
	return true
}

// candidates returns the concepts the pages put forward, in order of
// appearance.
func (w *Wiki) candidates() []string {
	var out []string
	seen := map[string]bool{}
	add := func(c string) {
		c = strings.Join(strings.Fields(c), " ")
		key := strings.ToLower(c)
		if c == "" || seen[key] || slices.Contains(Tags, strings.ToUpper(c)) {
			return
		}
		seen[key] = true
		out = append(out, c)
	}
	for _, p := range w.Pages {
		if p.Path == IndexPage {
			continue
		}
		EachLine(p.Text, func(_ int, line string) {
			for _, m := range boldRe.FindAllStringSubmatch(line, -1) {
				t := strings.TrimSpace(m[1])
				if n := len(strings.Fields(t)); n >= 1 && n <= 5 && !strings.ContainsAny(t, ":`[]()") && utf8.RuneCountInString(t) >= 3 {
					add(t)
				}
			}
			for _, m := range codeRe.FindAllStringSubmatch(line, -1) {
				if camel(m[1]) {
					add(m[1])
				}
			}
		})
	}
	return out
}

// camel reports whether an identifier mixes upper and lower case letters
// after its first character (AuthService, useCart; not JWT nor snake_case).
func camel(id string) bool {
	upper, lower := false, false
	for i, r := range id {
		switch {
		case unicode.IsUpper(r) && i > 0:
			upper = true
		case unicode.IsLower(r):
			lower = true
		}
	}
	return upper && lower
}

// containsWord reports whether word occurs in text as a whole word (both
// lower case).
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(text[i:], word)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(word)
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if !wordRune(before) && !wordRune(after) {
			return true
		}
		i = start + 1
		if i >= len(text) {
			return false
		}
	}
}

func wordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

// sectionOf returns a section of a page (report.Section: exact heading,
// else the first one that contains name).
func sectionOf(text, name string) (string, bool) { return report.Section(text, name) }
