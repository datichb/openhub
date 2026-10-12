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
