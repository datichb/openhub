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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0"},"devDependencies":{"jest":"29"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0o644))

	info := DetectStack(dir)
	assert.Equal(t, "typescript", info.Language)
	assert.Equal(t, "yarn", info.PackageManager)
	assert.Equal(t, "Next.js", info.Framework)
	assert.Equal(t, "jest", info.TestRunner)
}
