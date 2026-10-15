package docstest

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// linkFiles lists every Markdown file of the repository, relative to root
// and slash-separated: node_modules, hidden directories, testdata, the
// embedded copy of the hub content and the fixtures of the eval cases (the
// projects the agents work on) left out.
func linkFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			name := d.Name()
			if rel != "." && (name == "node_modules" || name == "testdata" || strings.HasPrefix(name, ".") ||
				rel == "cli/internal/hubcontent/hub" || (name == "fixture" && strings.HasPrefix(rel, "evals/"))) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// mdLink is a link of a Markdown file.
type mdLink struct {
	line   int
	target string
}

var (
	fenceLine  = regexp.MustCompile("^\\s*(```+|~~~+)")
	inlineCode = regexp.MustCompile("`+[^`]*`+")
	// [text](target "title"), ![alt](target), target between <> or not.
	inlineLink = regexp.MustCompile(`!?\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^\s()]*(?:\([^\s()]*\)[^\s()]*)*)(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)`)
	refDef     = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)`)
	hrefAttr   = regexp.MustCompile(`<a\s[^>]*href="([^"]+)"`)
	idAttr     = regexp.MustCompile(`<a\s[^>]*(?:id|name)="([^"]+)"`)
	atxHeading = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.*?)\s*(?:\s#+)?\s*$`)
	setextLine = regexp.MustCompile(`^\s{0,3}(=+|-+)\s*$`)
)

// proseLines returns the lines of a Markdown text with the fenced code
// blocks and the front matter blanked (line numbers kept).
func proseLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, len(lines))
	fence := ""
	front := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	for i, l := range lines {
		if front {
			if i > 0 && strings.TrimSpace(l) == "---" {
				front = false
			}
			continue
		}
		if m := fenceLine.FindStringSubmatch(l); m != nil {
			// A closing fence is as long as the opening one at least, of the
			// same character, without info string.
			switch {
			case fence == "":
				fence = m[1]
				continue
			case m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), m[1][:1])) == "":
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		out[i] = l
	}
	return out
}

// markdownLinks extracts the links of a Markdown text (code left out).
func markdownLinks(text string) []mdLink {
	var links []mdLink
	for i, l := range proseLines(text) {
		if l == "" {
			continue
		}
		l = inlineCode.ReplaceAllString(l, "")
		for _, m := range inlineLink.FindAllStringSubmatch(l, -1) {
			links = append(links, mdLink{i + 1, strings.Trim(m[1], "<>")})
		}
		if m := refDef.FindStringSubmatch(l); m != nil {
			links = append(links, mdLink{i + 1, strings.Trim(m[1], "<>")})
		}
		for _, m := range hrefAttr.FindAllStringSubmatch(l, -1) {
			links = append(links, mdLink{i + 1, m[1]})
		}
	}
	return links
}

var (
	headingLink  = regexp.MustCompile(`!?\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\([^)]*\)`)
	headingTag   = regexp.MustCompile(`<[^>]+>`)
	headingEmph  = regexp.MustCompile(`(^|[^\p{L}\p{N}])(\*{1,3}|_{1,3})([^*_]+?)(\*{1,3}|_{1,3})($|[^\p{L}\p{N}])`)
	headingCode  = regexp.MustCompile("`+([^`]*)`+")
	headingStars = regexp.MustCompile(`\*+`)
)

// headingText is the text GitHub renders for a heading: code spans keep
// their content, links their text, tags and emphasis markers go away.
func headingText(h string) string {
	var codes []string
	h = headingCode.ReplaceAllStringFunc(h, func(s string) string {
		codes = append(codes, headingCode.FindStringSubmatch(s)[1])
		return "\x00" + strconv.Itoa(len(codes)-1) + "\x00"
	})
	h = headingLink.ReplaceAllString(h, "$1")
	h = headingTag.ReplaceAllString(h, "")
	for prev := ""; prev != h; {
		prev = h
		h = headingEmph.ReplaceAllString(h, "$1$3$5")
	}
	h = headingStars.ReplaceAllString(h, "")
	for i, c := range codes {
		h = strings.Replace(h, "\x00"+strconv.Itoa(i)+"\x00", c, 1)
	}
	return h
}

// slug is the anchor GitHub gives a heading (github-slugger): lower case,
// letters (accents kept), marks, digits, underscores, hyphens and spaces
// kept, every space becoming a hyphen; the rest goes away.
func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || unicode.In(r, unicode.Pc):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// anchors returns the anchors of a Markdown text: its headings (a repeated
// slug gets -1, -2… as on GitHub) and the ids of its <a> tags.
func anchors(text string) map[string]bool {
	out := map[string]bool{}
	seen := map[string]int{}
	add := func(h string) {
		s := slug(headingText(h))
		if n, ok := seen[s]; ok {
			seen[s] = n + 1
			s = fmt.Sprintf("%s-%d", s, n+1)
		} else {
			seen[s] = 0
		}
		out[s] = true
	}
	lines := proseLines(text)
	for i, l := range lines {
		for _, m := range idAttr.FindAllStringSubmatch(l, -1) {
			out[m[1]] = true
		}
		if m := atxHeading.FindStringSubmatch(l); m != nil {
			add(m[2])
			continue
		}
		if i > 0 && setextLine.MatchString(l) {
			prev := strings.TrimSpace(lines[i-1])
			if prev != "" && !strings.HasPrefix(prev, "|") && !strings.HasPrefix(prev, "#") && !strings.HasPrefix(prev, "-") &&
				!strings.HasPrefix(prev, "*") && !strings.HasPrefix(prev, ">") && !strings.HasPrefix(prev, "<") &&
				(i < 2 || strings.TrimSpace(lines[i-2]) == "") {
				add(prev)
			}
		}
	}
	return out
}

func TestSlug(t *testing.T) {
	for h, want := range map[string]string{
		"Entrées calculées (`from:`)":                  "entrées-calculées-from",
		"`aliases` : autres noms du workflow":          "aliases--autres-noms-du-workflow",
		"oh session delete":                            "oh-session-delete",
		"3. Réglages : jour 1 (13 octobre)":            "3-réglages--jour-1-13-octobre",
		"**Gras** et _italique_, `snake_case`":         "gras-et-italique-snake_case",
		"[Lien](x.md) — tiret cadratin":                "lien--tiret-cadratin",
		"Qu'est-ce qu'un « workflow » ?":               "quest-ce-quun--workflow--",
		"CP-R3 : vérification par oh <sup>1</sup>":     "cp-r3--vérification-par-oh-1",
		"`oh run --headless` / `--output`":             "oh-run---headless----output",
		"Étape 2 : l'équipe":                           "étape-2--léquipe",
		"Fan-out (expérimental) ⚠":                     "fan-out-expérimental-",
		"Variables `{{ .oh.mode }}` et `{{ data … }}`": "variables--ohmode--et--data--",
	} {
		if got := slug(headingText(h)); got != want {
			t.Errorf("slug(%q) = %q, want %q", h, got, want)
		}
	}
	a := anchors("# Titre\n\n## Usage\n\n## Usage\n\nTexte\n---\n\n```md\n# Pas un titre\n```\n<a id=\"ici\"></a>\n")
	for _, want := range []string{"titre", "usage", "usage-1", "texte", "ici"} {
		if !a[want] {
			t.Errorf("anchor %q missing in %v", want, a)
		}
	}
	if a["pas-un-titre"] {
		t.Error("heading in a code block taken")
	}
}

func TestMarkdownLinks(t *testing.T) {
	text := "---\nupdated: x\n---\n[a](a.md) et ![img](<img dir/b.png> \"t\") `[pas](lien.md)`\n" +
		"````md\n```bash\n```\n[non](code.md)\n````\n[ref]: ref.md#ancre\n<a href=\"c.md\">c</a> [p](f(1).md)\n"
	var got []string
	for _, l := range markdownLinks(text) {
		got = append(got, fmt.Sprintf("%d:%s", l.line, l.target))
	}
	want := "4:a.md 4:img dir/b.png 10:ref.md#ancre 11:f(1).md 11:c.md"
	if strings.Join(got, " ") != want {
		t.Fatalf("links = %v, want %s", got, want)
	}
}

// TestDocLinks checks the relative links of every Markdown file of the
// repository: the file or directory exists and, for a Markdown file, the
// anchor is one of its headings as GitHub generates them.
func TestDocLinks(t *testing.T) {
	root := repoRoot(t)
	files := linkFiles(t, root)
	texts := map[string]string{}
	read := func(rel string) (string, bool) {
		if s, ok := texts[rel]; ok {
			return s, true
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", false
		}
		texts[rel] = string(b)
		return texts[rel], true
	}
	anchorsOf := map[string]map[string]bool{}
	var broken []string
	checked, withAnchor := 0, 0
	for _, f := range files {
		text, _ := read(f)
		for _, l := range markdownLinks(text) {
			target := l.target
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "//") ||
				regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`).MatchString(target) || strings.Contains(target, "{{") {
				continue
			}
			checked++
			p, frag, _ := strings.Cut(target, "#")
			if q := strings.Index(p, "?"); q >= 0 {
				p = p[:q]
			}
			if u, err := url.PathUnescape(p); err == nil {
				p = u
			}
			dest := f
			if p != "" {
				if strings.HasPrefix(p, "/") {
					dest = path.Clean(strings.TrimPrefix(p, "/"))
				} else {
					dest = path.Join(path.Dir(f), p)
				}
				if dest == ".." || strings.HasPrefix(dest, "../") {
					broken = append(broken, fmt.Sprintf("%s:%d: %s (outside the repository)", f, l.line, target))
					continue
				}
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dest))); err != nil {
					broken = append(broken, fmt.Sprintf("%s:%d: %s (no such file)", f, l.line, target))
					continue
				}
			}
			if frag == "" || !strings.HasSuffix(dest, ".md") {
				continue
			}
			withAnchor++
			if u, err := url.PathUnescape(frag); err == nil {
				frag = u
			}
			a, ok := anchorsOf[dest]
			if !ok {
				s, _ := read(dest)
				a = anchors(s)
				anchorsOf[dest] = a
			}
			if !a[frag] {
				broken = append(broken, fmt.Sprintf("%s:%d: %s (no such anchor)", f, l.line, target))
			}
		}
	}
	t.Logf("%d files, %d relative links, %d anchors checked", len(files), checked, withAnchor)
	if len(broken) > 0 {
		t.Fatalf("%d broken links:\n%s", len(broken), strings.Join(broken, "\n"))
	}
}
