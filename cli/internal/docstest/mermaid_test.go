// Package docstest holds tests over the documentation of the repository.
package docstest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// repoRoot is the root of the repository (this package is cli/internal/docstest).
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "diagrams")); err != nil {
		t.Skipf("no docs at %s (not the repository)", root)
	}
	return root
}

// docFiles lists the Markdown files of docs/ and of the repository root,
// relative to root (node_modules left out).
func docFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	top, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range top {
		files = append(files, filepath.Base(f))
	}
	err = filepath.WalkDir(filepath.Join(root, "docs"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			rel, _ := filepath.Rel(root, p)
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

// mermaidBlock is a ```mermaid block of a Markdown file.
type mermaidBlock struct {
	file   string // relative to the repository root
	line   int    // line of the opening fence (1-based)
	body   string
	source string // diagram source the block declares to copy ("" = none), relative to file
}

// sourceLine names the diagram source a block is an exact copy of
// ("> Source du diagramme : [...](../diagrams/x.mermaid)").
var sourceLine = regexp.MustCompile(`^>\s*(?:Source du diagramme|Diagram source)\s*:.*\]\(([^)]+\.mermaid)\)`)

// mermaidBlocks extracts the ```mermaid blocks of a Markdown text (fences
// indented in lists included).
func mermaidBlocks(file, text string) []mermaidBlock {
	lines := strings.Split(text, "\n")
	var out []mermaidBlock
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```mermaid" {
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		b := mermaidBlock{file: file, line: i + 1}
		var body []string
		j := i + 1
		for ; j < len(lines) && strings.TrimSpace(lines[j]) != "```"; j++ {
			body = append(body, strings.TrimPrefix(lines[j], indent))
		}
		b.body = strings.Join(body, "\n")
		for k := j + 1; k < len(lines); k++ { // the first line after the block
			if l := strings.TrimSpace(lines[k]); l != "" {
				if m := sourceLine.FindStringSubmatch(l); m != nil {
					b.source = m[1]
				}
				break
			}
		}
		out = append(out, b)
		i = j
	}
	return out
}

func allBlocks(t *testing.T, root string) []mermaidBlock {
	t.Helper()
	var blocks []mermaidBlock
	for _, f := range docFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, mermaidBlocks(f, string(data))...)
	}
	return blocks
}

func normalize(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func TestMermaidBlocksExtraction(t *testing.T) {
	text := "intro\n```mermaid\nflowchart LR\n  A --> B\n```\n\n> Source du diagramme : [`x`](../diagrams/x.mermaid)\n\n" +
		"- item\n\n  ```mermaid\n  sequenceDiagram\n    A->>B: hi\n  ```\n\ntext\n\n> Diagram source: [y](y.mermaid)\n```go\nx\n```\n"
	got := mermaidBlocks("d.md", text)
	if len(got) != 2 {
		t.Fatalf("blocks = %d, want 2: %+v", len(got), got)
	}
	if got[0].body != "flowchart LR\n  A --> B" || got[0].source != "../diagrams/x.mermaid" || got[0].line != 2 {
		t.Errorf("first block = %+v", got[0])
	}
	if got[1].body != "sequenceDiagram\n  A->>B: hi" || got[1].source != "" {
		t.Errorf("indented block, source line too far = %+v", got[1])
	}
}

// P0.7: the diagrams are Mermaid, embedded in the docs; no SVG rendered by
// hand is left to go stale.
func TestNoDiagramSVG(t *testing.T) {
	root := repoRoot(t)
	svgs, err := filepath.Glob(filepath.Join(root, "docs", "diagrams", "*.svg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range svgs {
		t.Errorf("%s: diagrams are .mermaid sources, embedded as ```mermaid blocks (no SVG)", s)
	}
	// Only the references that resolve into docs/diagrams count: a slide
	// deck may ship its own rendered images (docs/presentations/diagrams).
	ref := regexp.MustCompile(`[\w./-]*diagrams/[\w.-]+\.svg`)
	diagrams := filepath.Join("docs", "diagrams")
	for _, f := range docFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for i, l := range strings.Split(string(data), "\n") {
			for _, m := range ref.FindAllString(l, -1) {
				target := filepath.Join(filepath.Dir(f), filepath.FromSlash(m))
				if strings.HasPrefix(m, "/") {
					target = filepath.FromSlash(strings.TrimPrefix(m, "/"))
				}
				if filepath.Dir(target) == diagrams {
					t.Errorf("%s:%d: reference to %s (use the .mermaid source or a ```mermaid block)", f, i+1, m)
				}
			}
		}
	}
}

// A block followed by « Source du diagramme » / "Diagram source" is a copy
// of that source, and every .mermaid linked from the docs exists.
func TestMermaidCopiesMatchSources(t *testing.T) {
	root := repoRoot(t)
	for _, b := range allBlocks(t, root) {
		if b.source == "" {
			continue
		}
		p := filepath.Join(root, filepath.Dir(b.file), b.source)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s:%d: diagram source %s: %v", b.file, b.line, b.source, err)
			continue
		}
		if normalize(string(data)) != normalize(b.body) {
			t.Errorf("%s:%d: the block differs from its source %s (copy the source, or say « Diagramme détaillé » / \"Detailed diagram\" for a variant)", b.file, b.line, b.source)
		}
	}
	link := regexp.MustCompile(`\]\(([^)\s]+\.mermaid)\)`)
	for _, f := range docFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range link.FindAllStringSubmatch(string(data), -1) {
			if _, err := os.Stat(filepath.Join(root, filepath.Dir(f), m[1])); err != nil {
				t.Errorf("%s: link to a missing diagram %s", f, m[1])
			}
		}
	}
}

// Every Mermaid block of the docs, and every source in docs/diagrams,
// renders without error (needs mmdc, the Mermaid CLI: npm i -g @mermaid-js/mermaid-cli).
func TestMermaidRenders(t *testing.T) {
	mmdc, err := exec.LookPath("mmdc")
	if err != nil {
		t.Skip("mmdc is not installed (npm i -g @mermaid-js/mermaid-cli)")
	}
	if testing.Short() {
		t.Skip("renders every diagram with a headless browser")
	}
	root := repoRoot(t)
	type diagram struct{ where, body string }
	var diagrams []diagram
	seen := map[string]bool{}
	add := func(where, body string) {
		if n := normalize(body); !seen[n] {
			seen[n] = true
			diagrams = append(diagrams, diagram{where, n})
		}
	}
	for _, b := range allBlocks(t, root) {
		add(fmt.Sprintf("%s:%d", b.file, b.line), b.body)
	}
	sources, err := filepath.Glob(filepath.Join(root, "docs", "diagrams", "*.mermaid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sources {
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, s)
		add(rel, string(data))
	}
	if len(diagrams) == 0 {
		t.Fatal("no Mermaid diagram found in the docs")
	}

	dir := t.TempDir()
	// A render that times out says so (errTimeout): on a loaded machine the
	// browser is slow, which is not a broken diagram.
	run := func(in, out string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, mmdc, "-q", "-i", in, "-o", out)
		setProcessGroup(cmd)
		cmd.WaitDelay = 10 * time.Second
		var log bytes.Buffer
		cmd.Stdout, cmd.Stderr = &log, &log
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("%w (%v): %s", errTimeout, err, firstLines(log.String(), 5))
			}
			return fmt.Errorf("%w: %s", err, firstLines(log.String(), 5))
		}
		return nil
	}
	render := func(name, body string) error {
		in := filepath.Join(dir, name+".mmd")
		if err := os.WriteFile(in, []byte(body+"\n"), 0o600); err != nil {
			return err
		}
		return run(in, filepath.Join(dir, name+".svg"))
	}
	// mmdc itself must work (headless browser installed) before blaming a diagram.
	if err := render("probe", "flowchart LR\n  A --> B"); err != nil {
		t.Skipf("mmdc cannot render a trivial diagram (browser missing?): %v", err)
	}

	// Every diagram at once (one browser); one by one only to name the culprits.
	var all strings.Builder
	for _, d := range diagrams {
		all.WriteString("```mermaid\n" + d.body + "\n```\n\n")
	}
	batch := filepath.Join(dir, "all.md")
	if err := os.WriteFile(batch, []byte(all.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	batchErr := run(batch, filepath.Join(dir, "all.out.md"))
	if batchErr == nil {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	errs := make([]error, len(diagrams))
	for i, d := range diagrams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			errs[i] = render(fmt.Sprintf("d%03d", i), d.body)
		}()
	}
	wg.Wait()
	// A diagram that timed out among four browsers gets one more try, alone.
	for i, err := range errs {
		if errors.Is(err, errTimeout) {
			errs[i] = render(fmt.Sprintf("r%03d", i), diagrams[i].body)
		}
	}
	failed := false
	for i, err := range errs {
		if err != nil {
			failed = true
			t.Errorf("%s: the Mermaid diagram does not render: %v", diagrams[i].where, err)
		}
	}
	switch {
	case failed:
	case errors.Is(batchErr, errTimeout):
		t.Logf("all the diagrams at once timed out; they render one by one: %v", batchErr)
	default:
		t.Errorf("the diagrams render one by one but not together (mmdc on all the blocks failed): %v", batchErr)
	}
}

// errTimeout marks a render stopped by its time limit.
var errTimeout = errors.New("mmdc timed out")

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
