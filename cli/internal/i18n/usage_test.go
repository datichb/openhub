package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// keyUse is a literal key passed to i18n.T or i18n.Tf in the CLI sources.
type keyUse struct {
	key  string
	pos  string
	args int // Tf arguments after the key; -1 for T or a spread call
}

// literalKeyUses parses every non-test Go file under root and returns the
// i18n.T / i18n.Tf calls whose key is a string literal (dynamic keys built
// by concatenation are skipped).
func literalKeyUses(t *testing.T, root string) []keyUse {
	t.Helper()
	var uses []keyUse
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if name := d.Name(); name == "testdata" || name == "hubcontent" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
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
			if !ok || pkg.Name != "i18n" || (sel.Sel.Name != "T" && sel.Sel.Name != "Tf") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			args := -1
			if sel.Sel.Name == "Tf" && !call.Ellipsis.IsValid() {
				args = len(call.Args) - 1
			}
			uses = append(uses, keyUse{key: key, pos: fset.Position(lit.Pos()).String(), args: args})
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return uses
}

// TestLiteralKeysExist fails when the code uses a key missing from the
// locales (it would be displayed raw), or passes Tf a number of arguments
// that differs from the format verbs of the message (%!(EXTRA …) or
// %!s(MISSING)).
func TestLiteralKeysExist(t *testing.T) {
	data, err := localeFS.ReadFile("locales/en.json")
	require.NoError(t, err)
	var en map[string]string
	require.NoError(t, json.Unmarshal(data, &en))

	uses := literalKeyUses(t, filepath.Join("..", ".."))
	require.NotEmpty(t, uses, "no i18n call found: wrong source root?")
	for _, u := range uses {
		msg, ok := en[u.key]
		if !ok {
			t.Errorf("%s: key %q is missing from the locales", u.pos, u.key)
			continue
		}
		if u.args < 0 {
			continue
		}
		if verbs := formatArgCount(msg); verbs != u.args {
			t.Errorf("%s: key %q has %d format verb(s), Tf receives %d argument(s)", u.pos, u.key, verbs, u.args)
		}
	}
}

var argVerbRe = regexp.MustCompile(`%(?:\[(\d+)\])?[+\-#0 ]*(?:\d+)?(?:\.\d+)?[sdqvwfgetxXoUpTbn%]`)

// formatArgCount is the number of arguments a message consumes, following
// fmt: verbs take the next argument, %[n]s jumps to argument n.
func formatArgCount(msg string) int {
	next, used := 0, 0
	for _, m := range argVerbRe.FindAllStringSubmatch(msg, -1) {
		if m[0] == "%%" {
			continue
		}
		if m[1] != "" {
			n, _ := strconv.Atoi(m[1])
			next = n - 1
		}
		next++
		if next > used {
			used = next
		}
	}
	return used
}

func TestFormatArgCount(t *testing.T) {
	for msg, want := range map[string]int{
		"plain":                   0,
		"100%% sure":              0,
		"%s and %d":               2,
		"%[1]s · %[1]s":           1,
		"%s then %s, again %[1]s": 2,
		"%.1f%% of %s":            2,
	} {
		if got := formatArgCount(msg); got != want {
			t.Errorf("formatArgCount(%q) = %d, want %d", msg, got, want)
		}
	}
}
