package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/datichb/openhub/cli/internal/domain"
)

// A12 follow-up: the user messages of oh config, oh config model and
// oh review --publish come from the locales; fmt.Errorf only wraps a
// translated message ("%s: %w"), errors.New never takes a literal.
func TestTranslatedErrorsInConfigAndReview(t *testing.T) {
	fset := token.NewFileSet()
	for _, file := range []string{"config.go", "config_model.go", "config_fields_v5.go", "audit_review_debug.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, _ := strconv.Unquote(lit.Value)
			switch {
			case pkg.Name == "errors" && sel.Sel.Name == "New":
				t.Errorf("%s: errors.New(%q): use i18n", fset.Position(lit.Pos()), v)
			case pkg.Name == "fmt" && sel.Sel.Name == "Errorf" && v != "%s: %w" && v != "%s" && v != "%w: %s":
				t.Errorf("%s: fmt.Errorf(%q): use i18n", fset.Position(lit.Pos()), v)
			}
			return true
		})
	}
}

// A12 follow-up: an unknown project in oh config model says so.
func TestProjectLoadErrorTranslated(t *testing.T) {
	useLocale(t, "fr")
	if err := projectLoadError("nope", errNotFoundForTest()); err.Error() != "projet inconnu : nope (oh project list)" {
		t.Fatalf("err = %v", err)
	}
}

func errNotFoundForTest() error { return fmt.Errorf("project nope: %w", domain.ErrNotFound) }
