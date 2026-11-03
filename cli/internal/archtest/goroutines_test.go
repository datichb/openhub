package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
// cmd/ is guarded as a whole since v5.1 (A7): the daemon of a remote job
// (cmd/runner.go), ohd (cmd/daemon*.go), the end of the claims (cmd/session_claims.go)
// and also the tview interface of the TUI (`safego.Go(func() {
// app.QueueUpdateDraw(...) })`, spinners, watchers), the web server, the
// heartbeat of an attach and the fan-out driver of an evaluation. A timer
// is safego.AfterFunc.
//
// internal/tui/v2/shell joins in v5.1 (A10): the click handlers of the
// lists, the copy toast of the selection, the dismissal timer of the toasts
// and the rotation of the notifications file ran in bare goroutines.
var guardedDirs = []string{
	"internal/daemon/",
	"internal/credproxy/",
	"internal/runsvc/",
	"internal/gateway/",
	"cmd/",
	"internal/tui/v2/shell/",
}

// unguardedFiles are the files of a guarded directory that still hold a bare
// go statement, with the reason. The list only shrinks: a file that has none
// left must be removed from it (TestUnguardedFilesStillNeedIt).
var unguardedFiles = map[string]string{
	"cmd/provider.go": "the steps of the Bedrock assistant, which the inline wizard replaces (A8)",
}

// bareGoStatements returns the position of every go statement of a Go
// source file, found by parsing it (a comment or a string saying "go func"
// is no statement), and of every time.AfterFunc call, whose function also
// runs in a goroutine of its own.
func bareGoStatements(t *testing.T, fset *token.FileSet, path string) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GoStmt:
			out = append(out, fset.Position(n.Go))
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "AfterFunc" {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" {
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
		if _, ok := unguardedFiles[f.rel]; ok {
			continue
		}
		for _, dir := range guardedDirs {
			if !strings.HasPrefix(f.rel, dir) {
				continue
			}
			scanned[dir]++
			for _, pos := range bareGoStatements(t, fset, f.path) {
				t.Errorf("%s:%d: bare go statement or time.AfterFunc — use safego.Go (a panic here would end the daemon; audit v5 A-02), or safego.Supervise for a loop the process needs", f.rel, pos.Line)
			}
		}
	}
	for _, dir := range guardedDirs {
		if scanned[dir] == 0 {
			t.Errorf("no Go file found under %s: the guard no longer checks anything, update guardedDirs", dir)
		}
	}
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

// The check sees a go statement wherever it is, and nothing else.
func TestBareGoDetection(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import (
	"fmt"
	"time"
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
	want := []int{11, 13, 15, 16}
	if len(lines) != len(want) {
		t.Fatalf("go statements at lines %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("go statements at lines %v, want %v", lines, want)
		}
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o600) }
