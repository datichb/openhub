package bricks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// C1.6: the reviewer runs the build, tests, lint and type check of the
// detected stack, never an install or a publish.
func TestVerifyCommands(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		want   []string
		absent []string
	}{
		{"go", map[string]string{"go.mod": "module x\n"}, []string{"go build ./...*", "go test*", "go vet*"}, []string{"npm test*"}},
		{"npm typescript", map[string]string{"package.json": `{"devDependencies":{"typescript":"6","vitest":"4"}}`},
			[]string{"npm test*", "npm run build*", "npm run lint*", "npm run typecheck*", "npm run test*", "npx --no tsc --noEmit*"},
			[]string{"pnpm test*", "yarn test*", "bun test*"}},
		{"plain js", map[string]string{"package.json": `{}`}, []string{"npm test*"}, []string{"npx --no tsc --noEmit*"}},
		{"pnpm", map[string]string{"package.json": `{"devDependencies":{"typescript":"6"}}`, "pnpm-lock.yaml": ""},
			[]string{"npm test*", "pnpm test*", "pnpm run build*", "pnpm exec tsc --noEmit*"}, []string{"yarn test*"}},
		{"yarn and bun", map[string]string{"package.json": `{}`, "yarn.lock": "", "bun.lock": ""},
			[]string{"yarn test*", "yarn run lint*", "bun test*", "bun run build*"}, []string{"pnpm test*"}},
		{"python uv", map[string]string{"pyproject.toml": "[project]\nname = 'x'\n", "uv.lock": ""},
			[]string{"pytest*", ".venv/bin/python -m pytest*", "uv run --no-sync pytest*", "ruff check*", "mypy*"}, []string{"poetry run pytest*"}},
		{"poetry", map[string]string{"pyproject.toml": "[tool.poetry]\nname = 'x'\n", "poetry.lock": ""},
			[]string{"poetry run pytest*"}, []string{"uv run --no-sync pytest*"}},
		{"rust", map[string]string{"Cargo.toml": "[package]\n"}, []string{"cargo build*", "cargo test*", "cargo clippy*"}, nil},
		{"maven", map[string]string{"pom.xml": "<project/>"}, []string{"mvn -q test*", "mvn -q verify*"}, nil},
		{"gradle", map[string]string{"build.gradle.kts": ""}, []string{"./gradlew test*", "./gradlew build*"}, nil},
		{"make", map[string]string{"Makefile": "test:\n"}, []string{"make test*", "make build*", "make lint*"}, nil},
		{"nothing", map[string]string{"README.md": "x"}, nil, []string{"npm test*", "make test*"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := VerifyCommands(DetectSignals(mkProject(t, c.files)))
			for _, w := range c.want {
				assert.Contains(t, got, w)
			}
			for _, a := range c.absent {
				assert.NotContains(t, got, a)
			}
			assert.IsIncreasing(t, got)
		})
	}
}

func TestVerifyCommandsNeverInstall(t *testing.T) {
	all := Signals{}
	for _, s := range []string{"javascript", "typescript", "npm", "pnpm", "yarn", "bun", "golang", "python", "uv", "poetry", "pip", "rust", "cargo", "maven", "gradle", "make"} {
		all.add(s, ".")
	}
	for _, p := range VerifyCommands(all) {
		for _, bad := range []string{"install", "publish", " add", " ci", "sync*", "npx tsc", "bunx", " get", "deploy"} {
			assert.NotContains(t, p, bad, p)
		}
		assert.False(t, strings.HasPrefix(p, "*"), "anchored at the command: %s", p)
	}
}

// R3: the concrete commands oh runs at a checkpoint, by kind; each one is
// allowed by VerifyCommands (never an install).
func TestVerifyRuns(t *testing.T) {
	all := []string{"build", "test", "lint", "typecheck"}
	cases := []struct {
		name  string
		files map[string]string
		kinds []string
		want  []string
	}{
		{"go", map[string]string{"go.mod": "module x\n"}, all, []string{"go build ./...", "go test ./...", "go vet ./..."}},
		{"go default kinds", map[string]string{"go.mod": "module x\n"}, []string{"build", "test", "typecheck"}, []string{"go build ./...", "go test ./..."}},
		{"make target first", map[string]string{"go.mod": "module x\n", "Makefile": "test:\n\tgo test ./...\nVAR := 1\n"}, []string{"build", "test"},
			[]string{"go build ./...", "make test"}},
		{"pnpm scripts", map[string]string{"package.json": `{"scripts":{"test":"vitest run","build":"vite build"},"devDependencies":{"typescript":"6"}}`, "pnpm-lock.yaml": ""}, all,
			[]string{"pnpm run build", "pnpm test", "pnpm exec tsc --noEmit"}},
		{"npm init test", map[string]string{"package.json": `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1","typecheck":"tsc -b"}}`}, all,
			[]string{"npm run typecheck"}},
		{"python uv", map[string]string{"pyproject.toml": "[project]\nname = 'x'\ndependencies = ['pytest']\n", "uv.lock": ""}, all, []string{"uv run --no-sync pytest"}},
		{"rust", map[string]string{"Cargo.toml": "[package]\n"}, []string{"test"}, []string{"cargo test"}},
		{"gradle without wrapper", map[string]string{"build.gradle.kts": ""}, all, nil},
		{"gradle", map[string]string{"build.gradle.kts": "", "gradlew": "#!/bin/sh\n"}, []string{"test"}, []string{"./gradlew test"}},
		{"nothing", map[string]string{"README.md": "x"}, all, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := mkProject(t, c.files)
			sig := DetectSignals(dir)
			runs := VerifyRuns(dir, sig, ".", c.kinds)
			var got []string
			for _, r := range runs {
				got = append(got, r.String())
				allowed := false
				for _, p := range VerifyCommands(sig) {
					allowed = allowed || wildcardOK(p, r.String())
				}
				assert.True(t, allowed, "%s is not allowed by VerifyCommands", r)
			}
			assert.Equal(t, c.want, got)
		})
	}
}

// wildcardOK matches a VerifyCommands pattern (a trailing * only).
func wildcardOK(pattern, cmd string) bool {
	return strings.HasPrefix(cmd, strings.TrimSuffix(pattern, "*"))
}
