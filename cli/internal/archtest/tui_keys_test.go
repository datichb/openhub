package archtest

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Key guard (v5.1, A12): the keys of the TUI go through internal/tui/v2/keys.
// A view or a widget asks keys.Delete.Matches(event) or the keys.Own action of
// its own letter; it does not read the letter of the event. The production
// code of internal/tui/** (keys/ itself excepted, it is where the letters are
// bound) and of cmd/tui_*.go is parsed, and two things are counted per file:
//
//	a switch on the rune of an event, with a case on a rune literal
//	    switch ev.Rune() { case 'j': … }
//	a comparison of the rune of an event with a rune literal
//	    ev.Rune() == ' '
//
// A rune read into a variable first (r := ev.Rune()) counts the same. The
// keys that the code reads this way today are listed in
// testdata/tui_keys_exceptions.txt, per file, with their count. The list only
// shrinks, as the one of the style guard does; it is empty since A12.

const tuiKeysExceptionsFile = "testdata/tui_keys_exceptions.txt"

const tuiKeysAdvice = "ask a key of the package keys (keys.Delete.Matches(event), keys.Toggle…) or declare the letter of the view with keys.Own and use its Matches; do not read the letter of the event"

// tuiKeyFinding is one rune of an event read in a file.
type tuiKeyFinding struct {
	line int
	text string
}

// isRuneCall tells whether e is x.Rune() (the rune of a tcell event).
func isRuneCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Rune"
}

func isCharLit(e ast.Expr) (string, bool) {
	l, ok := e.(*ast.BasicLit)
	if !ok || l.Kind != token.CHAR {
		return "", false
	}
	return l.Value, true
}

// tuiKeyFindings parses a Go file and returns the runes of events that it
// reads.
func tuiKeyFindings(fset *token.FileSet, path string, src any) ([]tuiKeyFinding, error) {
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	// The names that hold the rune of an event: r := ev.Rune().
	held := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) == len(n.Rhs) {
				for i, r := range n.Rhs {
					if id, ok := n.Lhs[i].(*ast.Ident); ok && isRuneCall(r) {
						held[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i, v := range n.Values {
					if isRuneCall(v) {
						held[n.Names[i].Name] = true
					}
				}
			}
		}
		return true
	})
	isEventRune := func(e ast.Expr) bool {
		if id, ok := e.(*ast.Ident); ok {
			return held[id.Name]
		}
		return isRuneCall(e)
	}

	var out []tuiKeyFinding
	add := func(pos token.Pos, text string) {
		out = append(out, tuiKeyFinding{fset.Position(pos).Line, text})
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			if n.Tag == nil || !isEventRune(n.Tag) {
				return true
			}
			for _, c := range n.Body.List {
				for _, e := range c.(*ast.CaseClause).List {
					if v, ok := isCharLit(e); ok {
						add(e.Pos(), "case "+v)
					}
				}
			}
		case *ast.BinaryExpr:
			if n.Op != token.EQL && n.Op != token.NEQ {
				return true
			}
			if v, ok := isCharLit(n.Y); ok && isEventRune(n.X) {
				add(n.Pos(), n.Op.String()+" "+v)
			} else if v, ok := isCharLit(n.X); ok && isEventRune(n.Y) {
				add(n.Pos(), v+" "+n.Op.String())
			}
		}
		return true
	})
	return out, nil
}

// tuiKeysGuarded tells whether a production file is under the guard.
func tuiKeysGuarded(rel string) bool {
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	switch {
	case strings.HasPrefix(rel, "internal/tui/v2/keys/"):
		return false // where the letters are bound
	case strings.HasPrefix(rel, "internal/tui/"):
		return true
	case strings.HasPrefix(rel, "cmd/tui_") && strings.Count(rel, "/") == 1:
		return true
	}
	return false
}

func tuiKeysCounts(t *testing.T) (counts map[string]int, found map[string][]tuiKeyFinding, scanned int) {
	t.Helper()
	fset := token.NewFileSet()
	counts = map[string]int{}
	found = map[string][]tuiKeyFinding{}
	for _, f := range goFiles(t) {
		if !tuiKeysGuarded(f.rel) {
			continue
		}
		scanned++
		list, err := tuiKeyFindings(fset, f.path, nil)
		if err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		if len(list) > 0 {
			counts[f.rel] = len(list)
			found[f.rel] = list
		}
	}
	return counts, found, scanned
}

// readTUIKeysExceptions reads the frozen list: "<file> <count>", with # for
// comments.
func readTUIKeysExceptions(t *testing.T) map[string]int {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleRoot(t), "internal", "archtest", filepath.FromSlash(tuiKeysExceptionsFile)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]int{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("%s:%d: expected \"<file> <count>\", got %q", tuiKeysExceptionsFile, n, line)
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil || count < 1 {
			t.Fatalf("%s:%d: bad count %q", tuiKeysExceptionsFile, n, fields[1])
		}
		if _, dup := out[fields[0]]; dup {
			t.Fatalf("%s:%d: %q is listed twice", tuiKeysExceptionsFile, n, fields[0])
		}
		out[fields[0]] = count
	}
	return out
}

// TestTUIKeysGoThroughKeys: the TUI does not read the letter of an event; the
// ones that do are frozen and can only go down.
func TestTUIKeysGoThroughKeys(t *testing.T) {
	counts, found, scanned := tuiKeysCounts(t)
	frozen := readTUIKeysExceptions(t)
	if scanned == 0 {
		t.Fatal("the guard found no file under internal/tui and cmd/tui_*: it checks nothing any more")
	}

	files := map[string]bool{}
	for f := range counts {
		files[f] = true
	}
	for f := range frozen {
		files[f] = true
	}
	for _, file := range sortedKeys(files) {
		got, want := counts[file], frozen[file]
		switch {
		case got > want:
			var lines []string
			for _, fd := range found[file] {
				lines = append(lines, fmt.Sprintf("%s:%d %s", file, fd.line, fd.text))
			}
			t.Errorf("%s: %d key(s) read from the rune of an event, %d accepted — %s (the list must not grow)\n\t%s",
				file, got, want, tuiKeysAdvice, strings.Join(lines, "\n\t"))
		case got < want && got == 0:
			t.Errorf("%s: no key read from a rune left — remove the line \"%s %d\" from %s", file, file, want, tuiKeysExceptionsFile)
		case got < want:
			t.Errorf("%s: %d key(s) left, %d listed — write \"%s %d\" in %s", file, got, want, file, got, tuiKeysExceptionsFile)
		}
	}
}

// The list has no line for a file that no longer exists or is not guarded.
func TestTUIKeysExceptionsPointToGuardedFiles(t *testing.T) {
	for file := range readTUIKeysExceptions(t) {
		if !tuiKeysGuarded(file) {
			t.Errorf("%s is listed in %s but is not under the guard", file, tuiKeysExceptionsFile)
		}
		if _, err := os.Stat(filepath.Join(moduleRoot(t), filepath.FromSlash(file))); err != nil {
			t.Errorf("%s is listed in %s but does not exist", file, tuiKeysExceptionsFile)
		}
	}
}

// The detection sees the runes of events that are read, and only them.
func TestTUIKeysDetection(t *testing.T) {
	src := `package views

func f(ev *tcell.EventKey, event *tcell.EventKey, name string, c byte) {
	switch ev.Rune() {
	case 'j':
	case 'k', 'g':
	case other():
	}
	r := event.Rune()
	switch r {
	case ' ':
	}
	if ev.Rune() == 'x' || ev.Rune() != ' ' {
	}
	if '?' == event.Rune() {
	}
	switch s := event.Rune(); s {
	case '{':
	}

	// not keys
	switch name[0] {
	case 'a':
	}
	switch c {
	case 'b':
	}
	if ev.Rune() == cleanup.Rune() {
	}
	if ev.Key() == tcell.KeyRune {
	}
	switch ev.Key() {
	case tcell.KeyEnter:
	}
	for _, ch := range name {
		if ch == 'x' {
		}
	}
}
`
	got, err := tuiKeyFindings(token.NewFileSet(), "p.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, f := range got {
		texts = append(texts, f.text)
	}
	sort.Strings(texts)
	want := []string{"!= ' '", "== 'x'", "'?' ==", "case ' '", "case '{'", "case 'g'", "case 'j'", "case 'k'"}
	sort.Strings(want)
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Errorf("found %q, expected %q", texts, want)
	}
}
