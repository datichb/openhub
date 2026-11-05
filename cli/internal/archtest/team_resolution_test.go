package archtest

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Which team a command acts on is decided in one place (ResolveTeam in cmd/):
// --team, the team chosen in the TUI, the team of the project, the default
// team. A command that reads ActiveTeam() of a *config.Config directly (however
// it got it: a field, a variable, a parameter, the return of config.Load) takes
// the default team whatever the user asked for, and mixes its repository with
// the member of another team. The files below still read it, for a reason each;
// a new use is refused (a lower count is fine). ActiveTeam() of another type,
// the shell of the TUI for one, is a step of the resolution and is not
// concerned.
var activeTeamAllowed = map[string]int{
	// oh init starts from the hub team it finds.
	"cmd/init_wizard.go":   1,
	"cmd/team_identity.go": 1, // identity of the member in the default team: the same person in every team
	"cmd/history.go":       1, // member of the sessions of the history: the default team
	"cmd/root.go":          1, // migration of the former per-project override towards the default team
	// The hub settings page reads the tracker type and the enforced push labels of the default team. Giving it
	// another team changes SettingsViewConfig and its wiring in cmd/tui_views_hub.go (file of a parallel batch,
	// 541-b): to review with the lot 5.6 (Scope).
	"internal/tui/v2/views/settings_view.go": 1,
}

// configPackage is the package that defines Config and its ActiveTeam method.
const configPackage = module + "/internal/config"

func TestCommandsDoNotReadTheActiveTeamDirectly(t *testing.T) {
	root := moduleRoot(t)
	mod := loadTypedModule(t)
	if !hasConfigActiveTeam(mod.packages[configPackage]) {
		t.Fatalf("%s.Config has no ActiveTeam method any more: the detector would see nothing; update it", configPackage)
	}
	got := map[string]int{}
	scanned := 0
	for path, p := range mod.packages {
		if path == configPackage || path == module+"/internal/archtest" {
			continue
		}
		scanned++
		for _, pos := range configActiveTeamUses(mod.fset, p.info) {
			rel, err := filepath.Rel(root, pos.Filename)
			if err != nil {
				t.Fatal(err)
			}
			got[filepath.ToSlash(rel)]++
		}
	}
	if scanned == 0 {
		t.Fatal("no package scanned")
	}
	for file, n := range got {
		if allowed := activeTeamAllowed[file]; n > allowed {
			t.Errorf("%s: %d use(s) of (*config.Config).ActiveTeam, %d allowed; resolve the team with ResolveTeam (cmd/team_resolve.go) or Service.ResolveTeam so that --team, the team of the TUI and the team of the project are honoured", file, n, allowed)
		}
	}
}

// hasConfigActiveTeam reports whether the package config has a type Config with
// an ActiveTeam method.
func hasConfigActiveTeam(p *typedPackage) bool {
	if p == nil {
		return false
	}
	obj := p.pkg.Scope().Lookup("Config")
	if obj == nil {
		return false
	}
	return types.NewMethodSet(types.NewPointer(obj.Type())).Lookup(p.pkg, "ActiveTeam") != nil
}

// TestConfigActiveTeamDetector: a *config.Config is seen under every form, the
// ActiveTeam method of another type is not, and a comment is no use.
func TestConfigActiveTeamDetector(t *testing.T) {
	const configSrc = `package config

type TeamConfig struct{ ID string }
type Config struct{}

func (c *Config) ActiveTeam() TeamConfig { return TeamConfig{} }

func Load() *Config { return &Config{} }
`
	// A type named Config in another package is not the Config of oh.
	const otherSrc = `package other

type Config struct{}

func (c *Config) ActiveTeam() int { return 0 }
`
	const appSrc = `package app

import (
	"example.com/other"
	"github.com/datichb/openhub/cli/internal/config"
)

// a.cfg.ActiveTeam() and cfg.ActiveTeam() in a comment are no call.

type shell struct{}

func (shell) ActiveTeam() *int { return nil }

type app struct {
	cfg    *config.Config
	Config *config.Config
	sh     shell
	o      other.Config
}

type embedding struct{ *config.Config }

type holder struct{ c config.Config }

func fromField(a app) {
	_ = a.cfg.ActiveTeam() // want
	_ = a.Config.ActiveTeam().ID // want
	_ = a.sh.ActiveTeam()
	_ = a.o.ActiveTeam()
}

func fromVariable() {
	var cfg *config.Config
	_ = cfg.ActiveTeam() // want
}

func fromParameter(cfg *config.Config) int {
	return len(cfg.ActiveTeam().ID) // want
}

func fromReturn() {
	_ = config.Load().ActiveTeam() // want
}

func fromValue(h holder) {
	var v config.Config
	_ = v.ActiveTeam() // want
	_ = h.c.ActiveTeam() // want
}

func fromEmbedding(e embedding) {
	_ = e.ActiveTeam() // want
}

func asValue(cfg *config.Config) {
	f := cfg.ActiveTeam // want
	_ = f
	g := (*config.Config).ActiveTeam // want
	_ = g
}

func notConcerned(s shell, o *other.Config) {
	_ = s.ActiveTeam()
	_ = shell{}.ActiveTeam()
	_ = o.ActiveTeam()
}
`
	fset := token.NewFileSet()
	imports := map[string]*types.Package{}
	check := func(path, name, src string) *types.Info {
		t.Helper()
		f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
		conf := types.Config{Importer: importerFunc(func(p string) (*types.Package, error) {
			if pkg, ok := imports[p]; ok {
				return pkg, nil
			}
			return importer.Default().Import(p)
		})}
		pkg, err := conf.Check(path, fset, []*ast.File{f}, info)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		imports[path] = pkg
		return info
	}
	check(configPackage, "config.go", configSrc)
	check("example.com/other", "other.go", otherSrc)
	info := check("example.com/app", "app.go", appSrc)

	var want []int
	for i, line := range strings.Split(appSrc, "\n") {
		if strings.HasSuffix(line, "// want") {
			want = append(want, i+1)
		}
	}
	if len(want) != 10 {
		t.Fatalf("the source of the test changed: %d expected uses, want 10", len(want))
	}
	var got []int
	for _, pos := range configActiveTeamUses(fset, info) {
		got = append(got, pos.Line)
	}
	sort.Ints(got)
	if !slices.Equal(got, want) {
		t.Fatalf("lines of the uses of (*config.Config).ActiveTeam: got %v, want %v", got, want)
	}
}

type importerFunc func(string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// configActiveTeamUses returns the position of every use of the ActiveTeam
// method of config.Config (a call, a method value, a method expression), on a
// *config.Config or a config.Config however it is reached: a variable, a field,
// a parameter, the return of a function, an embedded field.
func configActiveTeamUses(fset *token.FileSet, info *types.Info) []token.Position {
	return usesOf(fset, info, func(obj types.Object) bool {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Name() != "ActiveTeam" {
			return false
		}
		sig, ok := fn.Origin().Type().(*types.Signature)
		if !ok || sig.Recv() == nil {
			return false
		}
		recv := sig.Recv().Type()
		if ptr, ok := recv.(*types.Pointer); ok {
			recv = ptr.Elem()
		}
		named, ok := types.Unalias(recv).(*types.Named)
		if !ok {
			return false
		}
		o := named.Obj()
		return o.Pkg() != nil && o.Pkg().Path() == configPackage && o.Name() == "Config"
	})
}
