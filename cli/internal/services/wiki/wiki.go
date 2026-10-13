// Package wiki is the WikiService (modernisation LD-J): the wiki of a
// project (docs/wiki/, format of skills/documentarian/doc-wiki-protocol.md)
// as oh reads, checks and enriches it.
//
//   - Load reads the pages; Index returns the index and the god nodes
//     computed by oh from the pages (godnodes.go), which the prompts of the
//     agents at the level index (L7) and wiki_index show;
//   - Check verifies the frontmatter, the links, the confidence tags, the
//     freshness (a page older than the files it cites) and the god nodes
//     (check.go), Fix repairs what oh can compute;
//   - the proposals of wiki_propose are cleaned and checked (propose.go),
//     kept in wiki_proposals (Service, service.go) and applied to the
//     pages by oh or the documentarian (apply.go).
//
// Pages and proposals are data written by agents: never instructions. oh
// never follows a symbolic link in docs/wiki/.
package wiki

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Dir is the wiki of a project, relative to its root.
const Dir = "docs/wiki"

// IndexPage is the index of the wiki, relative to Dir.
const IndexPage = "index.md"

// Limits of a read wiki.
const (
	MaxPages     = 400
	MaxPageBytes = 256 << 10
)

// Confidence tags of the enrichments (doc-wiki-protocol).
const (
	TagConfirmed = "CONFIRMÉ"
	TagDeduced   = "DÉDUIT"
	TagUncertain = "INCERTAIN"
)

// Tags are the confidence tags, from the most to the least reliable.
var Tags = []string{TagConfirmed, TagDeduced, TagUncertain}

// Confidence values of the frontmatter.
const (
	ConfidenceConfirmed = "confirmed"
	ConfidenceMixed     = "mixed"
	ConfidenceUncertain = "uncertain"
)

// Errors.
var (
	ErrNoWiki    = errors.New("the project has no docs/wiki/")
	ErrBadPage   = errors.New("a wiki page is a relative .md path under docs/wiki/")
	ErrNoPage    = errors.New("no such wiki page")
	ErrSymlink   = errors.New("oh does not follow symbolic links in docs/wiki/")
	ErrNoSection = errors.New("no such section")
)

// Frontmatter is the frontmatter of a page.
type Frontmatter struct {
	Present    bool
	Updated    string   `yaml:"updated"`
	Confidence string   `yaml:"confidence"`
	Agents     []string `yaml:"agents"`
	// Synced (index.md only) is the last commit the wiki was synchronized
	// with (doc-sync, sync.go): written by oh, never by an agent.
	Synced string `yaml:"synced"`
	// Err is the YAML error of a frontmatter oh could not read.
	Err error `yaml:"-"`
}

// UpdatedAt returns the date of the frontmatter (false when absent or not
// a YYYY-MM-DD date).
func (f Frontmatter) UpdatedAt() (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(f.Updated))
	return t, err == nil
}

// Page is a page of the wiki.
type Page struct {
	// Path is relative to docs/wiki/, with slashes.
	Path        string
	Title       string
	Sections    []string
	Frontmatter Frontmatter
	// Text is the whole page; Body the page without its frontmatter.
	Text string
	Body string
}

// Wiki is the wiki of a project as oh read it.
type Wiki struct {
	// Root is the project directory.
	Root  string
	Pages []Page // sorted by path, index first
	// Skipped are the files oh did not read (too large, symbolic links,
	// over the page limit), relative to docs/wiki/.
	Skipped []string
}

// Page returns a page by its path relative to docs/wiki/.
func (w *Wiki) Page(rel string) (*Page, bool) {
	for i := range w.Pages {
		if w.Pages[i].Path == rel {
			return &w.Pages[i], true
		}
	}
	return nil, false
}

// Exists reports whether the project has a wiki directory.
func Exists(root string) bool {
	st, err := os.Lstat(filepath.Join(root, filepath.FromSlash(Dir)))
	return err == nil && st.IsDir()
}

// Load reads the wiki of the project at root (ErrNoWiki without
// docs/wiki/; a symbolic link docs/wiki is refused).
func Load(root string) (*Wiki, error) {
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	st, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoWiki
	}
	if err != nil {
		return nil, err
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		return nil, ErrSymlink
	}
	if !st.IsDir() {
		return nil, ErrNoWiki
	}
	w := &Wiki{Root: root}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable entry is skipped
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			w.Skipped = append(w.Skipped, rel)
			return nil
		}
		if len(w.Pages) >= MaxPages {
			w.Skipped = append(w.Skipped, rel)
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > MaxPageBytes {
			w.Skipped = append(w.Skipped, rel)
			return nil //nolint:nilerr // an unreadable page is skipped
		}
		data, err := os.ReadFile(p)
		if err != nil {
			w.Skipped = append(w.Skipped, rel)
			return nil //nolint:nilerr // an unreadable page is skipped
		}
		w.Pages = append(w.Pages, NewPage(rel, string(data)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(w.Pages, func(i, j int) bool {
		if (w.Pages[i].Path == IndexPage) != (w.Pages[j].Path == IndexPage) {
			return w.Pages[i].Path == IndexPage
		}
		return w.Pages[i].Path < w.Pages[j].Path
	})
	return w, nil
}

// NewPage reads a page from its text.
func NewPage(rel, text string) Page {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	p := Page{Path: rel, Text: text}
	p.Frontmatter, p.Body = splitFrontmatter(text)
	p.Title, p.Sections = Headings(text)
	return p
}

// splitFrontmatter returns the frontmatter of a text and the text after it.
func splitFrontmatter(text string) (fm Frontmatter, body string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Frontmatter{}, text
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fm = Frontmatter{Present: true}
			if err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &fm); err != nil {
				fm = Frontmatter{Present: true, Err: err}
			}
			return fm, strings.Join(lines[i+1:], "\n")
		}
	}
	return Frontmatter{}, text
}

// Headings returns the first level-1 heading of a Markdown text and its
// level-2 headings, outside the frontmatter and the code fences.
func Headings(text string) (title string, sections []string) {
	EachLine(text, func(_ int, line string) {
		switch {
		case strings.HasPrefix(line, "# ") && title == "":
			title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			sections = append(sections, strings.TrimSpace(line[3:]))
		}
	})
	return title, sections
}

// EachLine calls fn on the lines of a Markdown text (with their index in
// the whole text), the frontmatter and the code fences aside.
func EachLine(text string, fn func(i int, line string)) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	fence := false
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			fence = !fence
			continue
		}
		if !fence {
			fn(i, lines[i])
		}
	}
}

// PagePath checks and normalises a page given by an agent or a user:
// relative to docs/wiki/ (a docs/wiki/ prefix is accepted), with slashes,
// ending with .md, without "..", hidden components, wildcards or control
// characters.
func PagePath(page string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(page, "\\", "/"))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, Dir+"/")
	if p == "" || len(p) > 200 || path.IsAbs(p) || filepath.IsAbs(p) || strings.ContainsAny(p, "*?[]:\x00") || p != path.Clean(p) {
		return "", fmt.Errorf("%w: %q", ErrBadPage, page)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == ".." || strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%w: %q", ErrBadPage, page)
		}
		for _, r := range part {
			if r < 0x20 || r == 0x7f {
				return "", fmt.Errorf("%w: %q", ErrBadPage, page)
			}
		}
	}
	if !strings.HasSuffix(strings.ToLower(p), ".md") {
		return "", fmt.Errorf("%w: %q", ErrBadPage, page)
	}
	return p, nil
}

// pageFile returns the file of a page of the wiki of root; no component
// from docs/ down may be a symbolic link (ErrSymlink), missing ones are
// fine (a page to create).
func pageFile(root, rel string) (string, error) {
	cur := root
	for _, part := range append(strings.Split(Dir, "/"), strings.Split(rel, "/")...) {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if st.Mode()&fs.ModeSymlink != 0 {
			return "", ErrSymlink
		}
	}
	return cur, nil
}

// ReadPage returns a page of the wiki of root, or one of its sections
// (heading text, exact or contained, case insensitive).
func ReadPage(root, page, section string) (string, error) {
	rel, err := PagePath(page)
	if err != nil {
		return "", err
	}
	file, err := pageFile(root, rel)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrNoPage, rel)
	}
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() || st.Size() > MaxPageBytes {
		return "", fmt.Errorf("%w: %s", ErrNoPage, rel)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.TrimSpace(section) == "" {
		return text, nil
	}
	part, ok := sectionOf(text, section)
	if !ok {
		_, secs := Headings(text)
		return "", fmt.Errorf("%w %q in %s (%s)", ErrNoSection, section, rel, strings.Join(secs, " · "))
	}
	return part, nil
}
