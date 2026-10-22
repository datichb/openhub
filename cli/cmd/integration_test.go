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

// testBinary is the path to the compiled oh binary for integration tests, and
// testBinaryHome the HOME it runs with (a minimal hub, so the init gate passes).
var (
	testBinary     string
	testBinaryHome string
)

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
	testBinaryHome = testHome

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
	cmd.Env = append(os.Environ(), "HOME="+testBinaryHome)
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

// QB6: git runs no background maintenance in the tests (temp dir cleanup).
func TestGitMaintenanceIsOffInTests(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("git", "config", "--get", "maintenance.auto").Output()
	require.NoError(t, err, "reading maintenance.auto")
	assert.Equal(t, "false", strings.TrimSpace(string(out)), "maintenance.auto")
}

func TestDoctorRuns(t *testing.T) {
	t.Parallel()
	stdout, _, exitCode := runOh(t, "doctor")
	// Exit code 1 when a check fails (QB1), e.g. no container engine on the
	// test machine; 0 otherwise.
	if strings.Contains(stdout, "✗") {
		assert.Equal(t, 1, exitCode, "a failed check")
	} else {
		assert.Equal(t, 0, exitCode, "all checks passed")
	}
	for _, want := range []string{"OS / Architecture", "Go runtime", "git"} {
		assert.Contains(t, stdout, want)
	}
}

// What a script sees of the binary: the exit code (U-05: 0 success, 1 failure,
// 2 usage) and the two streams. The commands run in order on one hub (mcp
// enable, then mcp disable), and none of them starts a session.
func TestBinarySmoke(t *testing.T) {
	const nonZero = -1
	tests := []struct {
		name         string
		args         []string
		exit         int // nonZero: any failure
		stdout       []string
		stderr       []string
		stdoutAbsent []string
		stderrAbsent []string
	}{
		{name: "version", args: []string{"version"}, exit: 0, stdout: []string{"oh ", "commit:", "go:", "os/arch:", "dev"}},
		{name: "help", args: []string{"--help"}, exit: 0, stdout: []string{"oh", "  run ", "  mr ", "oh help --all"},
			stdoutAbsent: []string{"oh start", "  audit", "  debug", "mr create"}},
		{name: "help --all", args: []string{"help", "--all"}, exit: 0, stdout: []string{"oh", "run [workflow]", "mr create"},
			stdoutAbsent: []string{"oh start", "  audit", "  debug"}},
		{name: "completion bash", args: []string{"completion", "bash"}, exit: 0, stdout: []string{"bash"}},
		{name: "completion zsh", args: []string{"completion", "zsh"}, exit: 0, stdout: []string{"compdef"}},
		{name: "config path", args: []string{"config", "path"}, exit: 0, stdout: []string{".oh", "hub.toml"}},
		{name: "config list", args: []string{"config", "list"}, exit: 0, stdout: []string{""}},
		{name: "project list on an empty database", args: []string{"project", "list"}, exit: 0, stdout: []string{""}},
		{name: "status", args: []string{"status"}, exit: 0},
		{name: "unknown command", args: []string{"nonexistent-command-xyz"}, exit: 2, stderr: []string{""}},
		{name: "unknown option", args: []string{"version", "--bogus-flag"}, exit: 2},
		{name: "removed command: deploy", args: []string{"deploy", "--check"}, exit: 2, stderr: []string{"oh deploy"}},
		{name: "removed command: sync", args: []string{"sync"}, exit: 2},
		{name: "mcp status", args: []string{"mcp", "status"}, exit: 0, stdout: []string{"Figma", "GitLab"}},
		{name: "mcp enable", args: []string{"mcp", "enable", "figma"}, exit: 0},
		{name: "mcp disable", args: []string{"mcp", "disable", "figma"}, exit: 0},
		{name: "mcp reset needs a project", args: []string{"mcp", "reset", "figma"}, exit: nonZero, stderr: []string{"project"}},
		{name: "mcp unknown service", args: []string{"mcp", "enable", "invalid-service"}, exit: nonZero, stderr: []string{"unknown service"}},
		{name: "mcp list", args: []string{"mcp", "list"}, exit: 0, stdout: []string{"figma", "gitlab"}},
		{name: "mcp list as JSON", args: []string{"mcp", "list", "--json"}, exit: 0, stdout: []string{`"name"`, `"figma"`}},
		// A deprecated command runs (here it stops at the missing GitLab write
		// access) after a warning that names its replacement, in the language
		// of the hub; cobra prints no English sentence of its own (U-02).
		{name: "deprecated command", args: []string{"review", "--publish"}, exit: 1, stderr: []string{"oh mr create"}, stderrAbsent: []string{"is deprecated"}},
		{name: "its replacement does not warn", args: []string{"mr", "create"}, exit: 1, stderrAbsent: []string{"oh mr create"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exit := runOh(t, tt.args...)
			if tt.exit == nonZero {
				assert.NotEqual(t, 0, exit, "exit code")
			} else {
				assert.Equal(t, tt.exit, exit, "exit code")
			}
			for _, want := range tt.stdout {
				assert.Contains(t, stdout, want, "stdout")
			}
			for _, want := range tt.stderr {
				assert.Contains(t, stderr, want, "stderr")
			}
			if len(tt.stdout) > 0 {
				assert.NotEmpty(t, stdout, "stdout")
			}
			if len(tt.stderr) > 0 {
				assert.NotEmpty(t, stderr, "stderr")
			}
			for _, absent := range tt.stdoutAbsent {
				assert.NotContains(t, stdout, absent, "stdout")
			}
			for _, absent := range tt.stderrAbsent {
				assert.NotContains(t, stderr, absent, "stderr")
			}
		})
	}
}
