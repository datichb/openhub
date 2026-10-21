package keys

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyConvention reads every hints line of the TUI (the French messages
// whose key holds « hint », and the fmt.Sprintf formats of the TUI code that
// join « key label » parts with « · ») and reports the parts that break the
// convention:
//   - a key of the convention used for another action (strict keys: g G / r
//     ? d x R y n; e, s, A and c may carry actions of a view);
//   - an action of the convention on another letter (« a ajouter » instead
//     of n, « r restaurer » instead of R, « D diff » instead of c).
//
// Any deviation fails: fix the hint (see docs/dev/tui-conventions.md).
func TestKeyConvention(t *testing.T) {
	fr := frMessages(t)
	var found []string
	for _, line := range hintLines(t, fr) {
		for _, d := range checkHintLine(line.text) {
			found = append(found, line.source+" | "+d)
		}
	}
	for _, d := range uniq(found) {
		t.Errorf("key outside the convention (fix it, see docs/dev/tui-conventions.md):\n\t%q", d)
	}
}

// Synonyms of the actions in the French hints (prefixes, lower case).
var synonyms = map[string][]string{
	"create":     {"créer", "ajouter", "nouveau", "nouvelle", "dupliquer"},
	"edit":       {"éditer", "modifier"},
	"delete":     {"supprimer"},
	"archive":    {"archiver", "classer"},
	"resume":     {"reprendre", "restaurer", "récupérer"},
	"compare":    {"comparer", "diff"},
	"accept":     {"valider", "accepter", "autoriser"},
	"refuse":     {"refuser", "rejeter"},
	"leave":      {"laisser"},
	"refresh":    {"rafraîchir", "recharger", "actualiser", "re-check", "revérifier"},
	"filter":     {"filtrer", "rechercher", "chercher"},
	"help":       {"aide"},
	"next_match": {"suivant"},
	"prev_match": {"précédent"},
	"first":      {"premier", "début"},
	"last":       {"dernier", "fin"},
}

// keyActions: the actions of each key of the convention (all contexts).
func keyActions() map[string][]string {
	out := map[string][]string{}
	for _, a := range All() {
		if r := a.Rune(); r != 0 {
			out[string(r)] = append(out[string(r)], a.ID)
		}
	}
	out["e"] = append(out["e"], "edit")
	return out
}

// strictKeys never carry another action than theirs.
var strictKeys = map[string]bool{"g": true, "G": true, "/": true, "r": true, "?": true, "d": true, "x": true, "R": true, "y": true, "n": true}

// ruleActions: the actions whose label must use their own key.
var ruleActions = []string{"create", "edit", "delete", "archive", "resume", "compare", "accept", "refuse", "leave", "refresh", "filter"}

func labelIs(label, action string) bool {
	for _, s := range synonyms[action] {
		if strings.HasPrefix(label, s) {
			return true
		}
	}
	return false
}

// checkHintLine returns the parts of a hints line that break the
// convention, as « part → reason ».
func checkHintLine(line string) []string {
	byKey := keyActions()
	var out []string
	for _, part := range strings.Split(line, " · ") {
		part = strings.TrimSpace(part)
		field, label, ok := strings.Cut(part, " ")
		if !ok {
			continue
		}
		ks := hintKeys(field)
		if ks == nil {
			continue
		}
		label = strings.ToLower(strings.TrimSpace(label))
		labels := strings.Split(label, "/")
		for i, k := range ks {
			l := label
			if len(labels) == len(ks) {
				l = labels[i]
			} else if len(ks) > 1 {
				continue // « y/n décider »: one label for several keys
			}
			if l == "" || strings.HasPrefix(l, unresolved) {
				continue
			}
			if strictKeys[k] {
				okLabel := false
				for _, id := range byKey[k] {
					okLabel = okLabel || labelIs(l, id)
				}
				if !okLabel {
					out = append(out, part+" → "+k+" est réservée ("+strings.Join(byKey[k], ", ")+")")
					continue
				}
			}
			for _, id := range ruleActions {
				if labelIs(l, id) && !containsStr(keysOfAction(id), k) {
					out = append(out, part+" → "+id+" se fait avec "+strings.Join(keysOfAction(id), "/"))
					break
				}
			}
		}
	}
	return out
}

func keysOfAction(id string) []string {
	var out []string
	for _, a := range All() {
		if a.ID == id && a.Rune() != 0 {
			out = append(out, string(a.Rune()))
		}
	}
	if id == "edit" {
		out = append(out, "e")
	}
	return out
}

// hintKeys parses the key field of a part ("d", "g/G", "/"); nil when it
// is not a letter key (Enter, Ctrl+S, Tab…).
func hintKeys(field string) []string {
	if field == "/" {
		return []string{"/"}
	}
	var out []string
	for _, p := range strings.Split(field, "/") {
		if len([]rune(p)) != 1 || !keyRune.MatchString(p) {
			return nil
		}
		out = append(out, p)
	}
	return out
}

var keyRune = regexp.MustCompile(`^[A-Za-z0-9?*]$`)

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func uniq(list []string) []string {
	sort.Strings(list)
	out := list[:0]
	for i, s := range list {
		if i == 0 || s != list[i-1] {
			out = append(out, s)
		}
	}
	return out
}

type hintLine struct{ source, text string }

func frMessages(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "i18n", "locales", "fr.json"))
	require.NoError(t, err)
	var fr map[string]string
	require.NoError(t, json.Unmarshal(data, &fr))
	return fr
}

// hintLines are the hints messages and the Sprintf formats of the TUI code
// (the i18n.T arguments resolved in French).
func hintLines(t *testing.T, fr map[string]string) []hintLine {
	t.Helper()
	var out []hintLine
	for k, v := range fr {
		if strings.HasPrefix(k, "tui.") && (strings.Contains(strings.ToLower(k), "hint") || strings.HasPrefix(k, "tui.keys.launch_recap")) {
			out = append(out, hintLine{source: "fr.json " + k, text: v})
		}
	}
	var files []string
	for _, dir := range []string{"../views", "../shell", "../widgets", "../layout", "../router"} {
		m, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		files = append(files, m...)
	}
	m, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "cmd", "tui*.go"))
	require.NoError(t, err)
	files = append(files, m...)
	require.NotEmpty(t, files)
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				name = recvName(fn.Recv.List[0].Type) + "." + name
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isCall(call, "fmt", "Sprintf") || len(call.Args) == 0 {
					return true
				}
				format, ok := stringLit(call.Args[0])
				if !ok || !strings.Contains(format, " · ") {
					return true
				}
				args := make([]string, 0, len(call.Args)-1)
				for _, a := range call.Args[1:] {
					args = append(args, resolveArg(a, fr))
				}
				out = append(out, hintLine{source: filepath.Base(path) + " " + name, text: applyFormat(format, args)})
				return true
			})
		}
	}
	return out
}

func recvName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvName(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return recvName(x.X)
	}
	return "?"
}

func isCall(call *ast.CallExpr, pkg, fn string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && sel.Sel.Name == fn
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func resolveArg(e ast.Expr, fr map[string]string) string {
	if s, ok := stringLit(e); ok {
		return s
	}
	if call, ok := e.(*ast.CallExpr); ok && isCall(call, "i18n", "T") && len(call.Args) == 1 {
		if k, ok := stringLit(call.Args[0]); ok {
			return fr[k]
		}
	}
	return unresolved
}

// unresolved stands for an argument the scan cannot resolve (never a key).
const unresolved = "…"

var verbRe = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

func applyFormat(format string, args []string) string {
	i := 0
	return verbRe.ReplaceAllStringFunc(format, func(v string) string {
		if v == "%%" {
			return "%"
		}
		if i >= len(args) {
			return unresolved
		}
		i++
		return args[i-1]
	})
}

func TestCheckHintLine(t *testing.T) {
	cases := map[string]int{
		"d supprimer · x archiver · R reprendre · c comparer": 0,
		"y valider · n refuser · e modifier · s laisser":      0,
		"y/n décider · a attacher · e enchaîner":              0,
		"g/G début/fin · n/N suivant/précédent · / filtrer":   0,
		"Enter éditer · Ctrl+S lancer · Esc retour":           0,
		"D supprimer":          1, // delete on D
		"r restaurer":          1, // r is refresh, restore is R
		"a ajouter":            1, // create is n
		"x cleanup":            1, // x never deletes
		"n renommer":           1, // n is reserved
		"D diff":               1, // compare is c
		"f filtrer":            1, // filter is /
		"c reprendre":          1, // resume is R
		"a … · r …":            0, // unresolved labels
		"… actifs":             0,
		"[[] /] onglets":       0,
		"h/l colonnes":         0,
		"y YAML brut":          1,
		"g récupérer":          1,
		"v valider":            1,
		"d reset · r re-check": 1,
	}
	for line, want := range cases {
		require.Len(t, checkHintLine(line), want, "%s: %v", line, checkHintLine(line))
	}
}
