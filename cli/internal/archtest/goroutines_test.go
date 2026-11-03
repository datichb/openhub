package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A-02 (audit v5): a panic in a goroutine nobody recovers ends the process.
// The daemon, its proxy, the launcher and the gateways run in the process of
// the user (on Windows the daemon lives inside oh, so a panic would also
// close the interface and leave the tool servers running): every goroutine
// of these packages goes through internal/safego, which recovers and logs the
// panic (safego.Go; safego.Supervise for a loop the process needs).

// guardedDirs are the directories (and their subdirectories) where a bare
// `go` statement is refused in non-test files. An entry that is not a
// directory names the files that start with it.
//
// The whole of cmd/ and internal/ is guarded (v5.2.1: the guard first covered
// the daemon, the proxy, the launcher, the gateways, cmd/ and the shell of the
// TUI; the views, the widgets, the router, the services, the adapters, the
// runtimes, teamstate, evalrun and mcp ran bare goroutines that a panic of an
// external stream, a callback or a store ended the process with, and
// under the TUI without a crash file). Every goroutine goes through
// internal/safego, which recovers the panic, logs it and writes the crash
// report. What the guard refuses, besides the `go` statement:
//
//   - time.AfterFunc, also imported under another name (safego.AfterFunc);
//   - the Go method of an errgroup.Group or of a sync.WaitGroup, unless its
//     argument is a call to safego (g.Go(safego.WrapErr(fn)),
//     wg.Go(safego.Wrap(fn))): the check reads the syntax, not the types, so
//     any other method named Go is refused too, and the argument decides.
var guardedDirs = []string{
	"cmd/",
	"internal/",
}

// unguardedDirs are directories of guardedDirs the guard does not read, with
// the reason.
var unguardedDirs = map[string]string{
	"internal/safego/": "the launchers themselves",
}

// unguardedFiles are the files of a guarded directory that still hold a bare
// go statement, with the reason. The list only shrinks: a file that has none
// left must be removed from it (TestUnguardedFilesStillNeedIt).
var unguardedFiles = map[string]string{}

const (
	safegoPath = "github.com/datichb/openhub/cli/internal/safego"
	timePath   = "time"
)

// importNames returns the names under which a file imports the package at
// path (its last element when it is not renamed).
func importNames(file *ast.File, path string) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != path {
			continue
		}
		switch {
		case imp.Name != nil:
			names[imp.Name.Name] = true
		default:
			names[path[strings.LastIndex(path, "/")+1:]] = true
		}
	}
	return names
}

// isSafegoCall reports whether e is a call of a function of safego.
func isSafegoCall(e ast.Expr, safegoNames map[string]bool) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && safegoNames[pkg.Name]
}

// bareGoStatements returns the position of every goroutine a Go source file
// starts without safego, found by parsing it (a comment or a string saying
// "go func" is no statement): the go statements, the time.AfterFunc calls,
// whose function also runs in a goroutine of its own, and the Go methods
// (errgroup, sync.WaitGroup) whose argument is not a call to safego.
func bareGoStatements(t *testing.T, fset *token.FileSet, path string) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	timeNames, safegoNames := importNames(file, timePath), importNames(file, safegoPath)
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GoStmt:
			out = append(out, fset.Position(n.Go))
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				break
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			switch {
			case sel.Sel.Name == "AfterFunc" && isIdent && timeNames[pkg.Name]:
				out = append(out, fset.Position(n.Pos()))
			case sel.Sel.Name == "Go" && (!isIdent || !safegoNames[pkg.Name]):
				if len(n.Args) != 1 || !isSafegoCall(n.Args[0], safegoNames) {
					out = append(out, fset.Position(n.Pos()))
				}
			}
		}
		return true
	})
	return out
}

func TestNoBareGoInCriticalPackages(t *testing.T) {
	fset := token.NewFileSet()
	scanned := map[string]int{}
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		if _, ok := unguardedFiles[f.rel]; ok || inDirs(f.rel, unguardedDirs) {
			continue
		}
		for _, dir := range guardedDirs {
			if !strings.HasPrefix(f.rel, dir) {
				continue
			}
			scanned[dir]++
			for _, pos := range bareGoStatements(t, fset, f.path) {
				t.Errorf("%s:%d: bare go statement, time.AfterFunc or Go method — use safego.Go, safego.AfterFunc, safego.Wrap/WrapErr (a panic here would end the process; audit v5 A-02), or safego.Supervise for a loop the process needs", f.rel, pos.Line)
			}
		}
	}
	for _, dir := range guardedDirs {
		if scanned[dir] == 0 {
			t.Errorf("no Go file found under %s: the guard no longer checks anything, update guardedDirs", dir)
		}
	}
}

func inDirs(rel string, dirs map[string]string) bool {
	for dir := range dirs {
		if strings.HasPrefix(rel, dir) {
			return true
		}
	}
	return false
}

// An unguarded file must still hold a go statement: when the last one goes,
// the file joins the guard.
func TestUnguardedFilesStillNeedIt(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, f := range goFiles(t) {
		reason, ok := unguardedFiles[f.rel]
		if !ok {
			continue
		}
		seen[f.rel] = true
		if len(bareGoStatements(t, fset, f.path)) == 0 {
			t.Errorf("%s has no bare go statement left (%s): remove it from unguardedFiles", f.rel, reason)
		}
	}
	for rel := range unguardedFiles {
		if !seen[rel] {
			t.Errorf("%s is not a Go file of the module: remove it from unguardedFiles", rel)
		}
	}
}

// The check sees a go statement wherever it is, a time.AfterFunc under any
// name of the package, and the Go method of a group unless safego wraps its
// function — and nothing else.
func TestBareGoDetection(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import (
	"fmt"
	"time"
	clock "time"

	"golang.org/x/sync/errgroup"

	"github.com/datichb/openhub/cli/internal/safego"
)

// go func() {} is a comment
func f() {
	fmt.Println("go func() {}()")
	go g()
	func() {
		go func() {}()
	}()
	defer func() { go g() }()
	time.AfterFunc(time.Second, g)
	clock.AfterFunc(time.Second, g)
	safego.AfterFunc(time.Second, g)
	safego.Go(g)
	var eg errgroup.Group
	eg.Go(func() error { return nil })
	eg.Go(safego.WrapErr(func() error { return nil }))
	var wg sync.WaitGroup
	wg.Go(g)
	wg.Go(safego.Wrap(g))
}

func g() {}
`
	path := dir + "/p.go"
	if err := writeFile(path, src); err != nil {
		t.Fatal(err)
	}
	got := bareGoStatements(t, token.NewFileSet(), path)
	var lines []int
	for _, p := range got {
		lines = append(lines, p.Line)
	}
	want := []int{16, 18, 20, 21, 22, 26, 29}
	if len(lines) != len(want) {
		t.Fatalf("go statements at lines %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("go statements at lines %v, want %v", lines, want)
		}
	}
}

// What the guard exempts exists: a renamed or deleted directory is removed
// from the list.
func TestUnguardedDirsExist(t *testing.T) {
	root := moduleRoot(t)
	for dir := range unguardedDirs {
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			t.Errorf("%s is not a directory of the module: remove it from unguardedDirs", dir)
		}
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o600) }
