package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// U-11: no user message is written in the code. The guard parses every file
// of cmd/ and of the TUI views and fails on a prose literal given to a sink
// that shows text to the user: errors.New, fmt.Errorf, fmt.Fprint*/Print*,
// fmt.Sprintf, the prompts of the CLI (askInput, askYN, askSelect), the
// widgets of the TUI (SetTitle, SetText…) and the forms (Title, Description,
// NewOption…). Such text goes through i18n.T / i18n.Tf (locales/en.json and
// fr.json). The help of the commands (Short, Long, flag usages) is checked by
// TestEveryHelpIsTranslated.
//
// A literal that must not be translated is listed in literalExceptions with
// its reason; nothing else is allowed.

// proseRe matches a text made of words: two words of letters separated by
// spaces or punctuation.
var proseRe = regexp.MustCompile(`[A-Za-zÀ-ÿ]{2,}[ ,;:'’-]+[A-Za-zÀ-ÿ]{2,}`)

// literalSinkMethods are the methods (of huh forms and tview widgets) whose
// string arguments are shown to the user.
var literalSinkMethods = map[string]bool{
	"Title": true, "Description": true, "Placeholder": true, "Affirmative": true, "Negative": true,
	"NewOption": true, "SetTitle": true, "SetText": true, "SetLabel": true, "SetFooter": true, "ShowToast": true,
	"AddButton": true,
}

// wordRe matches a single word, which is a text for a button label.
var wordRe = regexp.MustCompile(`^[A-Za-zÀ-ÿ]{3,}$`)

// literalSinkFields are the struct fields (steps and fields of the forms) that
// hold text shown to the user.
var literalSinkFields = map[string]bool{"Label": true, "Title": true, "Desc": true, "Description": true, "Placeholder": true, "Hint": true, "Help": true}

// literalSinkFuncs are the prompt helpers of the CLI.
var literalSinkFuncs = map[string]bool{"askInput": true, "askYN": true, "askSelect": true}

// literalExceptions are the prose literals that stay in the code, with the
// reason. They are exact literals (any file of the guard).
var literalExceptions = map[string]string{
	// File names and technical identifiers.
	"oh-backup-%s.tar.gz":          "file name",
	"oh-history-%s.json":           "file name",
	"oh — OpenHub CLI %s":          "product name and version",
	"Google Slides":                "product name",
	"Amazon Bedrock":               "product name",
	"GitHub Copilot":               "product name",
	"exit status %d":               "text of an error that is never printed (ExitError), for errors.Is/As",
	"  %s%s%s  oh (OpenHub CLI)\n": "product name",
	// Text written into other systems, in a fixed language: git commit
	// messages, documents committed to the team-state, prompts for the agents.
	"patterns: add %s":                      "git commit message",
	"patterns: validate %s":                 "git commit message",
	"patterns: remove %s":                   "git commit message",
	"team: update member %s":                "git commit message",
	"team: add member %s":                   "git commit message",
	"takeover: enriched brief for %s/%s":    "git commit message",
	"tracker: discovery config for %s (%s)": "git commit message",
	"# Takeover Brief (enrichi): %s\n\n%s":  "document committed to the team-state",
	"[ai-delegated]%s %s — %s":              "ticket title written to the tracker",
	"review prête pour %s/%s":               "notification sent to the team (shared, fixed language)",
	"audit findings sur %s/%s":              "notification sent to the team (shared, fixed language)",
	"conflit de claim sur %s/%s":            "notification sent to the team (shared, fixed language)",
	"Travaille sur le ticket %s. Utilise `bd prime` pour le contexte et `bd ready` pour les tâches disponibles.": "prompt for the agent",
	// Protocols and markup.
	"event: ping\ndata: {\"status\":\"connected\"}\n\n": "server-sent event",
	// Output of the eval harness, read by developers (make test-eval).
	"eval pass:":                         "eval harness output",
	"eval pass: session":                 "eval harness output",
	"eval pass: wiki %s %s → %s %s\n":    "eval harness output",
	"eval pass: %s %s → %s %s\n":         "eval harness output",
	"more than %d decisions":             "eval harness output",
	"pass budget spent ($%.3f of $%.2f)": "eval harness output",
	"eval pass: stopping the session:":   "eval harness output",
	"eval pass: fan-out:":                "eval harness output",
	// Commands to type, shown as they are.
	"  %s bd init: %s\n":            "name of a command and its output",
	"%s brew uninstall: %v\n":       "name of a command and its output",
	"     oh repair --restore %s\n": "command to type",
	"     oh import %s\n":           "command to type",
	// Programming errors of the TUI (a developer fixes them, the user never
	// meets them) and sentinel errors compared with errors.Is.
	"action %s on the key of %s":                 "programming error",
	"actions %s and %s on the same key %q":       "programming error",
	"action %s without Run":                      "programming error",
	"global action %s without Run":               "programming error",
	"provider credentials missing":               "sentinel error (errors.Is)",
	"transfer of the existing session failed":    "sentinel error (errors.Is)",
	"checkpoint verification must be overridden": "sentinel error (errors.Is)",
}

// literalHit is a prose literal given to a sink.
type literalHit struct {
	pos, text string
}

// userLiterals lists the prose literals of the sinks in the non-test files of dir.
func userLiterals(t *testing.T, dir string) []literalHit {
	t.Helper()
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go file in %s: %v", dir, err)
	}
	var hits []literalHit
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		button := false // AddButton(i18n.T("cmd.button.next")): a single word is a text too
		var visit func(args []ast.Expr)
		visit = func(args []ast.Expr) {
			for _, a := range args {
				switch x := a.(type) {
				case *ast.BasicLit:
					if x.Kind != token.STRING {
						continue
					}
					v, _ := strconv.Unquote(x.Value)
					if (proseRe.MatchString(v) || (button && wordRe.MatchString(v))) && !strings.HasPrefix(v, "<") { // markup (SVG), not a message
						if _, ok := literalExceptions[v]; !ok {
							hits = append(hits, literalHit{fset.Position(x.Pos()).String(), v})
						}
					}
				case *ast.CompositeLit: // []string{"option", …} given to a prompt
					visit(x.Elts)
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok { // Label: "…" in a form step or field
				if id, ok := kv.Key.(*ast.Ident); ok && literalSinkFields[id.Name] {
					visit([]ast.Expr{kv.Value})
				}
				return true
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				if literalSinkFuncs[fn.Name] {
					visit(call.Args)
				}
			case *ast.SelectorExpr:
				pkg, _ := fn.X.(*ast.Ident)
				switch {
				case pkg != nil && pkg.Name == "errors" && fn.Sel.Name == "New",
					pkg != nil && pkg.Name == "fmt" && (fn.Sel.Name == "Errorf" || fn.Sel.Name == "Sprintf" || fn.Sel.Name == "Sprint" ||
						strings.HasPrefix(fn.Sel.Name, "Fprint") || strings.HasPrefix(fn.Sel.Name, "Print")),
					literalSinkMethods[fn.Sel.Name]:
					button = fn.Sel.Name == "AddButton"
					visit(call.Args)
					button = false
				}
			}
			return true
		})
	}
	return hits
}

func TestNoLiteralUserMessages(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "internal", "tui", "v2", "views")} {
		for _, h := range userLiterals(t, dir) {
			t.Errorf("%s: %q is written in the code: use i18n.T/i18n.Tf (or list it in literalExceptions with its reason)", h.pos, h.text)
		}
	}
}

// U-11: every exception is still in the code (the list does not outlive the
// literals it excuses).
func TestLiteralExceptionsAreUsed(t *testing.T) {
	found := map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range []string{".", filepath.Join("..", "internal", "tui", "v2", "views")} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if l, ok := n.(*ast.BasicLit); ok && l.Kind == token.STRING {
					v, _ := strconv.Unquote(l.Value)
					found[v] = true
				}
				return true
			})
		}
	}
	for lit := range literalExceptions {
		if !found[lit] {
			t.Errorf("literalExceptions: %q is not in the code any more, remove it", lit)
		}
	}
}

// The detection itself: a prose literal in a sink is found, the same text in
// i18n.T or a non-sink is not.
func TestLiteralGuardDetects(t *testing.T) {
	dir := t.TempDir()
	src := `package x
import ("errors"; "fmt")
func f(w interface{ Write([]byte) (int, error) }) error {
	fmt.Fprintf(w, "Création du projet %s\n", "x")
	_ = askYN(w, "Continuer le traitement ?", true)
	_ = askSelect(w, "choix", []string{"Première option", "Seconde option"})
	_ = t("Ceci est un texte")
	_ = fmt.Sprintf("%s: %w", "a", nil)
	return errors.New("something went wrong")
}
func t(string) string { return "" }
func askYN(w interface{ Write([]byte) (int, error) }, p string, d bool) bool { return d }
func askSelect(w interface{ Write([]byte) (int, error) }, p string, o []string) int { return 0 }
`
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, h := range userLiterals(t, dir) {
		got = append(got, h.text)
	}
	want := []string{"Création du projet %s\n", "Continuer le traitement ?", "Première option", "Seconde option", "something went wrong"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("found %q, want %q", got, want)
	}
}
