package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// Output of the commands: a command writes to the writers of its
// cobra command (cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.InOrStdin()) or to
// those of the app (a.IO.Out, a.IO.ErrOut), through internal/cliout for what
// has a shape (table, JSON, status message). That is what lets a test read
// the output and lets oh run in the process of a caller. The production
// files of cmd/ therefore name neither os.Stdout nor os.Stderr, and call
// neither fmt.Print, fmt.Printf nor fmt.Println.
//
// The uses that remain are listed below, per file and per kind, with their
// count and the reason: the standard streams handed to a subprocess, the
// questions put to the terminal (is it one, how big), and the places where
// there is no command to ask (the logger of the process, the last error of
// Execute). The list only shrinks: a new use fails the test, and so does a
// count that went down until the line is updated or removed.

type outputUse struct {
	count  int
	reason string
}

var cmdOutputExceptions = map[string]outputUse{
	"cmd/help.go os.Stdout":             {2, "Stat to know whether stdout is a terminal, and the stdout of the pager subprocess"},
	"cmd/help.go os.Stderr":             {1, "the stderr of the pager subprocess"},
	"cmd/init_steps.go os.Stdout":       {1, "the height of the terminal (term.GetSize needs the file descriptor)"},
	"cmd/misc.go os.Stdout":             {1, "the bd subprocess is given the streams of oh"},
	"cmd/misc.go os.Stderr":             {1, "the bd subprocess is given the streams of oh"},
	"cmd/run_request.go os.Stdout":      {1, "the client of the tool takes over the terminal"},
	"cmd/run_request.go os.Stderr":      {1, "the client of the tool takes over the terminal"},
	"cmd/workflow_teamcli.go os.Stdout": {1, "the editor started for a draft takes over the terminal"},
	"cmd/workflow_teamcli.go os.Stderr": {1, "the editor started for a draft takes over the terminal"},
	"cmd/root.go os.Stderr":             {5, "the logger of the process (slog, 2), and Execute and MustApp, which run where there is no command to ask (panic, last error, app not initialized)"},
	"cmd/config_notices.go os.Stderr":   {1, "the notice handler is called by config.Load, outside of any command"},
	"cmd/config.go os.Stderr":           {1, "the setter of a key of the field table (Set func) has no command"},
	"cmd/start_helpers.go os.Stderr":    {1, "errWriter: the error stream of the process when there is no app yet"},
}

// outputUses returns, for a source file, what the guard refuses: the uses of
// os.Stdout and os.Stderr (as a value, a call argument, a field…) and the
// calls of fmt.Print, fmt.Printf and fmt.Println, by kind.
func outputUses(t *testing.T, fset *token.FileSet, path string) map[string][]token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := map[string][]token.Position{}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case pkg.Name == "os" && (sel.Sel.Name == "Stdout" || sel.Sel.Name == "Stderr"):
			kind := "os." + sel.Sel.Name
			out[kind] = append(out[kind], fset.Position(sel.Pos()))
		case pkg.Name == "fmt" && (sel.Sel.Name == "Print" || sel.Sel.Name == "Printf" || sel.Sel.Name == "Println"):
			out["fmt.Print*"] = append(out["fmt.Print*"], fset.Position(sel.Pos()))
		}
		return true
	})
	return out
}

func TestCommandsWriteToTheirWriters(t *testing.T) {
	fset := token.NewFileSet()
	scanned := 0
	found := map[string]int{}
	for _, f := range goFiles(t) {
		dir, _, ok := strings.Cut(f.rel, "/")
		if !ok || dir != "cmd" || strings.Count(f.rel, "/") != 1 || strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		scanned++
		for kind, positions := range outputUses(t, fset, f.path) {
			key := f.rel + " " + kind
			found[key] = len(positions)
			want := cmdOutputExceptions[key].count
			if len(positions) <= want {
				continue
			}
			for _, pos := range positions {
				t.Errorf("%s:%d: %s in a command — write to cmd.OutOrStdout() / cmd.ErrOrStderr() (or a.IO.Out / a.IO.ErrOut), through cliout for a table, a JSON document or a status message; a use that cannot is listed in cmdOutputExceptions with its reason", f.rel, pos.Line, kind)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no Go file found under cmd/: the guard no longer checks anything")
	}
	var keys []string
	for key := range cmdOutputExceptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		want := cmdOutputExceptions[key].count
		if got := found[key]; got < want {
			t.Errorf("%s: %d use(s) listed in cmdOutputExceptions (%s), %d left — lower the count or remove the line", key, want, cmdOutputExceptions[key].reason, got)
		}
	}
}

// The check sees the streams and the prints wherever they are, and nothing
// else (a comment, a string, another package called os).
func TestOutputUsesDetection(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import (
	"fmt"
	"os"
)

// os.Stdout and fmt.Println are named in this comment
func f(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprintln(w, "os.Stdout fmt.Println(x)")
	fmt.Println("x")
	fmt.Printf("%d", 1)
	fmt.Print("x")
	fmt.Sprint("x")
	fmt.Fprintf(os.Stdout, "x")
	g(os.Stderr)
	_ = os.Stdin
	c := struct{ Stdout interface{} }{}
	c.Stdout = os.Stdout
}

func g(interface{}) {}
`
	path := dir + "/p.go"
	if err := writeFile(path, src); err != nil {
		t.Fatal(err)
	}
	got := outputUses(t, token.NewFileSet(), path)
	counts := map[string]string{}
	for kind, positions := range got {
		counts[kind] = fmt.Sprint(len(positions))
	}
	want := map[string]string{"fmt.Print*": "3", "os.Stdout": "2", "os.Stderr": "1"}
	if fmt.Sprint(counts) != fmt.Sprint(want) {
		t.Fatalf("found %v, want %v", counts, want)
	}
}
