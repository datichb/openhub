package archtest

import (
	"go/parser"
	"go/token"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// Layers: the business packages know neither the screens nor the command
// line, the screens do not reach the storage, the configuration does not open
// a database. The violations that exist today are listed (and frozen) below:
// the test fails when one is added, and when a listed one is gone, so that
// the list only shrinks.

// layerRule forbids the files (or packages) selected by from to import what
// forbidden selects. exceptions holds the violations accepted for now, as
// "<file or package> -> <import>" with the reason.
type layerRule struct {
	name       string
	advice     string
	from       func(rel string) bool
	forbidden  func(imp string) bool
	transitive bool // checked on the whole dependency graph (go list -deps), per package
	exceptions map[string]string
}

func under(prefixes ...string) func(string) bool {
	return func(rel string) bool {
		for _, p := range prefixes {
			if rel == p || strings.HasPrefix(rel, p+"/") {
				return true
			}
		}
		return false
	}
}

func importsOf(prefixes ...string) func(string) bool {
	return func(imp string) bool {
		for _, p := range prefixes {
			if imp == p || strings.HasPrefix(imp, p+"/") {
				return true
			}
		}
		return false
	}
}

// storageRoot are the files of cmd that may open the database and build its
// stores: they are the composition root of the command line.
var storageRoot = map[string]bool{
	"cmd/root.go":       true,
	"cmd/startup.go":    true,
	"cmd/v5_runtime.go": true,
}

var layerRules = []layerRule{
	{
		name:   "business packages do not depend on the screens or the command line",
		advice: "services, runsvc, daemon and domain are used by the TUI and by cmd, never the other way round: move what they need into a neutral package or pass it as a parameter (interface, callback)",
		from:   under("internal/services", "internal/runsvc", "internal/daemon", "internal/domain"),
		forbidden: func(imp string) bool {
			return imp == module+"/cmd" || importsOf(module+"/internal/tui")(imp)
		},
		transitive: true,
	},
	{
		name:      "the screens do not import the storage",
		advice:    "a view receives what it displays through a callback or an interface injected by cmd (see the Deps of the views); storage packages are opened by the composition root",
		from:      under("internal/tui"),
		forbidden: importsOf(module + "/internal/storage"),
		exceptions: map[string]string{
			"internal/tui/v2/views/secrets_view.go -> " + module + "/internal/storage/keychain": "the secrets view reads the keychain index (keychain.SecretEntry, injected *keychain.Store)",
		},
	},
	{
		name:      "the configuration does not open a database",
		advice:    "internal/config reads and writes files only; what touches SQLite belongs to internal/storage/sqlite (or a migration command in cmd)",
		from:      under("internal/config"),
		forbidden: importsOf("database/sql"),
		exceptions: map[string]string{
			"internal/config/migrate.go -> database/sql": "MigrateProjectTeamToTeamID updates projects.team_id through a *sql.DB passed by cmd",
		},
	},
	{
		name:      "the launcher does not import the TUI",
		advice:    "launching a session is not a screen: the launcher gets its styles through a parameter or a small neutral package",
		from:      under("internal/launcher"),
		forbidden: importsOf(module + "/internal/tui"),
		exceptions: map[string]string{
			"internal/launcher/ui_cli.go -> " + module + "/internal/tui/theme": "the CLI prompts of the launcher (ui_cli.go) use the theme forms and styles",
		},
	},
	{
		name:       "the daemon client and its API do not import the daemon",
		advice:     "daemon/api (types, routes, errors, paths) and daemon/client are below daemon: a type both sides need goes into daemon/api",
		from:       under("internal/daemon/client", "internal/daemon/api"),
		forbidden:  func(imp string) bool { return imp == module+"/internal/daemon" },
		transitive: true,
	},
	{
		name:       "runsvc, services and mcp reach the daemon through daemon/client and daemon/api",
		advice:     "import internal/daemon/client (Client, Ensure) and internal/daemon/api (types, errors, paths), not internal/daemon, which is the server; cmd is the one place that starts it",
		from:       under("internal/runsvc", "internal/services", "internal/mcp"),
		forbidden:  func(imp string) bool { return imp == module+"/internal/daemon" },
		transitive: true,
	},
	{
		name:   "cmd opens the database only from its composition root",
		advice: "a command receives the store (or the service built on it) from the app context set up by cmd/root.go, cmd/startup.go and cmd/v5_runtime.go; do not import internal/storage/sqlite in a new file of cmd",
		from: func(rel string) bool {
			return strings.HasPrefix(rel, "cmd/") && strings.Count(rel, "/") == 1 && !storageRoot[rel]
		},
		forbidden: importsOf(module + "/internal/storage/sqlite"),
		exceptions: cmdStorageExceptions(
			"budget.go", "daemon.go", "daemon_remote.go", "doctor_orphans.go", "doctor_security.go", "eval.go",
			"eval_fanout.go", "export.go", "fanout.go", "gc.go", "purge.go", "runner.go", "session_trace.go",
			"tui_remote.go", "tui_sessions_tabs.go", "v5_inputs_traces.go", "v5_remote.go",
		),
	},
}

// cmdStorageExceptions are the files of cmd that build their own stores today.
// repair.go, doctor_db.go and errors_i18n.go open or describe the database on
// purpose, without migrating it (OpenRaw, DBPath, MigrationError).
func cmdStorageExceptions(files ...string) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		out["cmd/"+f+" -> "+module+"/internal/storage/sqlite"] = "builds its stores itself; to move to the composition root"
	}
	for _, f := range []string{"repair.go", "doctor_db.go", "errors_i18n.go"} {
		out["cmd/"+f+" -> "+module+"/internal/storage/sqlite"] = "opens or describes the database without migrating it (OpenRaw, DBPath, MigrationError)"
	}
	return out
}

// TestLayers checks every layer rule over the production code (test files
// are not part of the layers).
func TestLayers(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found")
	}
	files := layerFileImports(t)
	deps := packageDeps(t)

	for _, r := range layerRules {
		t.Run(r.name, func(t *testing.T) {
			found := map[string]bool{}
			if r.transitive {
				for pkg, list := range deps {
					if !r.from(pkg) {
						continue
					}
					for _, d := range list {
						if r.forbidden(d) {
							found[pkg+" -> "+strings.TrimPrefix(d, module+"/")] = true
						}
					}
				}
			} else {
				for file, imps := range files {
					if !r.from(file) {
						continue
					}
					for _, imp := range imps {
						if r.forbidden(imp) {
							found[file+" -> "+imp] = true
						}
					}
				}
			}
			for _, v := range sortedKeys(found) {
				if _, ok := r.exceptions[v]; !ok {
					t.Errorf("%s: %s — %s (a known violation is listed in layerRules, but the list must not grow)", r.name, v, r.advice)
				}
			}
			stale := []string{}
			for e := range r.exceptions {
				if !found[e] {
					stale = append(stale, e)
				}
			}
			sort.Strings(stale)
			for _, e := range stale {
				t.Errorf("%s: the exception %q no longer happens — remove it from layerRules", r.name, e)
			}
		})
	}
}

// layerFileImports lists the imports of every production file of the module,
// by path relative to the module root.
func layerFileImports(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string][]string{}
	for _, f := range goFiles(t) {
		if strings.HasSuffix(f.rel, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f.path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		for _, imp := range file.Imports {
			out[f.rel] = append(out[f.rel], strings.Trim(imp.Path.Value, `"`))
		}
	}
	return out
}

// packageDeps is the whole dependency list (go list -deps) of every package
// of the module, by path relative to the module root.
func packageDeps(t *testing.T) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Deps " "}}`, "./...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	res := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, deps, _ := strings.Cut(line, "|")
		rel := strings.TrimPrefix(strings.TrimPrefix(pkg, module), "/")
		if rel == "" {
			rel = "."
		}
		res[rel] = strings.Fields(deps)
	}
	return res
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
