package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// The help of the commands is not written in the code. The text of
// a command (Short, Long, Example) and of an option (its usage) is replaced,
// when the help is shown, by the one of the locales under the conventional key
// (cmd.<command path>.short|long|example, cmd.<command path>.flags.<name>), so
// a text written next to the command never appeared: it was dead, and it went
// out of step with the locale (an option listed in one and not in the other).
// The text is added to fr.json and en.json under that key, and the code
// leaves it empty. A text built with i18n.T is not a literal and stays
// allowed.

// helpFlagFuncs maps the methods that define an option to the index of their
// usage argument.
var helpFlagFuncs = map[string]int{
	"String": 2, "StringP": 3, "Bool": 2, "BoolP": 3, "Int": 2, "IntP": 3, "Int64": 2, "Int64P": 3,
	"Float64": 2, "Float64P": 3, "Duration": 2, "DurationP": 3, "Uint": 2, "UintP": 3,
	"StringSlice": 2, "StringSliceP": 3, "StringArray": 2, "StringArrayP": 3, "StringToString": 2, "StringToStringP": 3,
	"IntSlice":  2,
	"StringVar": 3, "StringVarP": 4, "BoolVar": 3, "BoolVarP": 4, "IntVar": 3, "IntVarP": 4,
	"DurationVar": 3, "DurationVarP": 4, "StringSliceVar": 3, "StringSliceVarP": 4,
	"Float64Var": 3, "Float64VarP": 4, "Int64Var": 3, "Int64VarP": 4, "StringArrayVar": 3, "StringArrayVarP": 4,
	"Var": 3, "VarP": 4,
}

// isStringLiteral reports whether e is a string written in the code: a
// literal, or literals joined by +.
func isStringLiteral(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Kind == token.STRING
	case *ast.BinaryExpr:
		return v.Op == token.ADD && isStringLiteral(v.X) && isStringLiteral(v.Y)
	}
	return false
}

func isEmptyLiteral(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

// helpLiterals returns the position of every help text written in a Go source
// file: the Short, Long and Example of a cobra.Command literal, and the usage
// of an option defined with a literal name or a variable.
func helpLiterals(t *testing.T, fset *token.FileSet, path string) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			sel, ok := n.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Command" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "cobra" {
				return true
			}
			for _, el := range n.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok && (key.Name == "Short" || key.Name == "Long" || key.Name == "Example") &&
					isStringLiteral(kv.Value) && !isEmptyLiteral(kv.Value) {
					out = append(out, fset.Position(kv.Pos()))
				}
			}
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			idx, ok := helpFlagFuncs[sel.Sel.Name]
			if !ok || len(n.Args) <= idx {
				return true
			}
			if _, named := n.Args[0].(*ast.BasicLit); !named && !strings.HasSuffix(sel.Sel.Name, "Var") && !strings.HasSuffix(sel.Sel.Name, "VarP") {
				return true
			}
			if isStringLiteral(n.Args[idx]) && !isEmptyLiteral(n.Args[idx]) {
				out = append(out, fset.Position(n.Args[idx].Pos()))
			}
		}
		return true
	})
	return out
}

func TestHelpIsNotInTheCode(t *testing.T) {
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range goFiles(t) {
		if !strings.HasPrefix(f.rel, "cmd/") || strings.Count(f.rel, "/") != 1 || strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		scanned++
		for _, pos := range helpLiterals(t, fset, f.path) {
			t.Errorf("%s:%d: help text written in the code — it is never shown (the locales hold it under cmd.<command path>.short|long|flags.<name>): put it in fr.json and en.json and leave it empty here, or build it with i18n.T", f.rel, pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("no Go file found under cmd/: the guard no longer checks anything")
	}
}

// The check sees a text written in the definition of a command or of an
// option, and nothing else (an empty text, i18n.T, a literal that is no help).
func TestHelpLiteralDetection(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import "github.com/spf13/cobra"

type other struct{ Short, Long string }

func f(c *cobra.Command, name string) {
	_ = &cobra.Command{
		Use:     "x",
		Short:   "written",
		Long:    "written " + "twice",
		Example: ` + "`written`" + `,
	}
	_ = &cobra.Command{Use: "y", Short: i18n.T("k"), Long: ""}
	_ = other{Short: "not a command", Long: "not a command"}
	f := c.Flags()
	f.Bool("a", false, "written")
	f.StringP("b", "b", "", "written")
	f.StringVar(&name, "c", "", "written")
	f.String("d", "", i18n.T("k"))
	f.String("e", "", "")
	c.Flags().Bool("f", false, "written")
	_ = f.Lookup("g")
}
`
	path := dir + "/p.go"
	if err := writeFile(path, src); err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, p := range helpLiterals(t, token.NewFileSet(), path) {
		lines = append(lines, p.Line)
	}
	sort.Ints(lines)
	want := []int{10, 11, 12, 17, 18, 19, 22}
	if len(lines) != len(want) {
		t.Fatalf("help texts at lines %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("help texts at lines %v, want %v", lines, want)
		}
	}
}
