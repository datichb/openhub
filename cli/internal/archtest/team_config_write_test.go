package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The config.toml of a team is shared: a member who saves a copy read earlier
// overwrites what a colleague changed meanwhile. Every edit goes through
// Repo.UpdateConfig / UpdateConfigFrom, which read the file again inside the
// transaction; Repo.SaveConfig (the whole file, from a copy) stays for the tests
// and for nothing else in the code of oh.
func TestNoCodeSavesAWholeTeamConfig(t *testing.T) {
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") || strings.HasPrefix(f.rel, "internal/teamstate/") {
			continue
		}
		scanned++
		for _, pos := range selectorCalls(t, fset, f.path, nil, "SaveConfig") {
			t.Errorf("%s:%d: SaveConfig replaces the whole config.toml from a copy; use UpdateConfig (patch on the fresh version) or UpdateConfigFrom (base → edited, conflict detected)", f.rel, pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned")
	}
}

func TestSaveConfigDetector(t *testing.T) {
	const src = `package x

// repo.SaveConfig in a comment is no call.
func f(repo interface{ SaveConfig() }) { repo.SaveConfig() }
`
	got := selectorCalls(t, token.NewFileSet(), "x.go", src, "SaveConfig")
	if len(got) != 1 || got[0].Line != 4 {
		t.Fatalf("want one call at line 4, got %v", got)
	}
}

// selectorCalls returns the position of every call x.<name>(...) in the source.
func selectorCalls(t *testing.T, fset *token.FileSet, name string, src any, method string) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == method {
				out = append(out, fset.Position(sel.Pos()))
			}
		}
		return true
	})
	return out
}
