package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// B8 (audit v5, A-01): the packages that use the team-state repository
// through the small interfaces of internal/teamstate (RepoInfo, ClaimReader,
// MemberReader, ...) do not name the concrete *teamstate.Repo type: a test
// serves them a fake, and the interfaces say what each one needs. Creating a
// repository (teamstate.NewRepo) stays allowed.
var interfaceOnlyDirs = []string{
	"internal/tracker/",
	"internal/mcp/team/",
}

const teamstatePath = "github.com/datichb/openhub/cli/internal/teamstate"

// concreteRepoUses returns the position of every use of the type
// teamstate.Repo in the source (a comment or a string naming it is no use).
func concreteRepoUses(t *testing.T, fset *token.FileSet, name string, src any) []token.Position {
	t.Helper()
	return teamstateUses(t, fset, name, src, "Repo")
}

// teamstateUses returns the position of every selector teamstate.<one of
// names> in the source.
func teamstateUses(t *testing.T, fset *token.FileSet, name string, src any, names ...string) []token.Position {
	t.Helper()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	alias := ""
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != teamstatePath {
			continue
		}
		alias = "teamstate"
		if imp.Name != nil {
			alias = imp.Name.Name
		}
	}
	if alias == "" {
		return nil
	}
	var out []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && slices.Contains(names, sel.Sel.Name) {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == alias {
				out = append(out, fset.Position(sel.Pos()))
			}
		}
		return true
	})
	return out
}

func TestInterfaceOnlyPackagesDoNotNameTheConcreteRepo(t *testing.T) {
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		guarded := false
		for _, dir := range interfaceOnlyDirs {
			guarded = guarded || strings.HasPrefix(f.rel, dir)
		}
		if !guarded {
			continue
		}
		scanned++
		for _, pos := range concreteRepoUses(t, fset, f.path, nil) {
			t.Errorf("%s:%d: *teamstate.Repo named in a package that takes the interfaces of teamstate (RepoInfo, ClaimReader, ...); declare the interface it needs", f.rel, pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned: interfaceOnlyDirs no longer matches anything")
	}
}

// B9: the views of the TUI take the small interfaces of teamstate they need
// (ClaimReader, MemberReader, WikiWriter, ...), not the whole repository:
// they name neither *teamstate.Repo nor the temporary compositions
// TeamStateReader / TeamStateWriter. Creating a repository
// (teamstate.NewRepo) is the one thing that stays, since the views open the
// clone of the team themselves; its result is held as an interface.
var tuiWideRepoTypes = []string{"Repo", "TeamStateReader", "TeamStateWriter"}

// tuiWideRepoExceptions lists the files of internal/tui that may still name
// one of tuiWideRepoTypes, each with its reason. None for now.
var tuiWideRepoExceptions = map[string]string{}

func TestTUIViewsDoNotNameTheWholeRepo(t *testing.T) {
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") || !strings.HasPrefix(f.rel, "internal/tui/") {
			continue
		}
		scanned++
		if _, ok := tuiWideRepoExceptions[f.rel]; ok {
			continue
		}
		for _, pos := range teamstateUses(t, fset, f.path, nil, tuiWideRepoTypes...) {
			t.Errorf("%s:%d: teamstate.Repo, TeamStateReader or TeamStateWriter named in the TUI; take the interfaces of the sub-domains the view needs (ClaimReader, MemberReader, WikiWriter, ...)", f.rel, pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned: internal/tui no longer matches anything")
	}
}

func TestTeamstateUsesDetectorNamesSeveralTypes(t *testing.T) {
	const src = `package x

import "` + teamstatePath + `"

func f(r teamstate.TeamStateReader, w teamstate.TeamStateWriter, c teamstate.ClaimReader) *teamstate.Repo { return nil }
`
	got := teamstateUses(t, token.NewFileSet(), "x.go", src, tuiWideRepoTypes...)
	if len(got) != 3 {
		t.Fatalf("want 3 uses (reader, writer, repo), got %v", got)
	}
}

func TestConcreteRepoDetector(t *testing.T) {
	const src = `package x

import ts "` + teamstatePath + `"

// teamstate.Repo in a comment is no use.
var _ = "teamstate.Repo in a string is none either"

func f(r *ts.Repo) ts.ClaimReader { return r }
func g() ts.RepoInfo              { return ts.NewRepo("", "") }
`
	got := concreteRepoUses(t, token.NewFileSet(), "x.go", src)
	if len(got) != 1 || got[0].Line != 8 {
		t.Fatalf("want one use at line 8, got %v", got)
	}
}
