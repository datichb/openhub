package wiki

import (
	"context"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Check restricted to a change (modernisation LD-H, LD-D): the reviewer
// (third axis, documentation) and doc-sync ask wiki_check for the files a
// change touched; oh answers, deterministically:
//
//   - the issues of the pages of the wiki the change updated (and of the
//     god nodes table when one did);
//   - drift_confirmed: a page states as CONFIRMÉ (a tag whose source is
//     the file, or a page whose confidence is confirmed and cites the file)
//     what a changed file now says, and the change does not update it;
//   - drift_god_node: a changed file touches a god node (its name is the
//     concept, `AuthService` and src/auth_service.go, or a page of the god
//     node cites it on a line that names the concept), and the change
//     updates none of the pages of the god node;
//   - the scope: the files, the commits of a range and the pages concerned
//     (every page that cites a changed file or holds a touched god node).
//
// The reviewer reports each drift as a 🟡 finding.

// CheckWith checks the wiki of the project at root, restricted to the
// change of o (Check when o restricts nothing).
func CheckWith(ctx context.Context, root string, o CheckOptions, modified Modified) (CheckReport, error) {
	w, err := Load(root)
	if err != nil {
		return CheckReport{}, err
	}
	if !o.Scoped() {
		return w.Check(ctx, modified), nil
	}
	sc, err := ResolveScope(ctx, root, o)
	if err != nil {
		return CheckReport{}, err
	}
	return w.CheckScoped(ctx, sc, modified), nil
}

// CheckScoped checks a loaded wiki, restricted to a change.
func (w *Wiki) CheckScoped(ctx context.Context, sc *CheckScope, modified Modified) CheckReport {
	full := w.Check(ctx, modified)
	rep := CheckReport{Pages: full.Pages, GodNodes: full.GodNodes, Scope: sc, Issues: []Issue{}}
	updated := map[string]bool{}
	for _, p := range sc.Updated {
		updated[p] = true
	}
	for _, is := range full.Issues {
		godTable := strings.HasPrefix(is.Kind, "god_node") && len(sc.Updated) > 0
		if updated[is.Page] || godTable {
			rep.Issues = append(rep.Issues, is)
		}
	}
	issues, concerned := w.drift(full.GodNodes, sc.Changed, updated)
	rep.Issues = append(rep.Issues, issues...)
	sc.Pages = concerned
	return rep
}

// citation is a project file a page cites.
type citation struct {
	file string
	line int
	tag  string // the tag of the line, "" for a link
	text string // the line, and the one above for a tag on its own line
}

// citations returns the project files a page cites (tag sources and links
// out of the wiki).
func citations(p Page) []citation {
	lines := strings.Split(p.Text, "\n")
	textOf := func(i int) string {
		t := lines[i]
		if i > 0 && strings.HasPrefix(strings.TrimSpace(t), "—") {
			t = lines[i-1] + "\n" + t
		}
		return t
	}
	var out []citation
	EachLine(p.Text, func(i int, line string) {
		if m := tagLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			for _, f := range sourceFiles(strings.TrimSpace(m[4])) {
				out = append(out, citation{file: f, line: i + 1, tag: m[1], text: textOf(i)})
			}
		}
	})
	for _, c := range outLinks(p) {
		out = append(out, citation{file: c.file, line: c.line, text: textOf(c.line - 1)})
	}
	return out
}

// drift returns the drift issues of a change and the pages it concerns.
func (w *Wiki) drift(gods []GodNode, changed []string, updated map[string]bool) (issues []Issue, pages []string) {
	code := map[string]bool{}
	for _, f := range changed {
		if !strings.HasPrefix(f, Dir+"/") {
			code[f] = true
		}
	}
	concerned := map[string]bool{}
	cites := map[string][]citation{} // page → its citations of changed files
	for _, p := range w.Pages {
		confirmed := strings.TrimSpace(p.Frontmatter.Confidence) == ConfidenceConfirmed
		seen := map[string]bool{}
		for _, c := range citations(p) {
			if !code[c.file] {
				continue
			}
			concerned[p.Path] = true
			cites[p.Path] = append(cites[p.Path], c)
			if updated[p.Path] || seen[c.file] || (c.tag != TagConfirmed && (c.tag != "" || !confirmed)) {
				continue
			}
			seen[c.file] = true
			issues = append(issues, Issue{Page: p.Path, Line: c.line, Kind: IssueDriftConfirmed, Severity: SeverityWarning,
				Message: i18n.Tf("wiki.check.drift_confirmed", c.file)})
		}
	}
	for _, g := range gods {
		var files []string
		page := ""
		for f := range code {
			if sameName(f, g.Concept) {
				files = append(files, f)
			}
		}
		for _, p := range g.Pages {
			for _, c := range cites[p] {
				if containsWord(strings.ToLower(citingText(c.text)), strings.ToLower(g.Concept)) {
					files = append(files, c.file)
					if page == "" {
						page = p
					}
				}
			}
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		files = slices.Compact(files)
		fresh := false
		for _, p := range g.Pages {
			concerned[p] = true
			fresh = fresh || updated[p]
		}
		if fresh {
			continue
		}
		if page == "" {
			page = g.Pages[0]
		}
		issues = append(issues, Issue{Page: page, Kind: IssueDriftGodNode, Severity: SeverityWarning,
			Message: i18n.Tf("wiki.check.drift_god_node", strings.Join(files, ", "), g.Concept, strings.Join(g.Pages, ", "))})
	}
	pages = make([]string, 0, len(concerned))
	for p := range concerned {
		pages = append(pages, p)
	}
	sort.Strings(pages)
	return issues, pages
}

// sameName reports whether a file is named after a concept
// (src/auth_service.go, auth.service.ts and AuthService.java are named
// after AuthService); a test file is not.
func sameName(file, concept string) bool {
	base := path.Base(file)
	stem := strings.TrimSuffix(base, path.Ext(base))
	c := alnum(concept)
	return len(c) >= 3 && alnum(stem) == c
}

func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
