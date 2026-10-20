package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// testBinary is the path to the compiled oh binary for integration tests.
var testBinary string

func TestMain(m *testing.M) {
	// os.Exit skips the deferred calls: the code is returned once the
	// temporary folders are removed (runMain).
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	// The real hub and home are out of reach of the tests (the Go caches of
	// the user stay where they are, testenv pins them: the build below).
	finish := testenv.Isolate()

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

	// Create minimal hub content structure so the init gate passes
	hubDir := filepath.Join(testHome, ".oh", "hub")
	if err := os.MkdirAll(filepath.Join(hubDir, "agents"), 0o755); err != nil {
		panic("failed to create hub agents dir: " + err.Error())
	}
	if err := os.MkdirAll(filepath.Join(hubDir, "skills"), 0o755); err != nil {
		panic("failed to create hub skills dir: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(hubDir, ".version"), []byte("dev"), 0o644); err != nil {
		panic("failed to write hub version: " + err.Error())
	}

	// No background git maintenance (a commit, a fetch or a push starts a
	// detached `git maintenance run --auto` that keeps writing to
	// .git/objects while t.TempDir is removed: flaky TestMigrateLegacyWorkflows).
	return finish(gittest.Main(m))
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

// QB6: git runs no background maintenance in the tests (temp dir cleanup).
func TestGitMaintenanceIsOffInTests(t *testing.T) {
	out, err := exec.Command("git", "config", "--get", "maintenance.auto").Output()
	if err != nil || strings.TrimSpace(string(out)) != "false" {
		t.Fatalf("maintenance.auto = %q (%v)", out, err)
	}
}

func TestHelpOutput(t *testing.T) {
	stdout, _, exitCode := runOh(t, "--help")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "oh")
	assert.Contains(t, stdout, "run [workflow]")
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
	// Exit code 1 when a check fails (QB1), e.g. no container engine on the
	// test machine; 0 otherwise.
	if strings.Contains(stdout, "✗") {
		assert.Equal(t, 1, exitCode)
	} else {
		assert.Equal(t, 0, exitCode)
	}
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

// --- MCP command integration tests ---

func TestMCPStatusRuns(t *testing.T) {
	stdout, _, exitCode := runOh(t, "mcp", "status")
	assert.Equal(t, 0, exitCode)
	// Should display service table with known services
	assert.Contains(t, stdout, "Figma")
	assert.Contains(t, stdout, "GitLab")
}

func TestMCPEnableHub(t *testing.T) {
	_, _, exitCode := runOh(t, "mcp", "enable", "figma")
	assert.Equal(t, 0, exitCode)
}

func TestMCPDisableHub(t *testing.T) {
	_, _, exitCode := runOh(t, "mcp", "disable", "figma")
	assert.Equal(t, 0, exitCode)
}

func TestMCPResetRequiresProject(t *testing.T) {
	_, stderr, exitCode := runOh(t, "mcp", "reset", "figma")
	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr, "project")
}

func TestMCPInvalidService(t *testing.T) {
	_, stderr, exitCode := runOh(t, "mcp", "enable", "invalid-service")
	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr, "unknown service")
}

func TestMCPListRuns(t *testing.T) {
	stdout, _, exitCode := runOh(t, "mcp", "list")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "figma")
	assert.Contains(t, stdout, "gitlab")
}

func TestMCPListJSON(t *testing.T) {
	stdout, _, exitCode := runOh(t, "mcp", "list", "--json")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, `"name"`)
	assert.Contains(t, stdout, `"figma"`)
}

// U-05: exit codes seen by a script (0 success, 2 usage).
func TestExitCodesOfTheBinary(t *testing.T) {
	_, _, code := runOh(t, "version")
	assert.Equal(t, 0, code)
	_, stderr, code := runOh(t, "deploy", "--check")
	assert.Equal(t, 2, code, "a removed command is not a success")
	assert.Contains(t, stderr, "oh deploy")
	_, _, code = runOh(t, "sync")
	assert.Equal(t, 2, code)
	_, _, code = runOh(t, "version", "--bogus-flag")
	assert.Equal(t, 2, code, "unknown option")
	_, _, code = runOh(t, "bogus-command")
	assert.Equal(t, 2, code, "unknown command")
}

// U-02: a deprecated command still works and warns in the language of the
// hub; the main help does not list it, cobra prints no English sentence.
func TestDeprecatedCommandsOfTheBinary(t *testing.T) {
	stdout, _, code := runOh(t, "--help")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stdout, "oh start")
	assert.NotContains(t, stdout, "  audit")
	assert.NotContains(t, stdout, "  debug")
	assert.Contains(t, stdout, "mr create")

	// The deprecated command runs (here it stops at the missing GitLab write
	// access), after its warning that names the replacement.
	_, stderr, code := runOh(t, "review", "--publish")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "oh mr create")
	assert.NotContains(t, stderr, "is deprecated")

	_, stderr, code = runOh(t, "mr", "create")
	assert.Equal(t, 1, code)
	assert.NotContains(t, stderr, "oh mr create", "the replacement does not warn")
}
