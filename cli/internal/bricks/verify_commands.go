package bricks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// jsManagers are the package managers whose scripts run the build, tests,
// lint and type check, with the form that runs the local tsc without ever
// downloading it (npx and bunx install a missing package: refused).
var jsManagers = []struct {
	name string
	tsc  string
}{
	{"npm", "npx --no tsc --noEmit*"},
	{"pnpm", "pnpm exec tsc --noEmit*"},
	{"yarn", "yarn tsc --noEmit*"},
	{"bun", "bun run tsc --noEmit*"},
}

// verifySignalCommands are the verification commands of the other signals.
var verifySignalCommands = map[string][]string{
	"golang": {"go build ./...*", "go test*", "go vet*"},
	"python": {"pytest*", "python -m pytest*", "python3 -m pytest*", ".venv/bin/python -m pytest*", ".venv/bin/pytest*", "ruff check*", "mypy*"},
	"uv":     {"uv run --no-sync pytest*", "uv run --no-sync ruff check*", "uv run --no-sync mypy*"},
	"poetry": {"poetry run pytest*", "poetry run ruff check*", "poetry run mypy*"},
	"rust":   {"cargo build*", "cargo test*", "cargo clippy*"},
	"maven":  {"mvn -q test*", "mvn -q verify*", "./mvnw -q test*", "./mvnw -q verify*"},
	"gradle": {"./gradlew test*", "./gradlew build*"},
	"make":   {"make test*", "make build*", "make lint*"},
}

// VerifyCommands returns the shell permission patterns of the commands that
// build, test, lint and type-check the project of signals (DetectSignals):
// npm scripts for any package.json, the pnpm, yarn and bun forms only when
// their lockfile is found, tsc when TypeScript is used, and the tools of the
// other stacks. Never an install or a publish command: the forms that could
// install (npx, uv run) are the ones that refuse to. Sorted, without
// duplicates; nil for a project without signal.
func VerifyCommands(s Signals) []string {
	seen := map[string]bool{}
	var out []string
	add := func(patterns ...string) {
		for _, p := range patterns {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	if s.Has("javascript") {
		for _, m := range jsManagers {
			if m.name != "npm" && !s.Has(m.name) {
				continue
			}
			add(m.name+" test*", m.name+" run test*", m.name+" run build*", m.name+" run lint*", m.name+" run typecheck*")
			if s.Has("typescript") {
				add(m.tsc)
			}
		}
	}
	for name, patterns := range verifySignalCommands {
		if s.Has(name) {
			add(patterns...)
		}
	}
	sort.Strings(out)
	return out
}

// VerifyRun is a concrete verification command run by oh at a checkpoint
// (`verify: { from: stack }`), of one kind (build, test, lint, typecheck).
type VerifyRun struct {
	Kind string
	Args []string
}

// String returns the command line.
func (r VerifyRun) String() string { return strings.Join(r.Args, " ") }

// Verification kinds of VerifyRuns.
const (
	VerifyKindBuild     = "build"
	VerifyKindTest      = "test"
	VerifyKindLint      = "lint"
	VerifyKindTypecheck = "typecheck"
)

// verifyRunsBySignal are the concrete commands of the signals without
// scripts, by kind. Each one is allowed by the patterns of VerifyCommands.
var verifyRunsBySignal = []struct {
	signal string
	kind   string
	cmd    string
}{
	{"golang", VerifyKindBuild, "go build ./..."},
	{"golang", VerifyKindTest, "go test ./..."},
	{"golang", VerifyKindLint, "go vet ./..."},
	{"rust", VerifyKindBuild, "cargo build"},
	{"rust", VerifyKindTest, "cargo test"},
	{"rust", VerifyKindLint, "cargo clippy"},
	{"maven", VerifyKindTest, "mvn -q verify"},
	{"gradle", VerifyKindTest, "./gradlew test"},
	{"gradle", VerifyKindBuild, "./gradlew build"},
}

// VerifyRuns returns the commands oh runs to verify the project folder dir
// (absolute) for kinds, from its signals at rel (the folder relative to
// the project, "." for the root): a Makefile target of the kind first (it
// knows the project), else the scripts of package.json (with the package
// manager of the lockfile), tsc for the type check of a TypeScript project
// without a typecheck script, the Python tests and the tools of the other
// stacks. Every command is allowed by VerifyCommands(s): never an install.
func VerifyRuns(dir string, s Signals, rel string, kinds []string) []VerifyRun {
	if rel == "" {
		rel = "."
	}
	targets := makeTargets(filepath.Join(dir, "Makefile"))
	scripts := packageScripts(filepath.Join(dir, "package.json"))
	mgr := "npm"
	for _, m := range jsManagers[1:] {
		if s.At(m.name, rel) {
			mgr = m.name
			break
		}
	}
	var out []VerifyRun
	add := func(kind, cmd string) { out = append(out, VerifyRun{Kind: kind, Args: strings.Fields(cmd)}) }
	for _, kind := range kinds {
		if s.At("make", rel) && targets[kind] {
			add(kind, "make "+kind)
			continue
		}
		if s.At("javascript", rel) {
			switch {
			case scripts[kind] && kind == VerifyKindTest:
				add(kind, mgr+" test")
			case scripts[kind]:
				add(kind, mgr+" run "+kind)
			case kind == VerifyKindTypecheck && s.At("typescript", rel):
				for _, m := range jsManagers {
					if m.name == mgr {
						add(kind, strings.TrimSuffix(m.tsc, "*"))
					}
				}
			}
		}
		if kind == VerifyKindTest && s.At("python", rel) && s.Has("pytest") {
			switch {
			case s.At("uv", rel):
				add(kind, "uv run --no-sync pytest")
			case s.At("poetry", rel):
				add(kind, "poetry run pytest")
			default:
				add(kind, "python3 -m pytest")
			}
		}
		for _, r := range verifyRunsBySignal {
			if r.kind == kind && s.At(r.signal, rel) {
				if r.signal == "gradle" && !fileExists(filepath.Join(dir, "gradlew")) {
					continue
				}
				add(kind, r.cmd)
			}
		}
	}
	return out
}

var makeTarget = regexp.MustCompile(`(?m)^([A-Za-z][A-Za-z0-9_-]*)\s*:([^=]|$)`)

// makeTargets returns the targets of a Makefile (nil when unreadable).
func makeTargets(file string) map[string]bool {
	data, err := readBounded(file)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, m := range makeTarget.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	return out
}

// packageScripts returns the scripts of a package.json (nil when unreadable).
func packageScripts(file string) map[string]bool {
	data, err := readBounded(file)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	out := map[string]bool{}
	for k, v := range pkg.Scripts {
		// npm init writes a test script that always fails.
		if strings.TrimSpace(v) != "" && !strings.Contains(v, "no test specified") {
			out[k] = true
		}
	}
	return out
}

func readBounded(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, signalMaxFileSize))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// R7 (TDD): test files of a stack and the reason a test run failed.

// testFilePatterns are the test files of each signal (sessionspec.PathMatch
// patterns: without a slash, the base name; `**` any folders).
var testFilePatterns = []struct {
	signal   string
	patterns []string
}{
	{"golang", []string{"*_test.go", "**/testdata/**"}},
	{"javascript", []string{"*.test.*", "*.spec.*", "**/__tests__/**", "**/__mocks__/**", "**/__snapshots__/**"}},
	{"python", []string{"test_*.py", "*_test.py", "conftest.py"}},
	{"rust", []string{"**/tests/**", "**/benches/**"}},
	{"maven", []string{"**/src/test/**"}},
	{"gradle", []string{"**/src/test/**"}},
}

// commonTestPatterns are test folders of every stack.
var commonTestPatterns = []string{"**/test/**", "**/tests/**", "**/spec/**", "**/fixtures/**"}

// TestFilePatterns returns the patterns of the test files of a project of
// signals: those of its stacks and the usual test folders; those of every
// known stack when no signal is detected. Sorted, without duplicates.
func TestFilePatterns(s Signals) []string {
	seen := map[string]bool{}
	var out []string
	add := func(ps []string) {
		for _, p := range ps {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	found := false
	for _, t := range testFilePatterns {
		if s.Has(t.signal) {
			add(t.patterns)
			found = true
		}
	}
	if !found {
		for _, t := range testFilePatterns {
			add(t.patterns)
		}
	}
	add(commonTestPatterns)
	sort.Strings(out)
	return out
}

// Reasons a test command failed for another reason than a failing test
// (RedFailure): the tests do not build, or none ran.
const (
	RedCompile = "compile"
	RedCollect = "collect"
	RedNoTests = "no_tests"
)

// compileMarkers are what the test tools print when the tests do not build:
// go test (`FAIL pkg [build failed]`, `[setup failed]`), cargo, tsc.
var compileMarkers = regexp.MustCompile(`\[build failed\]|\[setup failed\]|error: could not compile|(?m)error TS\d+:`)

// RedFailure tells why a test command written first failed: "" for a test
// that runs and fails (the expected red), else RedCompile (the tests or the
// code do not build), RedCollect (pytest cannot import the tests) or
// RedNoTests (pytest collected none). What a tool does not tell apart
// (jest, vitest, mvn: a failure is a failure) counts as a failing test.
func RedFailure(args []string, exit int, output []byte) string {
	if exit == 0 {
		return ""
	}
	if compileMarkers.Match(output) {
		return RedCompile
	}
	for _, a := range args {
		if a == "pytest" || strings.HasSuffix(a, "/pytest") {
			// pytest: 1 = tests failed, 2 = interrupted (collection
			// errors), 5 = no test collected.
			switch exit {
			case 1:
				return ""
			case 5:
				return RedNoTests
			default:
				return RedCollect
			}
		}
	}
	if strings.Contains(string(output), "ERROR collecting") {
		return RedCollect // pytest behind make or a script
	}
	return ""
}
