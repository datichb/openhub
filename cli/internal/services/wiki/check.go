package wiki

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Check of the wiki (LD-E, wiki_check, oh wiki check): deterministic, run
// by oh on the files.
//
//   - frontmatter: present, readable, `updated` a date, `confidence` one of
//     confirmed | mixed | uncertain and consistent with the tags of the
//     page (the lowest one wins), `agents` set; no other key (`synced` in
//     index.md), a warning, an error in a spec (format.go);
//   - links: every relative link of a page leads to an existing file (its
//     anchor to a heading of the page it targets);
//   - tags: every enrichment line reads `— `TAG` · agent · YYYY-MM-DD[ ·
//     source]`, a CONFIRMÉ one with its source; a page without any tag is
//     reported;
//   - freshness: a page older (`updated`) than a file it cites (source of a
//     tag, link out of the wiki), the date of a file being its last commit
//     (else its modification time); a cited file that does not exist;
//   - god nodes: the table of the index against the god nodes oh computes.
//
// Errors break the format; warnings tell what to update.

// Severities of an issue.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Kinds of issue.
const (
	IssueNoIndex            = "no_index"
	IssueSkipped            = "skipped"
	IssueFrontmatter        = "frontmatter"
	IssueUpdated            = "updated"
	IssueConfidence         = "confidence"
	IssueConfidenceMismatch = "confidence_mismatch"
	IssueAgents             = "agents"
	IssueBrokenLink         = "broken_link"
	IssueBrokenAnchor       = "broken_anchor"
	IssueTag                = "tag"
	IssueTagSource          = "tag_source"
	IssueUntagged           = "untagged"
	IssueStale              = "stale"
	IssueCitedMissing       = "cited_missing"
	IssueGodNodeMissing     = "god_node_missing"
	IssueGodNodeStale       = "god_node_stale"
	IssueGodNodeChanged     = "god_node_changed"
	IssueGodNodeTable       = "god_node_table"
	// LD-F: a page out of the layout of the wiki (layout.go).
	IssueLayout = "layout"
	// LD-H, LD-D: a change touches what a page states (drift.go).
	IssueDriftConfirmed = "drift_confirmed"
	IssueDriftGodNode   = "drift_god_node"
)

// maxCited bounds the cited files whose date oh reads.
const maxCited = 150

// Issue is a finding of the check.
type Issue struct {
	Page     string `json:"page,omitempty"` // relative to docs/wiki/
	Line     int    `json:"line,omitempty"` // 1-based
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	// Fixable: oh wiki check --fix repairs it.
	Fixable bool `json:"fixable,omitempty"`
}

// CheckReport is the result of a check.
type CheckReport struct {
	Pages    int       `json:"pages"`
	GodNodes []GodNode `json:"god_nodes,omitempty"`
	Issues   []Issue   `json:"issues"`
	// Scope is the change the check is restricted to (CheckWith).
	Scope *CheckScope `json:"scope,omitempty"`
}

// Errors returns the number of errors.
func (r CheckReport) Errors() int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			n++
		}
	}
	return n
}

// Modified returns the date of a project file (relative path): its last
// commit, else its modification time; false when it does not exist.
type Modified func(ctx context.Context, root, file string) (time.Time, bool)

// GitModified is the default Modified.
func GitModified(ctx context.Context, root, file string) (time.Time, bool) {
	st, err := statConfined(root, file)
	if err != nil {
		return time.Time{}, false
	}
	if gitutil.LooksLikeRepository(root) {
		cmd := gitutil.Command(ctx, root, "log", "-1", "--format=%cI", "--", file)
		if out, err := cmd.Output(); err == nil {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(out))); err == nil {
				if dirty := gitutil.Command(ctx, root, "status", "--porcelain", "--", file); dirty != nil {
					if d, err := dirty.Output(); err == nil && strings.TrimSpace(string(d)) == "" {
						return t, true
					}
				}
			}
		}
	}
	return st.ModTime(), true
}

var (
	tagLineRe  = regexp.MustCompile("— *`(CONFIRMÉ|DÉDUIT|INCERTAIN)` *· *([^·]+?) *· *([0-9]{4}-[0-9]{2}-[0-9]{2})(?: *· *(.+))?$")
	anyTagRe   = regexp.MustCompile("`(CONFIRMÉ|DÉDUIT|INCERTAIN)`")
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	sourceLnRe = regexp.MustCompile(`:\d+(?:-\d+)?$`)
)

// Check checks the wiki of the project at root.
func Check(ctx context.Context, root string, modified Modified) (CheckReport, error) {
	w, err := Load(root)
	if err != nil {
		return CheckReport{}, err
	}
	return w.Check(ctx, modified), nil
}

// Check checks a loaded wiki.
func (w *Wiki) Check(ctx context.Context, modified Modified) CheckReport {
	if modified == nil {
		modified = GitModified
	}
	rep := CheckReport{Pages: len(w.Pages)}
	add := func(i Issue) { rep.Issues = append(rep.Issues, i) }
	for _, s := range w.Skipped {
		add(Issue{Page: s, Kind: IssueSkipped, Severity: SeverityWarning, Message: i18n.T("wiki.check.skipped")})
	}
	if _, ok := w.Page(IndexPage); !ok {
		add(Issue{Page: IndexPage, Kind: IssueNoIndex, Severity: SeverityError, Message: i18n.T("wiki.check.no_index")})
	}
	anchors := map[string]map[string]bool{}
	for _, p := range w.Pages {
		anchors[p.Path] = pageAnchors(p.Text)
	}
	cited := 0
	dates := map[string]time.Time{}
	for _, p := range w.Pages {
		if PageKind(p.Path) == KindOther {
			add(Issue{Page: p.Path, Kind: IssueLayout, Severity: SeverityWarning, Message: i18n.T("wiki.check.layout")})
		}
		w.checkFrontmatter(p, add)
		keys := SeverityWarning
		if PageKind(p.Path) == KindSpec {
			keys = SeverityError // the format verify.spec checks
		}
		checkFrontmatterKeys(p, keys, add)
		w.checkLinks(p, anchors, add)
		files := checkTags(p, add)
		files = append(files, outLinks(p)...)
		updated, ok := p.Frontmatter.UpdatedAt()
		seen := map[string]bool{}
		for _, f := range files {
			if seen[f.file] {
				continue
			}
			seen[f.file] = true
			if !insideProject(f.file) {
				continue
			}
			t, known := dates[f.file]
			if !known {
				if cited >= maxCited {
					continue
				}
				cited++
				var exists bool
				if t, exists = modified(ctx, w.Root, f.file); !exists {
					add(Issue{Page: p.Path, Line: f.line, Kind: IssueCitedMissing, Severity: SeverityWarning, Message: i18n.Tf("wiki.check.cited_missing", f.file)})
					dates[f.file] = time.Time{}
					continue
				}
				dates[f.file] = t
			}
			if t.IsZero() || !ok {
				continue
			}
			if day(t).After(updated) {
				add(Issue{Page: p.Path, Line: f.line, Kind: IssueStale, Severity: SeverityWarning,
					Message: i18n.Tf("wiki.check.stale", p.Frontmatter.Updated, f.file, t.Format(time.DateOnly))})
			}
		}
	}
	ix := w.Index()
	rep.GodNodes = ix.GodNodes
	if ix.Present {
		checkGodNodes(ix, add)
	}
	return rep
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (w *Wiki) checkFrontmatter(p Page, add func(Issue)) {
	fm := p.Frontmatter
	switch {
	case !fm.Present:
		add(Issue{Page: p.Path, Line: 1, Kind: IssueFrontmatter, Severity: SeverityError, Message: i18n.T("wiki.check.no_frontmatter")})
		return
	case fm.Err != nil:
		add(Issue{Page: p.Path, Line: 1, Kind: IssueFrontmatter, Severity: SeverityError, Message: i18n.Tf("wiki.check.bad_frontmatter", fm.Err.Error())})
		return
	}
	if _, ok := fm.UpdatedAt(); !ok {
		add(Issue{Page: p.Path, Line: 1, Kind: IssueUpdated, Severity: SeverityError, Message: i18n.Tf("wiki.check.updated", fm.Updated)})
	}
	want := pageConfidence(p.Text)
	switch c := strings.TrimSpace(fm.Confidence); {
	case c != ConfidenceConfirmed && c != ConfidenceMixed && c != ConfidenceUncertain:
		add(Issue{Page: p.Path, Line: 1, Kind: IssueConfidence, Severity: SeverityError, Fixable: want != "",
			Message: i18n.Tf("wiki.check.confidence", fm.Confidence)})
	case want != "" && c != want:
		add(Issue{Page: p.Path, Line: 1, Kind: IssueConfidenceMismatch, Severity: SeverityWarning, Fixable: true,
			Message: i18n.Tf("wiki.check.confidence_mismatch", c, want)})
	}
	if len(fm.Agents) == 0 {
		add(Issue{Page: p.Path, Line: 1, Kind: IssueAgents, Severity: SeverityWarning, Message: i18n.T("wiki.check.agents")})
	}
}

// pageConfidence returns the confidence the tags of a page give ("" without
// tag): uncertain with an INCERTAIN, mixed with a DÉDUIT, else confirmed.
func pageConfidence(text string) string {
	has := map[string]bool{}
	EachLine(text, func(_ int, line string) {
		for _, m := range anyTagRe.FindAllStringSubmatch(line, -1) {
			has[m[1]] = true
		}
	})
	switch {
	case has[TagUncertain]:
		return ConfidenceUncertain
	case has[TagDeduced]:
		return ConfidenceMixed
	case has[TagConfirmed]:
		return ConfidenceConfirmed
	}
	return ""
}

// slug returns the anchor of a heading (GitHub style).
func slug(h string) string {
	h = strings.ToLower(plain(h))
	var b strings.Builder
	for _, r := range h {
		switch {
		case r == ' ' || r == '-':
			b.WriteRune('-')
		case r == '_' || wordRune(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pageAnchors(text string) map[string]bool {
	out := map[string]bool{}
	EachLine(text, func(_ int, line string) {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			s := slug(m[2])
			out[s] = true
			// Some renderers drop the emoji and trailing dashes.
			out[strings.Trim(s, "-")] = true
		}
	})
	return out
}

func (w *Wiki) checkLinks(p Page, anchors map[string]map[string]bool, add func(Issue)) {
	dir := path.Dir(p.Path)
	EachLine(p.Text, func(i int, line string) {
		for _, m := range linkRe.FindAllStringSubmatch(stripCode(line), -1) {
			raw := m[2]
			if strings.Contains(raw, "://") || strings.HasPrefix(raw, "mailto:") {
				continue
			}
			target, anchor, _ := strings.Cut(raw, "#")
			if target == "" {
				if anchor != "" && !anchors[p.Path][strings.ToLower(anchor)] {
					add(Issue{Page: p.Path, Line: i + 1, Kind: IssueBrokenAnchor, Severity: SeverityWarning, Message: i18n.Tf("wiki.check.anchor_local", anchor)})
				}
				continue
			}
			rel := path.Clean(path.Join(dir, target))
			if rel == ".." || strings.HasPrefix(rel, "../") {
				// Out of the wiki: a file of the project.
				file := path.Clean(path.Join(Dir, rel))
				if !insideProject(file) {
					continue
				}
				if _, err := statConfined(w.Root, file); errors.Is(err, fs.ErrNotExist) {
					add(Issue{Page: p.Path, Line: i + 1, Kind: IssueBrokenLink, Severity: SeverityError, Message: i18n.Tf("wiki.check.link_file", target)})
				}
				continue
			}
			if _, ok := w.Page(rel); !ok {
				if _, err := statConfined(w.Root, Dir+"/"+rel); err != nil {
					add(Issue{Page: p.Path, Line: i + 1, Kind: IssueBrokenLink, Severity: SeverityError, Message: i18n.Tf("wiki.check.link_page", target)})
				}
				continue
			}
			if anchor != "" && !anchors[rel][strings.ToLower(anchor)] {
				add(Issue{Page: p.Path, Line: i + 1, Kind: IssueBrokenAnchor, Severity: SeverityWarning, Message: i18n.Tf("wiki.check.anchor", anchor, rel)})
			}
		}
	})
}

// stripCode removes the inline code of a line (links in code are text).
func stripCode(line string) string {
	parts := strings.Split(line, "`")
	for i := 1; i < len(parts); i += 2 {
		parts[i] = ""
	}
	return strings.Join(parts, "")
}

type citedFile struct {
	file string // relative to the project
	line int
}

// checkTags checks the tag lines of a page and returns the files their
// sources cite.
func checkTags(p Page, add func(Issue)) []citedFile {
	var out []citedFile
	tagged := false
	EachLine(p.Text, func(i int, line string) {
		if !anyTagRe.MatchString(line) {
			return
		}
		tagged = true
		m := tagLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			if strings.Contains(line, "—") || strings.Contains(line, "·") {
				add(Issue{Page: p.Path, Line: i + 1, Kind: IssueTag, Severity: SeverityError,
					Message: i18n.T("wiki.check.tag")})
			}
			return
		}
		src := strings.TrimSpace(m[4])
		if m[1] == TagConfirmed && src == "" {
			add(Issue{Page: p.Path, Line: i + 1, Kind: IssueTagSource, Severity: SeverityWarning, Message: i18n.T("wiki.check.tag_source")})
		}
		for _, f := range sourceFiles(src) {
			out = append(out, citedFile{file: f, line: i + 1})
		}
	})
	if !tagged && p.Path != IndexPage && !narrative(p.Path) && strings.TrimSpace(p.Body) != "" {
		add(Issue{Page: p.Path, Kind: IssueUntagged, Severity: SeverityWarning, Message: i18n.T("wiki.check.untagged")})
	}
	return out
}

// sourceFiles returns the project files a tag source names (file:line,
// files separated by commas; a folder or a free text is not a file).
func sourceFiles(src string) []string {
	var out []string
	for _, part := range strings.Split(src, ",") {
		f := strings.Trim(strings.TrimSpace(part), "`")
		f = sourceLnRe.ReplaceAllString(f, "")
		if f == "" || strings.ContainsAny(f, " *?<>") || strings.Contains(f, "://") || strings.HasSuffix(f, "/") {
			continue
		}
		if !strings.Contains(path.Base(f), ".") && !strings.Contains(f, "/") {
			continue // a word, not a file
		}
		out = append(out, path.Clean(f))
	}
	return out
}

// outLinks returns the project files a page links to out of the wiki.
func outLinks(p Page) []citedFile {
	var out []citedFile
	dir := path.Dir(p.Path)
	EachLine(p.Text, func(i int, line string) {
		for _, m := range linkRe.FindAllStringSubmatch(stripCode(line), -1) {
			if strings.Contains(m[2], "://") {
				continue
			}
			target, _, _ := strings.Cut(m[2], "#")
			if target == "" {
				continue
			}
			rel := path.Clean(path.Join(dir, target))
			if rel == ".." || strings.HasPrefix(rel, "../") {
				out = append(out, citedFile{file: path.Clean(path.Join(Dir, rel)), line: i + 1})
			}
		}
	})
	return out
}

// insideProject reports whether a cleaned relative path stays in the project.
func insideProject(file string) bool {
	return file != "" && file != "." && file != ".." && !strings.HasPrefix(file, "../") && !path.IsAbs(file)
}

func checkGodNodes(ix Index, add func(Issue)) {
	if !ix.HasTable {
		if len(ix.GodNodes) > 0 {
			add(Issue{Page: IndexPage, Kind: IssueGodNodeTable, Severity: SeverityWarning,
				Message: i18n.Tf("wiki.check.god_node_table", len(ix.GodNodes))})
		}
		return
	}
	computed := map[string]GodNode{}
	for _, g := range ix.GodNodes {
		computed[strings.ToLower(g.Concept)] = g
	}
	declared := map[string]bool{}
	for _, d := range ix.Declared {
		key := strings.ToLower(d.Concept)
		declared[key] = true
		g, ok := computed[key]
		switch {
		case !ok:
			add(Issue{Page: IndexPage, Kind: IssueGodNodeStale, Severity: SeverityWarning, Fixable: true,
				Message: i18n.Tf("wiki.check.god_node_stale", d.Concept, MinGodNodePages)})
		case g.row == "":
			add(Issue{Page: IndexPage, Kind: IssueGodNodeChanged, Severity: SeverityWarning, Fixable: true,
				Message: i18n.Tf("wiki.check.god_node_changed", g.Concept, strings.Join(g.Pages, ", "), g.Criticality)})
		}
	}
	for _, g := range ix.GodNodes {
		if !declared[strings.ToLower(g.Concept)] {
			add(Issue{Page: IndexPage, Kind: IssueGodNodeMissing, Severity: SeverityWarning, Fixable: true,
				Message: i18n.Tf("wiki.check.god_node_missing", g.Concept, len(g.Pages), strings.Join(g.Pages, ", "))})
		}
	}
}

// FixResult tells what Fix changed.
type FixResult struct {
	Pages []string `json:"pages"` // relative to docs/wiki/
}

// Fix repairs what oh can compute: the confidence of the frontmatter of
// each page, and the god nodes table of the index. It never follows a
// symbolic link.
func Fix(root string, now time.Time) (FixResult, error) {
	w, err := Load(root)
	if err != nil {
		return FixResult{}, err
	}
	var res FixResult
	for _, p := range w.Pages {
		want := pageConfidence(p.Text)
		if !p.Frontmatter.Present || p.Frontmatter.Err != nil || want == "" || strings.TrimSpace(p.Frontmatter.Confidence) == want {
			continue
		}
		text := setFrontmatter(p.Text, map[string]string{"confidence": want})
		if err := writePage(root, p.Path, text); err != nil {
			return res, err
		}
		res.Pages = append(res.Pages, p.Path)
	}
	changed, err := RefreshGodNodes(root, now)
	if err != nil {
		return res, err
	}
	if changed && !slices.Contains(res.Pages, IndexPage) {
		res.Pages = append(res.Pages, IndexPage)
	}
	return res, nil
}

// RefreshGodNodes rewrites the god nodes table of the index with the god
// nodes oh computes (the index needs a god nodes section; its date is
// updated when the table changes).
func RefreshGodNodes(root string, now time.Time) (bool, error) {
	w, err := Load(root)
	if err != nil {
		return false, err
	}
	idx, ok := w.Page(IndexPage)
	if !ok {
		return false, nil
	}
	ix := w.Index()
	if !ix.HasTable {
		return false, nil
	}
	text, changed := replaceGodNodeTable(idx.Text, ix)
	if !changed {
		return false, nil
	}
	if idx.Frontmatter.Present && idx.Frontmatter.Err == nil {
		text = setFrontmatter(text, map[string]string{"updated": now.Format(time.DateOnly)})
	}
	return true, writePage(root, IndexPage, text)
}

// replaceGodNodeTable replaces the table of the god nodes section of an
// index text (the text around it stays).
func replaceGodNodeTable(text string, ix Index) (string, bool) {
	lines := strings.Split(text, "\n")
	start, end := -1, -1
	in := false
	EachLine(text, func(i int, line string) {
		if strings.HasPrefix(line, "## ") {
			in = isGodNodeHeading(strings.TrimSpace(line[3:])) && start < 0
			return
		}
		if !in {
			return
		}
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			if start < 0 {
				start = i
			}
			end = i + 1
		} else if start >= 0 && strings.TrimSpace(line) != "" {
			in = false
		}
	})
	table := ix.GodNodeTable(0)
	if start < 0 {
		if len(table) == 0 {
			return text, false
		}
		// An empty section: the table goes right after its heading.
		for i, line := range lines {
			if strings.HasPrefix(line, "## ") && isGodNodeHeading(line[3:]) {
				out := append(append(append([]string{}, lines[:i+1]...), append([]string{""}, table...)...), lines[i+1:]...)
				return strings.Join(out, "\n"), true
			}
		}
		return text, false
	}
	if len(table) == 0 {
		// Keep the header: no row.
		table = lines[start:min(start+2, end)]
	}
	if slices.Equal(lines[start:end], table) {
		return text, false
	}
	out := append(append(append([]string{}, lines[:start]...), table...), lines[end:]...)
	return strings.Join(out, "\n"), true
}
