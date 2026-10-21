package bricks

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{"go with docker and gitlab-ci, not github-actions", map[string]string{
			"go.mod":         "module example",
			"Dockerfile":     "FROM alpine",
			".gitlab-ci.yml": "stages: [test]",
		}, []string{p + "docker", p + "gitlab-ci", p + "golang"}},
		{"go with github-actions, not gitlab-ci", map[string]string{
			"go.mod":                   "module example",
			".github/workflows/ci.yml": "name: CI",
		}, []string{p + "github-actions", p + "golang"}},
		{"next.js brings react, typescript and vitest", map[string]string{
			"package.json":  `{"dependencies":{"next":"14.0"},"devDependencies":{"vitest":"1.0"}}`,
			"tsconfig.json": "{}",
		}, []string{p + "nextjs", p + "react", p + "typescript", p + "vitest"}},
		{"next.js and react declared: react appears once", map[string]string{
			"package.json": `{"dependencies":{"next":"14.0","react":"18.0"}}`,
		}, []string{p + "nextjs", p + "react"}},
		{"QB3: django detected from pyproject.toml", map[string]string{
			"pyproject.toml": "[tool.poetry.dependencies]\ndjango = \"^4.2\"\n",
			"Dockerfile":     "FROM python:3.11",
		}, []string{p + "django", p + "docker", p + "python"}},
		{"QB3: fastapi detected from requirements.txt", map[string]string{
			"requirements.txt": "fastapi==0.110\nuvicorn\n",
		}, []string{p + "fastapi", p + "python"}},
		{"QB3: rails detected from the Gemfile", map[string]string{
			"Gemfile": "source 'https://rubygems.org'\ngem 'rails', '~> 7.1'\n",
		}, []string{p + "rails"}},
		{"empty project", map[string]string{}, nil},
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
	rn := mkProject(t, map[string]string{
		"package.json":  `{"dependencies":{"react-native":"0.76","react":"18"},"devDependencies":{"jest":"29","typescript":"5"}}`,
		"App.tsx":       "",
		"tsconfig.json": "{}",
	})
	assert.False(t, HasFrontend(rn), "a React Native app alone is not a web frontend")
}
