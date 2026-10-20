package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
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

var templateKeyRe = regexp.MustCompile(`\{\{\s*trf?\s+"([^"]+)"`)

// sourceStringLiterals returns every string literal of the non-test Go files
// under root (keys are also passed through variables, tables, structs and
// returned as reasons: a literal equal to a key is a use of that key).
func sourceStringLiterals(t *testing.T, root string) map[string]bool {
	t.Helper()
	lits := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root {
				if name := d.Name(); name == "testdata" || name == "hubcontent" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
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
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					lits[s] = true
					// Keys in the text of a template: {{tr "help.usage.flags"}}.
					for _, m := range templateKeyRe.FindAllStringSubmatch(s, -1) {
						lits[m[1]] = true
					}
				}
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return lits
}

var keyShapeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_\-]+){2,}$`)

// dynamicKeyPrefix returns the constant start of a key built at run time:
// i18n.T("prefix." + x), i18n.T(fmt.Sprintf("prefix.%s", x)), or
// i18n.T(key + "_eta") with key := "prefix.name". "" when the literal does
// not look like the start of a key.
func dynamicKeyPrefix(lit string) string {
	if i := strings.Index(lit, "%"); i >= 0 {
		lit = lit[:i]
		if strings.Count(lit, ".") < 1 || len(lit) < 5 {
			return ""
		}
		return lit
	}
	if keyShapeRe.MatchString(lit) || (strings.HasSuffix(lit, ".") && strings.Count(lit, ".") >= 2) {
		return lit
	}
	return ""
}

// TestNoOrphanKeys fails when a key of en.json is used nowhere in the code: a
// message nobody shows is a translation to maintain for nothing (U-11). A key
// is used when a string literal of the code is that key, or when it starts
// with a literal of the code that looks like a key and that the code
// completes at run time (i18n.T("prefix." + x), i18n.T(key + "_eta")).
// The keys of the help of the commands (cmd.<command>.short|long|flags.*)
// are read by convention from the command tree (localizeCommands) and are
// not checked here.
func TestNoOrphanKeys(t *testing.T) {
	data, err := localeFS.ReadFile("locales/en.json")
	require.NoError(t, err)
	var en map[string]string
	require.NoError(t, json.Unmarshal(data, &en))

	lits := sourceStringLiterals(t, filepath.Join("..", ".."))
	var prefixes []string
	for lit := range lits {
		if p := dynamicKeyPrefix(lit); p != "" {
			prefixes = append(prefixes, p)
		}
	}
	helpKey := regexp.MustCompile(`^cmd\.[^ ]+\.(short|long|example|deprecated|flags\.[^.]+)$`)
	var orphans []string
	for key := range en {
		if lits[key] || helpKey.MatchString(key) {
			continue
		}
		used := false
		for _, p := range prefixes {
			if strings.HasPrefix(key, p) {
				used = true
				break
			}
		}
		if !used {
			orphans = append(orphans, key)
		}
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("%d key(s) of en.json are used nowhere in the code (remove them from en.json and fr.json):\n  %s",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}

func TestDynamicKeyPrefix(t *testing.T) {
	for lit, want := range map[string]string{
		"tui.doctor.container.":       "tui.doctor.container.",
		"cmd.mcp.%s.label":            "cmd.mcp.",
		"tui.launch.runtime.build":    "tui.launch.runtime.build",
		"plain text":                  "",
		"a.b":                         "",
		"https://example.com/a/b.c.d": "",
		"cmd.run":                     "",
	} {
		if got := dynamicKeyPrefix(lit); got != want {
			t.Errorf("dynamicKeyPrefix(%q) = %q, want %q", lit, got, want)
		}
	}
}
