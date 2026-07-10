// Package opencode manages the opencode binary: locating, version checking, and launching.
package opencode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/datichb/openhub/cli/internal/config"
)

// BinaryName is the name of the opencode binary.
const BinaryName = "opencode"

// StartOpts configures how opencode is launched.
type StartOpts struct {
	ProjectPath     string
	ProjectID       string
	Agent           string
	Prompt          string
	Provider        string
	BearerToken     string // bedrock bearer token (legacy, still supported)
	APIKey          string // provider API key (anthropic, openrouter)
	AWSProfile      string // AWS profile override
	AWSRegion       string // AWS region override
	SessionTitle    string
	ResumeSessionID string
	ExtraArgs       []string
}

// FindBinary locates the opencode binary.
// Priority: 1) managed install in ~/.oh/bin/ 2) PATH lookup
func FindBinary() (string, error) {
	// Check managed install
	cfg, _ := config.Load()
	if cfg != nil && cfg.Opencode.InstallDir != "" {
		installDir := expandHome(cfg.Opencode.InstallDir)
		managed := filepath.Join(installDir, BinaryName)
		if _, err := os.Stat(managed); err == nil {
			return managed, nil
		}
		// Also check versioned binary
		if cfg.Opencode.Version != "" && cfg.Opencode.Version != "latest" {
			versioned := filepath.Join(installDir, BinaryName+"-"+cfg.Opencode.Version)
			if _, err := os.Stat(versioned); err == nil {
				return versioned, nil
			}
		}
	}

	// Fallback to PATH
	path, err := exec.LookPath(BinaryName)
	if err != nil {
		return "", fmt.Errorf("opencode not found in PATH or ~/.oh/bin/")
	}
	return path, nil
}

// Version returns the version of the installed opencode binary.
func Version() (string, error) {
	bin, err := FindBinary()
	if err != nil {
		return "", err
	}

	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("running opencode --version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Exec replaces the current process with opencode (unix exec).
// This never returns on success.
func Exec(opts StartOpts) error {
	bin, err := FindBinary()
	if err != nil {
		return err
	}

	args := buildArgs(opts)
	env := buildEnv(opts)

	// Change to project directory
	if opts.ProjectPath != "" {
		if err := os.Chdir(opts.ProjectPath); err != nil {
			return fmt.Errorf("changing to project directory %s: %w", opts.ProjectPath, err)
		}
	}

	// exec replaces the current process
	return syscall.Exec(bin, append([]string{BinaryName}, args...), env)
}

// Run starts opencode as a subprocess (useful for testing or when we need to wait).
func Run(opts StartOpts) error {
	bin, err := FindBinary()
	if err != nil {
		return err
	}

	args := buildArgs(opts)

	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if opts.ProjectPath != "" {
		cmd.Dir = opts.ProjectPath
	}

	// Set env
	cmd.Env = buildEnv(opts)

	return cmd.Run()
}

func buildArgs(opts StartOpts) []string {
	var args []string

	if opts.ResumeSessionID != "" {
		args = append(args, "-s", opts.ResumeSessionID)
		return args
	}

	if opts.Agent != "" {
		args = append(args, "--agent", opts.Agent)
	}
	if opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}

	args = append(args, opts.ExtraArgs...)
	return args
}

func buildEnv(opts StartOpts) []string {
	env := os.Environ()

	switch opts.Provider {
	case "bedrock":
		if opts.BearerToken != "" {
			env = appendEnv(env, "AWS_BEARER_TOKEN_BEDROCK", opts.BearerToken)
		}
		if opts.AWSProfile != "" {
			env = appendEnv(env, "AWS_PROFILE", opts.AWSProfile)
		}
		if opts.AWSRegion != "" {
			env = appendEnv(env, "AWS_REGION", opts.AWSRegion)
		}
	case "anthropic":
		if opts.APIKey != "" {
			env = appendEnv(env, "ANTHROPIC_API_KEY", opts.APIKey)
		}
	case "openrouter":
		if opts.APIKey != "" {
			env = appendEnv(env, "OPENROUTER_API_KEY", opts.APIKey)
		}
		// github-copilot: no env injection needed (relies on gh auth)
	}

	return env
}

func appendEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
