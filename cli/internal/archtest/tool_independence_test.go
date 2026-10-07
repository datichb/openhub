// Package archtest holds architecture tests over the whole module.
package archtest

import (
	"encoding/json"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// D19 (tool independence, 01-decisions): every specificity of a tool
// (names, provider ids, environment variables, API, paths, formats,
// versions, binary, displayed names) lives only in internal/adapters/<tool>.
// Outside of it: neutral interfaces and types; a single composition root
// (cmd/v5_adapters.go) picks the implementation. These tests fail on any
// import of an adapter, identifier or literal naming the tool, or native
// agent name of the tool, outside the short allow-list below.

const module = "github.com/datichb/openhub/cli"

// toolAdapters are the adapter packages (directory under internal/adapters).
var toolAdapters = []string{"opencodev2"}

// toolWord matches the name of a tool in identifiers and literals.
var toolWord = regexp.MustCompile(`(?i)opencode`)

// nativeAgents are the agents the tool ships (closed world, D13; rule 5 of
// 00-INDEX): only the adapter disables them.
var nativeAgents = []string{"build", "plan", "general", "explore"}

// allowed reports whether a file of the module may name a tool: the adapters
// themselves, the composition root, the legacy configuration keys read for
// compatibility, and the contract tests run against a real tool server
// (build tags integration, e2e, container).
func allowed(rel string, tags string) bool {
	switch {
	case strings.HasPrefix(rel, "internal/adapters/opencodev2/"):
		return true
	case rel == "cmd/v5_adapters.go", rel == "internal/config/legacy_keys.go", rel == "internal/config/legacy_keys_test.go":
		return true
	case strings.HasSuffix(rel, "_test.go") && tags != "":
		expr, err := constraint.Parse("//go:build " + tags)
		if err != nil {
			return false
		}
		for _, tag := range []string{"integration", "e2e", "container"} {
			if strings.Contains(expr.String(), tag) {
				return true
			}
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

type goFile struct {
	rel, path, tags string
}

func goFiles(t *testing.T) []goFile {
	t.Helper()
	root := moduleRoot(t)
	var out []goFile
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "bin" || rel == "dist" || strings.HasPrefix(rel, "internal/hubcontent/hub") || strings.HasPrefix(rel, "internal/archtest") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f := goFile{rel: rel, path: p}
		if data, err := os.ReadFile(p); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "//go:build ") {
					f.tags = strings.TrimPrefix(line, "//go:build ")
					break
				}
				if strings.HasPrefix(line, "package ") {
					break
				}
			}
		}
		out = append(out, f)
		return nil
	})
	return out
}

const d19 = "D19 (01-decisions): tool specifics live only in internal/adapters/<tool>; use a neutral interface or capability of the adapter, the tool name of ToolInfo, or the composition root cmd/v5_adapters.go"

// TestOnlyTheCompositionRootImportsAnAdapter: rule (1).
func TestOnlyTheCompositionRootImportsAnAdapter(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range goFiles(t) {
		if allowed(f.rel, f.tags) {
			continue
		}
		file, err := parser.ParseFile(fset, f.path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, ad := range toolAdapters {
				if strings.HasPrefix(path, module+"/internal/adapters/"+ad) {
					t.Errorf("%s imports %s — %s", f.rel, path, d19)
				}
			}
		}
	}
}

// TestNoToolNameOutsideTheAdapter: rules (2) and (3), over identifiers and
// string literals (comments are not code).
func TestNoToolNameOutsideTheAdapter(t *testing.T) {
	fset := token.NewFileSet()
	var found []string
	for _, f := range goFiles(t) {
		if allowed(f.rel, f.tags) {
			continue
		}
		file, err := parser.ParseFile(fset, f.path, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if toolWord.MatchString(x.Name) {
					found = append(found, fset.Position(x.Pos()).String()+": identifier "+x.Name)
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING && toolWord.MatchString(x.Value) {
					found = append(found, fset.Position(x.Pos()).String()+": literal "+x.Value)
				}
			case *ast.CompositeLit:
				n := 0
				for _, e := range x.Elts {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						for _, a := range nativeAgents {
							if lit.Value == `"`+a+`"` {
								n++
							}
						}
					}
				}
				if n >= 2 {
					found = append(found, fset.Position(x.Pos()).String()+": native agents of the tool")
				}
			}
			return true
		})
	}
	sort.Strings(found)
	for _, s := range found {
		root := moduleRoot(t) + string(filepath.Separator)
		t.Errorf("%s — %s", strings.TrimPrefix(s, root), d19)
	}
}

// TestNoToolNameInMessages: the displayed texts name the tool through the
// adapter (ToolInfo.DisplayName as a parameter), never as a fixed word.
func TestNoToolNameInMessages(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		data, err := os.ReadFile(filepath.Join(moduleRoot(t), "internal", "i18n", "locales", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			t.Fatal(err)
		}
		var keys []string
		for k, v := range msgs {
			if toolWord.MatchString(k) || toolWord.MatchString(v) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Errorf("%s %s names the tool: %q — %s", lang, k, msgs[k], d19)
		}
	}
}

// TestDependencyGraph: rule (1) at package level (go list -deps), so that a
// package does not reach an adapter through another one.
func TestDependencyGraph(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found")
	}
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if pkg == module+"/cmd" || strings.HasPrefix(pkg, module+"/internal/adapters/") {
			continue
		}
		for _, imp := range fields[1:] {
			for _, ad := range toolAdapters {
				if strings.HasPrefix(imp, module+"/internal/adapters/"+ad) {
					t.Errorf("package %s imports %s — %s", pkg, imp, d19)
				}
			}
		}
	}
}
