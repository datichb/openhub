package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectStack_Go(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), []byte("test:\n\tgo test"), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "go", info.Language)
	assert.Equal(t, "go modules", info.PackageManager)
	assert.Equal(t, "make test", info.TestRunner)
}

func TestDetectStack_TypeScript_Bun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"vitest":"1.0"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "typescript", info.Language)
	assert.Equal(t, "bun", info.PackageManager)
	assert.Equal(t, "vitest", info.TestRunner)
}

func TestDetectStack_Python_Poetry(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.poetry]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "poetry.lock"), []byte(""), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "python", info.Language)
	assert.Equal(t, "poetry", info.PackageManager)
	assert.Equal(t, "pytest", info.TestRunner)
}

func TestDetectStack_Rust(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "rust", info.Language)
	assert.Equal(t, "cargo", info.PackageManager)
	assert.Equal(t, "cargo test", info.TestRunner)
}

func TestDetectStack_Docker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine"), 0o644))

	info := DetectStack(dir)
	assert.True(t, info.HasDocker)
}

func TestDetectStack_CI(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("on: push"), 0o644))

	info := DetectStack(dir)
	assert.True(t, info.HasCI)
}

func TestDetectStack_Empty(t *testing.T) {
	dir := t.TempDir()
	info := DetectStack(dir)
	assert.Equal(t, "", info.Language)
	assert.Equal(t, "", info.Framework)
}

func TestBuildContext(t *testing.T) {
	info := StackInfo{
		Language:       "go",
		PackageManager: "go modules",
		TestRunner:     "go test ./...",
		HasDocker:      true,
	}
	ctx := BuildContext(info)
	assert.Contains(t, ctx, "Language: go")
	assert.Contains(t, ctx, "Package manager: go modules")
	assert.Contains(t, ctx, "Test command: go test ./...")
	assert.Contains(t, ctx, "Docker: yes")
}

func TestBuildContext_Empty(t *testing.T) {
	ctx := BuildContext(StackInfo{})
	assert.Equal(t, "", ctx)
}

func TestDetectStack_NextJS(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0"},"devDependencies":{"jest":"29","typescript":"5"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "typescript", info.Language)
	assert.Equal(t, "yarn", info.PackageManager)
	assert.Equal(t, "Next.js", info.Framework)
	assert.Equal(t, "jest", info.TestRunner)
}

// QB3: Python frameworks and Rails are detected.
func TestDetectStack_PythonFrameworksAndRails(t *testing.T) {
	for file, cases := range map[string]map[string]string{
		"requirements.txt": {"Django>=4.2\n": "django", "fastapi\n": "fastapi", "Flask==3\n": "flask", "requests\n": ""},
	} {
		for content, want := range cases {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644))
			info := DetectStack(dir)
			assert.Equal(t, "python", info.Language, content)
			assert.Equal(t, want, info.Framework, content)
		}
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem \"rails\"\ngem \"rspec-rails\"\n"), 0o644))
	info := DetectStack(dir)
	assert.Equal(t, "ruby", info.Language)
	assert.Equal(t, "rails", info.Framework)
	assert.Equal(t, "rspec", info.TestRunner)
}

// C3.2: detection is multi-signal; the primary language is the first one
// (by priority) found at the root, the others are kept.
func TestDetectStack_MultiSignal(t *testing.T) {
	write := func(dir, rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	dir := t.TempDir()
	write(dir, "package.json", `{"workspaces":["apps/*"],"devDependencies":{"typescript":"5"}}`)
	write(dir, "pnpm-lock.yaml", "")
	write(dir, "apps/web/package.json", `{"dependencies":{"next":"15"},"devDependencies":{"vitest":"2"}}`)
	write(dir, "services/api/go.mod", "module api")
	info := DetectStack(dir)
	assert.Equal(t, "typescript", info.Language, "root manifest first")
	assert.Equal(t, []string{"typescript", "go"}, info.Languages)
	assert.Equal(t, "Next.js", info.Framework)
	assert.Equal(t, "pnpm", info.PackageManager)
	assert.Equal(t, "vitest", info.TestRunner)
	assert.True(t, info.Signals.Has("golang"))
	ctx := BuildContext(info)
	assert.Contains(t, ctx, "Language: typescript\nOther languages: go")

	dir = t.TempDir()
	write(dir, "web/package.json", `{"dependencies":{"vue":"3"}}`)
	write(dir, "api/go.mod", "module api")
	info = DetectStack(dir)
	assert.Equal(t, "go", info.Language, "no root manifest: priority order")
	assert.Equal(t, []string{"go", "javascript"}, info.Languages)
	assert.Equal(t, "Vue", info.Framework)

	dir = t.TempDir()
	write(dir, "pom.xml", "<artifactId>spring-boot-starter-web</artifactId>")
	info = DetectStack(dir)
	assert.Equal(t, "java", info.Language)
	assert.Equal(t, "Spring Boot", info.Framework)
	assert.Equal(t, "maven", info.PackageManager)
	assert.Equal(t, "mvn test", info.TestRunner)

	dir = t.TempDir()
	write(dir, "package.json", `{"devDependencies":{"eslint-config-next":"15","@testing-library/jest-dom":"6"}}`)
	info = DetectStack(dir)
	assert.Equal(t, "javascript", info.Language)
	assert.Empty(t, info.Framework)
	assert.Equal(t, "npm test", info.TestRunner)
}
