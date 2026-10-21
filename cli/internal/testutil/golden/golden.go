// Package golden compares the output of a test with a golden file,
// testdata/<name>.golden in the directory of the package under test.
//
//	golden.Assert(t, "render", out) // testdata/render.golden
//
// The run with the flag -update (go test ./pkg -run TestRender -update)
// rewrites the golden file with the output instead of comparing; the diff of
// the golden file is then reviewed like code.
//
// The flag -update is declared here, once for all the packages that use
// golden. A test binary cannot declare two flags with the same name (the
// second declaration panics at start): a package that imports golden must not
// declare its own `update` flag (flag.Bool("update", …)); when a package is
// moved to golden, its flag and its helper are removed in the same change.
package golden

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

var update = flag.Bool("update", false, "rewrite the golden files with the output of the tests")

// Updating reports whether the run rewrites the golden files (-update). A test
// that keeps its golden files in step with something else (it removes the
// files of what no longer exists) acts on it.
func Updating() bool { return *update }

// Assert compares got with testdata/<name>.golden. The name may contain
// folders (testdata/<dir>/<name>.golden). A difference, or a missing file,
// fails the test (without stopping it) with the diff and the command that
// regenerates the file.
func Assert[T ~string | ~[]byte](t testing.TB, name string, got T) {
	t.Helper()
	if !filepath.IsLocal(name) {
		t.Errorf("golden: the name %q must be a relative path inside testdata", name)
		return
	}
	path := filepath.Join("testdata", name+".golden")
	data := []byte(got)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Errorf("golden: %v", err)
			return
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Errorf("golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("golden: cannot read %s: %v\nto create it: %s", path, err, regenerate(t))
		return
	}
	if string(want) == string(data) {
		return
	}
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(want)),
		B:        difflib.SplitLines(string(data)),
		FromFile: path + " (want)",
		ToFile:   "output (got)",
		Context:  3,
	})
	t.Errorf("golden: the output differs from %s:\n%s\nif the change is intended: %s", path, diff, regenerate(t))
}

// regenerate is the command that rewrites the golden files of the test.
func regenerate(t testing.TB) string {
	var run []string
	for _, part := range strings.Split(t.Name(), "/") {
		run = append(run, "^"+regexp.QuoteMeta(part)+"$")
	}
	return fmt.Sprintf("go test %s -run '%s' -update", packageArg(), strings.Join(run, "/"))
}

// packageArg is the package under test relative to its module (./internal/x),
// "." when the module is not found.
func packageArg() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for root := dir; ; root = filepath.Dir(root) {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			rel, err := filepath.Rel(root, dir)
			if err != nil || rel == "." {
				return "."
			}
			return "./" + filepath.ToSlash(rel)
		}
		if root == filepath.Dir(root) {
			return "."
		}
	}
}
