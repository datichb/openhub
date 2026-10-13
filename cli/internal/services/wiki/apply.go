package wiki

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Application of a proposal to a page (doc-wiki-protocol, incremental
// enrichment): the content goes at the end of its section (created at the
// end of the page when missing; at the end of the page without a
// section), as a list item followed by its tag line written by oh (tag,
// author seen by oh, date, source); a missing page is created with its
// frontmatter and a title. The frontmatter of the page is updated
// (updated, agents, confidence), then the god nodes table of the index.

// Entry is what is applied to a page.
type Entry struct {
	Page    string // relative to docs/wiki/
	Section string
	Content string // cleaned (CleanContent)
	Tag     string
	Source  string
	Agent   string // author seen by oh
	Date    time.Time
}

// TagLine renders the tag line of an entry.
func (e Entry) TagLine() string {
	parts := []string{"`" + e.Tag + "`", firstNonEmpty(e.Agent, "?"), e.Date.Format(time.DateOnly)}
	if e.Source != "" {
		parts = append(parts, e.Source)
	}
	return "— " + strings.Join(parts, " · ")
}

// block renders the entry as a list item with its tag line.
func (e Entry) block() []string {
	lines := strings.Split(strings.TrimRight(e.Content, "\n"), "\n")
	out := make([]string, 0, len(lines)+1)
	for i, l := range lines {
		switch {
		case i == 0 && (strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")):
			out = append(out, "- "+l[2:])
		case i == 0:
			out = append(out, "- "+l)
		case strings.TrimSpace(l) == "":
			out = append(out, "")
		default:
			out = append(out, "  "+l)
		}
	}
	return append(out, "  "+e.TagLine())
}

// Apply applies an entry to the wiki of root, then refreshes the god nodes
// table of the index.
func Apply(root string, e Entry) error {
	rel, err := PagePath(e.Page)
	if err != nil {
		return err
	}
	if !Exists(root) {
		return ErrNoWiki
	}
	file, err := pageFile(root, rel)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(file)
	var text string
	switch {
	case errors.Is(err, fs.ErrNotExist):
		text = newPageText(rel, e)
	case err != nil:
		return err
	default:
		text = insertEntry(strings.ReplaceAll(string(data), "\r\n", "\n"), e)
	}
	text = enrichFrontmatter(text, e)
	if err := writePage(root, rel, text); err != nil {
		return err
	}
	_, err = RefreshGodNodes(root, e.Date)
	return err
}

// newPageText returns a new page holding the entry.
func newPageText(rel string, e Entry) string {
	title := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	title = strings.ReplaceAll(strings.ReplaceAll(title, "-", " "), "_", " ")
	if r := []rune(title); len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
		title = string(r)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nupdated: %s\nconfidence: %s\nagents: [%s]\n---\n\n# %s\n", e.Date.Format(time.DateOnly), confidenceOf(e.Tag), firstNonEmpty(e.Agent, "?"), title)
	if e.Section != "" {
		fmt.Fprintf(&b, "\n## %s\n", e.Section)
	}
	b.WriteString("\n" + strings.Join(e.block(), "\n") + "\n")
	return b.String()
}

// insertEntry adds the entry at the end of its section.
func insertEntry(text string, e Entry) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	block := e.block()
	if e.Section == "" {
		return strings.Join(append(append(lines, ""), block...), "\n") + "\n"
	}
	want := normHeading(e.Section)
	start, level := -1, 0
	EachLine(text, func(i int, line string) {
		if start >= 0 {
			return
		}
		if m := headingRe.FindStringSubmatch(line); len(m) == 3 && len(m[1]) >= 2 && normHeading(m[2]) == want {
			start, level = i, len(m[1])
		}
	})
	if start < 0 {
		return strings.Join(append(append(lines, "", "## "+e.Section, ""), block...), "\n") + "\n"
	}
	end := len(lines)
	EachLine(text, func(i int, line string) {
		if i <= start || end != len(lines) {
			return
		}
		if m := headingRe.FindStringSubmatch(line); len(m) == 3 && len(m[1]) <= level {
			end = i
		}
	})
	// After the last non blank line of the section.
	at := end
	for at > start+1 && strings.TrimSpace(lines[at-1]) == "" {
		at--
	}
	ins := append([]string{}, block...)
	if at == start+1 || strings.TrimSpace(lines[at-1]) != "" {
		ins = append([]string{""}, ins...)
	}
	if at < len(lines) {
		ins = append(ins, "")
	}
	out := append(append(append([]string{}, lines[:at]...), ins...), trimLeadingBlank(lines[at:])...)
	return strings.Join(out, "\n") + "\n"
}

func trimLeadingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}

// normHeading compares headings without case, marks nor emoji.
func normHeading(h string) string {
	h = strings.ToLower(plain(strings.TrimLeft(strings.TrimSpace(h), "#")))
	return strings.Join(strings.FieldsFunc(h, func(r rune) bool { return !wordRune(r) && r != '\'' }), " ")
}

func confidenceOf(tag string) string {
	switch tag {
	case TagUncertain:
		return ConfidenceUncertain
	case TagDeduced:
		return ConfidenceMixed
	}
	return ConfidenceConfirmed
}

// enrichFrontmatter updates the frontmatter of a page after an entry:
// date, author among the agents, confidence of its tags.
func enrichFrontmatter(text string, e Entry) string {
	fm, _ := splitFrontmatter(text)
	if !fm.Present || fm.Err != nil {
		head := fmt.Sprintf("---\nupdated: %s\nconfidence: %s\nagents: [%s]\n---\n\n", e.Date.Format(time.DateOnly), confidenceOf(e.Tag), firstNonEmpty(e.Agent, "?"))
		if fm.Present {
			return text // unreadable: oh does not rewrite it (wiki_check reports it)
		}
		return head + text
	}
	agents := fm.Agents
	if e.Agent != "" && !slices.Contains(agents, e.Agent) {
		agents = append(append([]string{}, agents...), e.Agent)
	}
	set := map[string]string{"updated": e.Date.Format(time.DateOnly), "agents": "[" + strings.Join(agents, ", ") + "]"}
	if c := pageConfidence(text); c != "" {
		set["confidence"] = c
	}
	return setFrontmatter(text, set)
}

// setFrontmatter sets keys of the frontmatter of a text (values written as
// given), keeping the other lines.
func setFrontmatter(text string, set map[string]string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return text
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return text
	}
	done := map[string]bool{}
	for i := 1; i < end; i++ {
		k, _, ok := strings.Cut(lines[i], ":")
		k = strings.TrimSpace(k)
		if v, want := set[k]; ok && want && !strings.HasPrefix(lines[i], " ") {
			lines[i] = k + ": " + v
			done[k] = true
		}
	}
	var extra []string
	for _, k := range []string{"updated", "confidence", "agents", "synced"} {
		if v, ok := set[k]; ok && !done[k] {
			extra = append(extra, k+": "+v)
		}
	}
	out := append(append(append([]string{}, lines[:end]...), extra...), lines[end:]...)
	// Keep the YAML readable: a value oh could not write stays as it was.
	if fm, _ := splitFrontmatter(strings.Join(out, "\n")); fm.Err != nil {
		return text
	}
	return strings.Join(out, "\n")
}

// writePage writes a page of the wiki atomically (0644), never through a
// symbolic link.
func writePage(root, rel, text string) error {
	file, err := pageFile(root, rel)
	if err != nil {
		return err
	}
	if err := mkdirNoLink(root, path.Dir(rel)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".oh-wiki-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, file)
}

// mkdirNoLink creates the folders of a page under docs/wiki/, refusing a
// symbolic link on the way.
func mkdirNoLink(root, relDir string) error {
	cur := filepath.Join(root, filepath.FromSlash(Dir))
	if relDir == "." || relDir == "" {
		return nil
	}
	for _, part := range strings.Split(relDir, "/") {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Mkdir(cur, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case st.Mode()&fs.ModeSymlink != 0:
			return ErrSymlink
		case !st.IsDir():
			return fmt.Errorf("%w: %s is not a folder", ErrBadPage, part)
		}
	}
	return nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
