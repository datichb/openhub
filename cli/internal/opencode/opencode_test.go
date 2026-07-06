package opencode

import (
	"os"
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
	env := buildEnv(StartOpts{BearerToken: "my-secret-token"})
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
	env := buildEnv(StartOpts{})
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
