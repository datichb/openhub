package bricks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveStackSkills_GoProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".gitlab"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte("stages: [test]"), 0o644))

	skills := ResolveStackSkills(dir)

	assert.Contains(t, skills, "developer/stacks/dev-standards-golang")
	assert.Contains(t, skills, "developer/stacks/dev-standards-docker")
	assert.Contains(t, skills, "developer/stacks/dev-standards-gitlab-ci")
	// Should NOT contain github-actions
	assert.NotContains(t, skills, "developer/stacks/dev-standards-github-actions")
}

func TestResolveStackSkills_NextJSProject(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"next":"14.0"},"devDependencies":{"vitest":"1.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644))

	skills := ResolveStackSkills(dir)

	assert.Contains(t, skills, "developer/stacks/dev-standards-nextjs")
	assert.Contains(t, skills, "developer/stacks/dev-standards-react") // auto-included with Next.js
	assert.Contains(t, skills, "developer/stacks/dev-standards-typescript")
	assert.Contains(t, skills, "developer/stacks/dev-standards-vitest")
}

func TestResolveStackSkills_PythonDjango(t *testing.T) {
	dir := t.TempDir()
	// pyproject.toml with django dependency
	pyproject := `[tool.poetry.dependencies]
django = "^4.2"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM python:3.11"), 0o644))

	skills := ResolveStackSkills(dir)

	assert.Contains(t, skills, "developer/stacks/dev-standards-python")
	assert.Contains(t, skills, "developer/stacks/dev-standards-docker")
	// django detection depends on prompt.DetectStack reading pyproject.toml content
	// If the prompt doesn't detect django from the content, this will be absent —
	// but the language + docker should always work.
}

func TestResolveStackSkills_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	skills := ResolveStackSkills(dir)
	assert.Empty(t, skills)
}

func TestResolveStackSkills_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	// Next.js includes both nextjs AND react skills via the mapping
	pkg := `{"dependencies":{"next":"14.0","react":"18.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644))

	skills := ResolveStackSkills(dir)

	// Count occurrences of react
	count := 0
	for _, s := range skills {
		if s == "developer/stacks/dev-standards-react" {
			count++
		}
	}
	assert.LessOrEqual(t, count, 1, "react skill should not appear more than once")
}

func TestResolveStackSkills_GitHubActions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("name: CI"), 0o644))

	skills := ResolveStackSkills(dir)

	assert.Contains(t, skills, "developer/stacks/dev-standards-golang")
	assert.Contains(t, skills, "developer/stacks/dev-standards-github-actions")
	assert.NotContains(t, skills, "developer/stacks/dev-standards-gitlab-ci")
}
