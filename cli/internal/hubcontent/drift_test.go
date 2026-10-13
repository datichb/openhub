package hubcontent

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// contentDirs are the hub directories copied into hub/ (make sync-content).
var contentDirs = []string{"agents", "skills", "permissions", "workflows"}

// In the development repository, the embedded copy is the sources at the
// repository root: a stale copy ships (and tests) content that no longer
// exists. Release builds copy the sources first (GoReleaser before-hook).
func TestEmbeddedContentInSync(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range contentDirs {
		if st, err := os.Stat(filepath.Join(root, dir)); err != nil || !st.IsDir() {
			t.Skipf("no hub sources at %s (not the development repository)", root)
		}
	}
	var diffs []string
	for _, dir := range contentDirs {
		src, err := readTree(os.DirFS(filepath.Join(root, dir)), ".")
		if err != nil {
			t.Fatal(err)
		}
		emb, err := readTree(embedded, "hub/"+dir)
		if err != nil {
			t.Fatal(err)
		}
		diffs = append(diffs, treeDiff(dir, src, emb)...)
	}
	if len(diffs) > 0 {
		const shown = 20
		more := ""
		if len(diffs) > shown {
			more = "\n  …"
			diffs = diffs[:shown]
		}
		t.Fatalf("the embedded hub content differs from the sources: run `make sync-content` in cli/\n  %s%s", strings.Join(diffs, "\n  "), more)
	}
}

func TestTreeDiff(t *testing.T) {
	src := map[string][]byte{"a.md": []byte("a"), "b.md": []byte("b"), "c.md": []byte("c")}
	emb := map[string][]byte{"a.md": []byte("a"), "b.md": []byte("old"), "d.md": []byte("d")}
	got := treeDiff("skills", src, emb)
	want := []string{"skills/b.md: changed", "skills/c.md: missing from the copy", "skills/d.md: no longer in the sources"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("treeDiff = %q, want %q", got, want)
	}
	if d := treeDiff("skills", src, src); len(d) != 0 {
		t.Fatalf("identical trees: %q", d)
	}
}

// readTree reads every regular file under dir, keyed by its path relative to dir.
func readTree(fsys fs.FS, dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		rel := p
		if dir != "." {
			rel = strings.TrimPrefix(p, dir+"/")
		}
		files[rel] = data
		return nil
	})
	return files, err
}

func treeDiff(dir string, src, emb map[string][]byte) []string {
	var diffs []string
	for p, data := range src {
		switch e, ok := emb[p]; {
		case !ok:
			diffs = append(diffs, dir+"/"+p+": missing from the copy")
		case !bytes.Equal(e, data):
			diffs = append(diffs, dir+"/"+p+": changed")
		}
	}
	for p := range emb {
		if _, ok := src[p]; !ok {
			diffs = append(diffs, dir+"/"+p+": no longer in the sources")
		}
	}
	sort.Strings(diffs)
	return diffs
}
