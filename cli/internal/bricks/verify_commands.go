package bricks

import "sort"

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
