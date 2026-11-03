package archtest

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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
// they do not name *teamstate.Repo. Creating a repository
// (teamstate.NewRepo) is the one thing that stays, since the views open the
// clone of the team themselves; its result is held as an interface.
var tuiWideRepoTypes = []string{"Repo"}

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
			t.Errorf("%s:%d: teamstate.Repo named in the TUI; take the interfaces of the sub-domains the view needs (ClaimReader, MemberReader, WikiWriter, ...)", f.rel, pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned: internal/tui no longer matches anything")
	}
}

func TestTeamstateUsesDetectorNamesSeveralTypes(t *testing.T) {
	const src = `package x

import "` + teamstatePath + `"

func f(r teamstate.ClaimReader, w teamstate.MemberWriter, c teamstate.ConfigReader) *teamstate.Repo { return nil }
`
	got := teamstateUses(t, token.NewFileSet(), "x.go", src, "ClaimReader", "MemberWriter", "Repo")
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

// B10: the commands of cmd/ take the small interfaces of teamstate they need
// (or a minimal composition of them declared next to the command), not the
// whole repository. *teamstate.Repo stays where a command builds the
// repository or needs what no interface of a sub-domain offers (the clone, the
// workflows of services/workflow). Those files are listed, with the number of
// uses and the reason, in testdata/teamstate_repo_exceptions.txt: the list
// only shrinks, a new use fails the test, and so does a count that went down
// until the line is updated or removed.
const teamstateRepoExceptionsFile = "testdata/teamstate_repo_exceptions.txt"

// cmdRepoUses counts the uses of teamstate.Repo per file of cmd/ (the files of
// the package itself, not its tests).
func cmdRepoUses(t *testing.T) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]int{}
	scanned := 0
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") || !strings.HasPrefix(f.rel, "cmd/") || strings.Contains(strings.TrimPrefix(f.rel, "cmd/"), "/") {
			continue
		}
		scanned++
		if n := len(concreteRepoUses(t, fset, f.path, nil)); n > 0 {
			out[f.rel] = n
		}
	}
	if scanned == 0 {
		t.Fatal("no file scanned: cmd/ no longer matches anything")
	}
	return out
}

// readTeamstateRepoExceptions reads the frozen list: "<file> <count>", with #
// for comments (the reason of each line is written above it).
func readTeamstateRepoExceptions(t *testing.T) map[string]int {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleRoot(t), "internal", "archtest", filepath.FromSlash(teamstateRepoExceptionsFile)))
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
			t.Fatalf("%s:%d: expected \"<file> <count>\", got %q", teamstateRepoExceptionsFile, n, line)
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil || count < 1 {
			t.Fatalf("%s:%d: bad count %q", teamstateRepoExceptionsFile, n, fields[1])
		}
		if !strings.HasPrefix(fields[0], "cmd/") {
			t.Fatalf("%s:%d: %q is not a file of cmd/", teamstateRepoExceptionsFile, n, fields[0])
		}
		if _, dup := out[fields[0]]; dup {
			t.Fatalf("%s:%d: %q is listed twice", teamstateRepoExceptionsFile, n, fields[0])
		}
		out[fields[0]] = count
	}
	return out
}

func TestCmdDoesNotNameTheWholeRepo(t *testing.T) {
	uses, frozen := cmdRepoUses(t), readTeamstateRepoExceptions(t)
	var files []string
	for file := range uses {
		files = append(files, file)
	}
	for file := range frozen {
		if _, ok := uses[file]; !ok {
			files = append(files, file)
		}
	}
	sort.Strings(files)
	for _, file := range files {
		got, want := uses[file], frozen[file]
		switch {
		case got == want:
		case want == 0:
			t.Errorf("%s: *teamstate.Repo named %d time(s); take the interfaces of the sub-domains the function needs (ClaimReader, ConfigWriter, ...) or a minimal composition of them, and keep the concrete type for the command that builds the repository", file, got)
		case got == 0:
			t.Errorf("%s: no *teamstate.Repo left — remove the line \"%s %d\" from %s", file, file, want, teamstateRepoExceptionsFile)
		case got < want:
			t.Errorf("%s: %d use(s) of *teamstate.Repo left, %d listed — write \"%s %d\" in %s", file, got, want, file, got, teamstateRepoExceptionsFile)
		default:
			t.Errorf("%s: %d use(s) of *teamstate.Repo, %d listed — the list only shrinks: take an interface instead", file, got, want)
		}
	}
}

func TestCmdRepoUsesIgnoreCommentsAndAliases(t *testing.T) {
	const src = `package cmd

import ts "` + teamstatePath + `"

// teamstate.Repo in a comment is no use.
func a(r *ts.Repo) {}
func b(r ts.ClaimReader) *ts.Repo { return nil }
`
	if got := concreteRepoUses(t, token.NewFileSet(), "x.go", src); len(got) != 2 {
		t.Fatalf("want 2 uses, got %v", got)
	}
}
