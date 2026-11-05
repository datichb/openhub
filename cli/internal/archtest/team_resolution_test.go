package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Which team a command acts on is decided in one place (ResolveTeam in cmd/):
// --team, the team chosen in the TUI, the team of the project, the default
// team. A command that reads Config.ActiveTeam() directly takes the default team
// whatever the user asked for, and mixes its repository with the member of
// another team. The files below still read it, for a reason each; a new use is
// refused (a lower count is fine).
var activeTeamAllowed = map[string]int{
	// The palette « Reconfigurer » and oh init start from the hub team they find.
	"cmd/tui_team_actions.go": 1,
	"cmd/init_wizard.go":      1,
	"cmd/team_identity.go":    1, // identity of the member in the default team: the same person in every team
	"cmd/history.go":          1, // member of the sessions of the history: the default team
	"cmd/root.go":             1, // migration of the former per-project override towards the default team
	"cmd/tui_commands.go":     1, // commands of the palette: the home/palette lot rewrites them
}

func TestCommandsDoNotReadTheActiveTeamDirectly(t *testing.T) {
	fset := token.NewFileSet()
	got := map[string]int{}
	scanned := 0
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") || (!strings.HasPrefix(f.rel, "cmd/") && !strings.HasPrefix(f.rel, "internal/")) {
			continue
		}
		scanned++
		for range configActiveTeamCalls(t, fset, f.path, nil) {
			got[f.rel]++
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned")
	}
	for file, n := range got {
		if allowed := activeTeamAllowed[file]; n > allowed {
			t.Errorf("%s: %d use(s) of Config.ActiveTeam(), %d allowed; resolve the team with ResolveTeam (cmd/team_resolve.go) or Service.ResolveTeam so that --team, the team of the TUI and the team of the project are honoured", file, n, allowed)
		}
	}
}

func TestConfigActiveTeamDetector(t *testing.T) {
	const src = `package x

// a.Config.ActiveTeam() in a comment is no call.
func f(a struct{ Config cfg }, sh shell) {
	_ = a.Config.ActiveTeam()
	_ = sh.ActiveTeam()
	_ = a.Config.ActiveTeam().MemberID
}
`
	got := configActiveTeamCalls(t, token.NewFileSet(), "x.go", src)
	if len(got) != 2 || got[0].Line != 5 || got[1].Line != 7 {
		t.Fatalf("want the calls of lines 5 and 7, got %v", got)
	}
}

// configActiveTeamCalls returns the position of every call <x>.Config.ActiveTeam().
func configActiveTeamCalls(t *testing.T, fset *token.FileSet, name string, src any) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ActiveTeam" {
			return true
		}
		if recv, ok := sel.X.(*ast.SelectorExpr); ok && recv.Sel.Name == "Config" {
			out = append(out, fset.Position(sel.Pos()))
		}
		return true
	})
	return out
}
