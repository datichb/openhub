package archtest

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The tests that need to know the type of an expression (not only its spelling)
// read the module through go/types. go/packages is not a dependency of the
// module, so the loading is done here, with the standard library only:
//
//   - go list gives the packages of the module and the packages they depend on;
//   - the packages of the module are type-checked from their source (the
//     //go:embed files do not have to exist: go/types does not read them);
//   - every other package (the standard library, the third-party modules) is
//     read from the export data that go list -export builds, or takes from the
//     build cache.
//
// Only the files that the platform of the test compiles are seen: the files of
// another operating system are not.

type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Module     *struct {
		Main      bool
		GoVersion string
	}
}

func (p *listedPackage) inMainModule() bool { return p.Module != nil && p.Module.Main }

// typedPackage is a package of the module, type-checked.
type typedPackage struct {
	pkg  *types.Package
	info *types.Info
}

type typedModule struct {
	fset *token.FileSet
	// packages are the packages of cmd/ and internal/, by import path.
	packages map[string]*typedPackage
}

// loadTypedModule type-checks the packages of cmd/ and internal/ of the module
// (not their tests) and fails the test on the first type error: a package
// half understood would let a call go unseen.
func loadTypedModule(t *testing.T) *typedModule {
	t.Helper()
	root := moduleRoot(t)

	var listed []*listedPackage
	dec := json.NewDecoder(bytes.NewReader(goList(t, root, "-e", "-deps", "-json=ImportPath,Dir,GoFiles,Module", "./cmd/...", "./internal/...")))
	for {
		p := new(listedPackage)
		if err := dec.Decode(p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("go list: %v", err)
		}
		listed = append(listed, p)
	}

	byPath := map[string]*listedPackage{}
	var external []string
	for _, p := range listed {
		byPath[p.ImportPath] = p
		if !p.inMainModule() && p.ImportPath != "unsafe" {
			external = append(external, p.ImportPath)
		}
	}

	exports := map[string]string{}
	args := append([]string{"-e", "-export", "-f", "{{.ImportPath}}\t{{.Export}}"}, external...)
	for _, line := range strings.Split(string(goList(t, root, args...)), "\n") {
		if path, file, ok := strings.Cut(line, "\t"); ok && file != "" {
			exports[path] = file
		}
	}

	fset := token.NewFileSet()
	l := &moduleLoader{
		t:      t,
		fset:   fset,
		byPath: byPath,
		done:   map[string]*typedPackage{},
		gc: importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
			file, ok := exports[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return os.Open(file)
		}),
	}
	mod := &typedModule{fset: fset, packages: map[string]*typedPackage{}}
	for _, p := range listed {
		rel := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, module), "/")
		scanned := rel == "cmd" || strings.HasPrefix(rel, "cmd/") || strings.HasPrefix(rel, "internal/")
		if !p.inMainModule() || !scanned || strings.HasPrefix(rel, "internal/hubcontent/hub") {
			continue
		}
		mod.packages[p.ImportPath] = l.check(p)
	}
	if len(l.errs) > 0 {
		t.Fatalf("%d type error(s) while reading the module, the first ones:\n%s", len(l.errs), strings.Join(l.errs[:min(len(l.errs), 5)], "\n"))
	}
	return mod
}

func goList(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", strings.Join(args[:min(len(args), 4)], " "), err, stderr.String())
	}
	return out
}

type moduleLoader struct {
	t      *testing.T
	fset   *token.FileSet
	byPath map[string]*listedPackage
	done   map[string]*typedPackage
	gc     types.Importer
	errs   []string
}

func (l *moduleLoader) Import(path string) (*types.Package, error) {
	if p := l.byPath[path]; p != nil && p.inMainModule() {
		return l.check(p).pkg, nil
	}
	return l.gc.Import(path)
}

func (l *moduleLoader) check(p *listedPackage) *typedPackage {
	if tp, ok := l.done[p.ImportPath]; ok {
		return tp
	}
	var files []*ast.File
	for _, name := range p.GoFiles {
		f, err := parser.ParseFile(l.fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			l.errs = append(l.errs, err.Error())
			continue
		}
		files = append(files, f)
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	conf := types.Config{
		Importer:  l,
		GoVersion: "go" + p.Module.GoVersion,
		Error:     func(err error) { l.errs = append(l.errs, err.Error()) },
	}
	pkg, _ := conf.Check(p.ImportPath, l.fset, files, info)
	tp := &typedPackage{pkg: pkg, info: info}
	l.done[p.ImportPath] = tp
	return tp
}

// usesOf returns the position of every use of an object that matches, in
// the order of the source.
func usesOf(fset *token.FileSet, info *types.Info, match func(types.Object) bool) []token.Position {
	var out []token.Position
	for id, obj := range info.Uses {
		if match(obj) {
			out = append(out, fset.Position(id.Pos()))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Filename != out[j].Filename {
			return out[i].Filename < out[j].Filename
		}
		return out[i].Offset < out[j].Offset
	})
	return out
}
