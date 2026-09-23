package sweep

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyNone(t *testing.T) {
	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyNone,
		ProjectPath: t.TempDir(),
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, VerifyNone, result.Strategy)
	assert.Empty(t, result.Steps)
}

func TestVerifyBuild_Success(t *testing.T) {
	// Create a minimal Go project that builds
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyBuild,
		ProjectPath: tmpDir,
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "build", result.Steps[0].Name)
	assert.True(t, result.Steps[0].Success)
}

func TestVerifyBuild_Failure(t *testing.T) {
	// Create a Go project with syntax errors
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() { this wont compile }\n"), 0o644))

	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyBuild,
		ProjectPath: tmpDir,
	})
	require.NoError(t, err) // Verify itself shouldn't error, but the step should fail
	assert.False(t, result.Success)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "build", result.Steps[0].Name)
	assert.False(t, result.Steps[0].Success)
	assert.NotEmpty(t, result.Steps[0].Output) // should contain the build error
}

func TestVerifyTests_Success(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc Add(a, b int) int { return a + b }\nfunc main() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(`package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("expected 3")
	}
}
`), 0o644))

	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyTests,
		ProjectPath: tmpDir,
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "tests", result.Steps[0].Name)
	assert.True(t, result.Steps[0].Success)
}

func TestVerifyTests_Failure(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(`package main

import "testing"

func TestFail(t *testing.T) {
	t.Fatal("intentional failure")
}
`), 0o644))

	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyTests,
		ProjectPath: tmpDir,
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Steps, 1)
	assert.False(t, result.Steps[0].Success)
}

func TestVerifyAll_StopsOnFirstFailure(t *testing.T) {
	// Build fails → tests should not be run
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() { broken }\n"), 0o644))

	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyAll,
		ProjectPath: tmpDir,
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	// Only build step should have run (stopped before tests)
	require.GreaterOrEqual(t, len(result.Steps), 1)
	assert.Equal(t, "build", result.Steps[0].Name)
	assert.False(t, result.Steps[0].Success)
	// Tests step should NOT be present
	for _, step := range result.Steps {
		assert.NotEqual(t, "tests", step.Name)
	}
}

func TestVerifyCustom(t *testing.T) {
	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyCustom,
		CustomCmd:   "echo hello",
		ProjectPath: t.TempDir(),
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "custom", result.Steps[0].Name)
	assert.True(t, result.Steps[0].Success)
	assert.Contains(t, result.Steps[0].Output, "hello")
}

func TestVerifyCustom_Missing(t *testing.T) {
	_, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyCustom,
		CustomCmd:   "",
		ProjectPath: t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--sweep-verify-cmd is required")
}

func TestVerifyCustom_Failure(t *testing.T) {
	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyCustom,
		CustomCmd:   "exit 1",
		ProjectPath: t.TempDir(),
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
}

func TestVerifyLint_SkipsIfNotInstalled(t *testing.T) {
	// This test depends on golangci-lint NOT being in PATH.
	// If it is installed, the test verifies it runs without error.
	result, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyLint,
		ProjectPath: t.TempDir(),
	})
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "lint", result.Steps[0].Name)
	// If golangci-lint is not installed, it should skip gracefully
	// If it IS installed, it may fail on empty dir but that's fine
}

func TestVerifyUnknownStrategy(t *testing.T) {
	_, err := Verify(context.Background(), VerifyOpts{
		Strategy:    VerifyStrategy("bogus"),
		ProjectPath: t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown verify strategy")
}
