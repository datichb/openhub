package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A20 (piste T, T3): the shipped developer runs the project's Python tools
// (python3, its virtual environment, uv, poetry) but never installs in the
// system (pip outside a virtual environment, npm -g, gem install); A19: the
// hooks stay in force.
func TestDeveloperShellPermissions(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: agentSpec(t, repoHub(t), "developer"), Provider: "bedrock"})
	require.NoError(t, err)
	var shell []sessionspec.PermissionRule
	for _, a := range b.Spec.Agents {
		if a.ID == "developer" {
			for _, r := range a.Permissions {
				if r.Action == sessionspec.ActionShell {
					shell = append(shell, r)
				}
			}
		}
	}
	require.NotEmpty(t, shell)
	effect := func(cmd string) sessionspec.Effect {
		e, _ := evaluate(shell, cmd)
		return e
	}
	for _, cmd := range []string{
		"python3 -m pytest tests/ -v", "python3 --version", "python3 -m venv .venv", ".venv/bin/pip install -r requirements.txt",
		".venv/bin/python -m pytest", "venv/bin/pytest -q", "uv run pytest", "uv sync", "uv pip install -r requirements.txt", "poetry run pytest",
		"poetry install", "pip3 list", "pip show fastapi", "npm install", "npm test", "npx vitest run", "pnpm exec vitest", "go test ./...",
		"bundle exec rspec", "git commit -m 'feat: x'", "git checkout -b feat/x", "git stash list",
	} {
		assert.Equal(t, sessionspec.EffectAllow, effect(cmd), cmd)
	}
	for _, cmd := range []string{
		"pip install pytest", "pip3 install fastapi", "pip3 install --user fastapi", "python3 -m pip install fastapi",
		".venv/bin/pip install x --break-system-packages", "pip3 install fastapi --break-system-packages", "uv pip install x --system",
		"npm install -g typescript", "npm i -g x", "yarn global add x", "pnpm add -g x", "gem install rails", "npm link",
		"env sh -c 'rm -rf /'", "git checkout app/store.py", "git stash", "git commit --no-verify -m x",
	} {
		assert.Equal(t, sessionspec.EffectDeny, effect(cmd), cmd)
	}
}
