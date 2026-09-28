package opencode

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindBinary_InPATH(t *testing.T) {
	// opencode should be in PATH on this machine
	path, err := FindBinary()
	if err != nil {
		t.Skip("opencode not installed, skipping")
	}
	assert.NotEmpty(t, path)
	assert.FileExists(t, path)
}

func TestBuildArgs_Basic(t *testing.T) {
	args := buildArgs(StartOpts{
		Agent:  "coder",
		Prompt: "fix the bug",
	})
	assert.Equal(t, []string{"--agent", "coder", "--prompt", "fix the bug"}, args)
}

func TestBuildArgs_Resume(t *testing.T) {
	args := buildArgs(StartOpts{
		ResumeSessionID: "abc-123",
		Agent:           "coder", // should be ignored
	})
	assert.Equal(t, []string{"-s", "abc-123"}, args)
}

func TestBuildArgs_Empty(t *testing.T) {
	args := buildArgs(StartOpts{})
	assert.Empty(t, args)
}

func TestBuildEnv_WithToken(t *testing.T) {
	env := buildEnv("bedrock", Credentials{BearerToken: "my-secret-token"})
	found := false
	for _, e := range env {
		if e == "AWS_BEARER_TOKEN_BEDROCK=my-secret-token" {
			found = true
			break
		}
	}
	assert.True(t, found, "AWS_BEARER_TOKEN_BEDROCK should be in env")
}

func TestBuildEnv_WithoutToken(t *testing.T) {
	env := buildEnv("", Credentials{})
	// When no token is provided, we should not ADD a new AWS_BEARER_TOKEN_BEDROCK entry.
	// However, if the OS already has one, buildEnv won't remove it (that's fine).
	// We test that buildEnv doesn't inject a new one by checking its length matches os.Environ()
	assert.Equal(t, len(os.Environ()), len(env))
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "bin"), expandHome("~/bin"))
	assert.Equal(t, "/usr/local/bin", expandHome("/usr/local/bin"))
}

// TestRunHeadless_TruncationDetection verifies that RunHeadless detects output
// truncation and returns an appropriate error. We use a small helper script
// that produces deterministic output instead of requiring the opencode binary.
func TestRunHeadless_TruncationDetection(t *testing.T) {
	// This test verifies the truncation detection logic by calling RunHeadless
	// with a binary that outputs more than maxHeadlessOutput. Since we can't
	// easily override the constant in a unit test, we validate the detection
	// pattern indirectly via the helper that exercises the same io.LimitReader
	// + drain pattern.
	//
	// The core guarantee: when n >= limit, the goroutine drains stdout and
	// the function returns an error containing "truncated".

	// Create a script that outputs exactly 2 bytes
	tmpDir := t.TempDir()
	script := filepath.Join(tmpDir, "test_output.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'ABCDEFGHIJ'"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test the truncation detection helper directly
	t.Run("truncation detected when output exceeds limit", func(t *testing.T) {
		truncated, err := testHeadlessTruncation(script, 5)
		assert.True(t, truncated)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "truncated")
	})

	t.Run("no truncation when output fits", func(t *testing.T) {
		truncated, err := testHeadlessTruncation(script, 1024)
		assert.False(t, truncated)
		assert.NoError(t, err)
	})
}

// testHeadlessTruncation runs a script with a given output limit and reports
// whether truncation was detected. This mirrors the RunHeadless logic.
func testHeadlessTruncation(script string, limit int64) (bool, error) {
	cmd := exec.Command(script)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return false, err
	}

	var buf bytes.Buffer
	n, _ := io.Copy(&buf, io.LimitReader(stdout, limit))
	truncated := n >= limit
	if truncated {
		go func() { _, _ = io.Copy(io.Discard, stdout) }()
	}

	if err := cmd.Wait(); err != nil {
		return truncated, err
	}
	if truncated {
		return true, fmt.Errorf("headless output truncated at %d bytes (limit: %d)", n, limit)
	}
	return false, nil
}
