// Package opencode manages the opencode binary: locating, version checking, and launching.
package opencode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
)

// BinaryName is the name of the opencode binary.
const BinaryName = "opencode"

// Credentials holds provider-specific authentication tokens for opencode subprocesses.
// This mirrors platform.Credentials at the adapter boundary — the core opencode
// package intentionally does not import the platform package.
type Credentials struct {
	BearerToken string // Bedrock bearer token
	APIKey      string // Provider API key (Anthropic, OpenRouter)
	AWSProfile  string // AWS profile override
	AWSRegion   string // AWS region override
}

// StartOpts configures how opencode is launched.
type StartOpts struct {
	ProjectPath     string
	ProjectID       string
	Agent           string
	Prompt          string
	Provider        string
	Credentials     Credentials
	SessionTitle    string
	ResumeSessionID string
	Model           string   // Optional model override (e.g. "anthropic/claude-sonnet-4-20250514")
	Files           []string // Files to pre-attach to the session context
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
	env := buildEnv(opts.Provider, opts.Credentials)

	// Change to project directory
	if opts.ProjectPath != "" {
		if err := os.Chdir(opts.ProjectPath); err != nil {
			return fmt.Errorf("changing to project directory %s: %w", opts.ProjectPath, err)
		}
	}

	// exec replaces the current process (platform-specific implementation)
	return execReplace(bin, append([]string{BinaryName}, args...), env)
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
	cmd.Env = buildEnv(opts.Provider, opts.Credentials)

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
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	for _, f := range opts.Files {
		args = append(args, "--file", f)
	}
	if opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}

	args = append(args, opts.ExtraArgs...)
	return args
}

func buildEnv(provider string, creds Credentials) []string {
	env := os.Environ()

	switch provider {
	case "bedrock":
		if creds.BearerToken != "" {
			env = appendEnv(env, "AWS_BEARER_TOKEN_BEDROCK", creds.BearerToken)
		}
		if creds.AWSProfile != "" {
			env = appendEnv(env, "AWS_PROFILE", creds.AWSProfile)
		}
		if creds.AWSRegion != "" {
			env = appendEnv(env, "AWS_REGION", creds.AWSRegion)
		}
	case "anthropic":
		if creds.APIKey != "" {
			env = appendEnv(env, "ANTHROPIC_API_KEY", creds.APIKey)
		}
	case "openrouter":
		if creds.APIKey != "" {
			env = appendEnv(env, "OPENROUTER_API_KEY", creds.APIKey)
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

// HeadlessOpts configures a non-interactive opencode run.
type HeadlessOpts struct {
	ProjectPath string      // Working directory
	ProjectID   string      // Hub project ID (for env)
	Agent       string      // Agent to use (e.g. "brief-enricher")
	Prompt      string      // The prompt to send
	Format      string      // Output format: "" (default) or "json"
	Model       string      // Optional model override (provider/model)
	Files       []string    // Files to attach to the prompt
	ExtraArgs   []string    // Backend-specific passthrough arguments

	// Provider credentials — injected into the subprocess environment via
	// buildEnv(). Without these, headless runs rely on the parent shell's
	// environment, which may not contain keychain-stored tokens.
	Provider    string      // LLM provider name (bedrock, anthropic, openrouter)
	Credentials Credentials // Provider authentication tokens
}

// gracefulShutdownTimeout is the grace period between SIGTERM and SIGKILL
// when a headless run is cancelled via context.
const gracefulShutdownTimeout = 5 * time.Second

// RunHeadless executes opencode in non-interactive mode and captures output.
// Uses `opencode run` under the hood — no TUI, no stdin required.
//
// The context controls cancellation and timeouts: when ctx is done, the
// subprocess receives SIGTERM followed by SIGKILL after a grace period.
func RunHeadless(ctx context.Context, opts HeadlessOpts) (string, error) {
	bin, err := FindBinary()
	if err != nil {
		return "", fmt.Errorf("opencode binary not found: %w", err)
	}

	args := []string{"run"}
	if opts.Agent != "" {
		args = append(args, "--agent", opts.Agent)
	}
	if opts.Format != "" {
		args = append(args, "--format", opts.Format)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	for _, f := range opts.Files {
		args = append(args, "--file", f)
	}
	args = append(args, opts.ExtraArgs...)
	args = append(args, "--auto")
	args = append(args, opts.Prompt)

	cmd := exec.Command(bin, args...)
	if opts.ProjectPath != "" {
		cmd.Dir = opts.ProjectPath
	}

	// Inject provider credentials into the subprocess environment.
	cmd.Env = buildEnv(opts.Provider, opts.Credentials)

	// Capture output with a size limit to prevent OOM on large headless runs.
	const maxHeadlessOutput = 50 * 1024 * 1024 // 50 MB
	var buf bytes.Buffer

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("creating stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting opencode: %w", err)
	}

	// Read stdout and wait for completion in a goroutine so we can also
	// monitor context cancellation for graceful shutdown.
	type readResult struct {
		n         int64
		truncated bool
		copyErr   error
		waitErr   error
	}
	done := make(chan readResult, 1)
	go func() {
		var r readResult
		r.n, r.copyErr = io.Copy(&buf, io.LimitReader(stdout, maxHeadlessOutput))
		r.truncated = r.n >= maxHeadlessOutput
		if r.truncated {
			// Drain remaining stdout to unblock the subprocess pipe.
			go func() { _, _ = io.Copy(io.Discard, stdout) }()
		}
		r.waitErr = cmd.Wait()
		done <- r
	}()

	select {
	case r := <-done:
		// Normal completion — process exited before context was cancelled.
		if r.copyErr != nil {
			return buf.String(), fmt.Errorf("reading opencode output: %w", r.copyErr)
		}
		if r.waitErr != nil {
			combined := buf.String()
			if s := stderr.String(); s != "" {
				combined += "\n" + s
			}
			return combined, fmt.Errorf("opencode run failed: %w\noutput: %s",
				r.waitErr, strings.TrimSpace(combined))
		}
		if r.truncated {
			return buf.String(), fmt.Errorf("headless output truncated at %d bytes (limit: %d); result may be incomplete",
				r.n, maxHeadlessOutput)
		}
		return buf.String(), nil

	case <-ctx.Done():
		// Context cancelled — graceful shutdown: SIGTERM (Unix) / Kill (Windows) then SIGKILL.
		if cmd.Process != nil {
			_ = signalGraceful(cmd.Process)
		}
		select {
		case <-done:
			// Process exited after SIGTERM.
		case <-time.After(gracefulShutdownTimeout):
			// Force kill after grace period.
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-done
		}
		return buf.String(), ctx.Err()
	}
}
