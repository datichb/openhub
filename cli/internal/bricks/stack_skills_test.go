package bricks

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644))

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
	assert.Contains(t, skills, "developer/stacks/dev-standards-django", "QB3: django detected from pyproject.toml")
}

// QB3: FastAPI (requirements.txt) and Rails (Gemfile) projects get their
// stack skills.
func TestResolveStackSkills_FastAPIAndRails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.110\nuvicorn\n"), 0o644))
	skills := ResolveStackSkills(dir)
	assert.Contains(t, skills, "developer/stacks/dev-standards-python")
	assert.Contains(t, skills, "developer/stacks/dev-standards-fastapi")

	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\ngem 'rails', '~> 7.1'\n"), 0o644))
	assert.Equal(t, []string{"developer/stacks/dev-standards-rails"}, ResolveStackSkills(dir))
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

const stacksDir = "../../../skills/developer/stacks"

// C3.2: every mapped ref is a shipped stack skill, and every stack skill is
// reachable from a signal.
func TestStackSkillMapping_MatchesCatalog(t *testing.T) {
	entries, err := os.ReadDir(stacksDir)
	require.NoError(t, err)
	shipped := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			shipped["developer/stacks/"+strings.TrimSuffix(e.Name(), ".md")] = true
		}
	}
	require.NotEmpty(t, shipped)
	mapped := map[string]bool{}
	for signal, refs := range stackSkillMapping {
		for _, ref := range refs {
			assert.True(t, shipped[ref], "signal %s maps to a missing skill %s", signal, ref)
			mapped[ref] = true
		}
	}
	for ref := range shipped {
		assert.True(t, mapped[ref], "stack skill %s is reachable from no signal", ref)
	}
}

func TestResolveStackSkills_Projects(t *testing.T) {
	const p = "developer/stacks/dev-standards-"
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"monorepo", map[string]string{
			"apps/web/package.json": `{"dependencies":{"react":"19","react-dom":"19"},"devDependencies":{"typescript":"5","vitest":"2"}}`,
			"services/api/go.mod":   "module api",
		}, []string{p + "golang", p + "react", p + "typescript", p + "vitest"}},
		{"react native", map[string]string{
			"package.json":  `{"dependencies":{"react-native":"0.76","react":"18"},"devDependencies":{"jest":"29","typescript":"5"}}`,
			"App.tsx":       "",
			"tsconfig.json": "{}",
		}, []string{p + "jest", p + "react-native", p + "typescript"}},
		{"spring boot maven", map[string]string{
			"pom.xml":      "<parent><artifactId>spring-boot-starter-parent</artifactId></parent>",
			"compose.yaml": "",
		}, []string{p + "docker", p + "java", p + "springboot"}},
		{"android", map[string]string{
			"settings.gradle.kts":              `include(":app")`,
			"app/build.gradle.kts":             `plugins { id("com.android.application"); kotlin("android") }`,
			"app/src/main/AndroidManifest.xml": "<manifest/>",
		}, []string{p + "kotlin"}},
		{"false positives", map[string]string{
			"package.json": `{"devDependencies":{"eslint-config-next":"15","@testing-library/jest-dom":"6","prettier-plugin-svelte":"3"}}`,
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveStackSkills(mkProject(t, c.files))
			assert.Equal(t, c.want, got)
			assert.True(t, sort.StringsAreSorted(got))
		})
	}
	rn := mkProject(t, cases[1].files)
	assert.False(t, HasFrontend(rn), "a React Native app alone is not a web frontend")
}
