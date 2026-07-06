package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBinary is the path to the compiled oh binary for integration tests.
var testBinary string

func TestMain(m *testing.M) {
	// Build the binary into a temp directory
	tmpDir, err := os.MkdirTemp("", "oh-integration-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	testBinary = filepath.Join(tmpDir, "oh")
	buildCmd := exec.Command("go", "build", "-o", testBinary, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		panic("failed to build oh binary: " + err.Error() + "\n" + string(out))
	}

	// Set HOME to a temp dir so we don't interfere with real config
	testHome, err := os.MkdirTemp("", "oh-home-*")
	if err != nil {
		panic("failed to create temp home: " + err.Error())
	}
	defer os.RemoveAll(testHome)
	os.Setenv("HOME", testHome)

	os.Exit(m.Run())
}

func runOh(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run oh: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestVersionOutput(t *testing.T) {
	stdout, _, exitCode := runOh(t, "version")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "oh ")
	assert.Contains(t, stdout, "commit:")
	assert.Contains(t, stdout, "go:")
	assert.Contains(t, stdout, "os/arch:")
}

func TestHelpOutput(t *testing.T) {
	stdout, _, exitCode := runOh(t, "--help")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "oh")
	assert.Contains(t, stdout, "opencode")
}

func TestCompletionBash(t *testing.T) {
	stdout, _, exitCode := runOh(t, "completion", "bash")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "bash")
}

func TestCompletionZsh(t *testing.T) {
	stdout, _, exitCode := runOh(t, "completion", "zsh")
	assert.Equal(t, 0, exitCode)
	assert.NotEmpty(t, stdout)
}

func TestDoctorRuns(t *testing.T) {
	stdout, _, exitCode := runOh(t, "doctor")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "OS / Architecture")
	assert.Contains(t, stdout, "Go runtime")
	assert.Contains(t, stdout, "git")
}

func TestConfigPath(t *testing.T) {
	stdout, _, exitCode := runOh(t, "config", "path")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, ".oh")
	assert.Contains(t, stdout, "hub.toml")
}

func TestConfigList(t *testing.T) {
	stdout, _, exitCode := runOh(t, "config", "list")
	assert.Equal(t, 0, exitCode)
	// With defaults, should show something (or "Aucune configuration" message)
	assert.NotEmpty(t, stdout)
}

func TestProjectListEmpty(t *testing.T) {
	stdout, _, exitCode := runOh(t, "project", "list")
	assert.Equal(t, 0, exitCode)
	// Empty DB — should show "aucun projet" or similar, or just be empty
	assert.NotEmpty(t, stdout)
}

func TestStatusRuns(t *testing.T) {
	_, _, exitCode := runOh(t, "status")
	assert.Equal(t, 0, exitCode)
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, exitCode := runOh(t, "nonexistent-command-xyz")
	assert.NotEqual(t, 0, exitCode)
	require.NotEmpty(t, stderr)
}

func TestVersionFlag(t *testing.T) {
	stdout, _, exitCode := runOh(t, "version")
	assert.Equal(t, 0, exitCode)
	// Should contain version string (dev when built without ldflags)
	assert.Contains(t, stdout, "dev")
}
