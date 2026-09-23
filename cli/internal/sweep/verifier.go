package sweep

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// VerifyStrategy selects the post-sweep verification approach.
type VerifyStrategy string

const (
	VerifyNone   VerifyStrategy = "none"
	VerifyTests  VerifyStrategy = "tests"
	VerifyLint   VerifyStrategy = "lint"
	VerifyBuild  VerifyStrategy = "build"
	VerifyAll    VerifyStrategy = "all"
	VerifyCustom VerifyStrategy = "custom"
)

// ValidVerifyStrategies returns the list of valid verification strategies.
func ValidVerifyStrategies() []VerifyStrategy {
	return []VerifyStrategy{VerifyNone, VerifyTests, VerifyLint, VerifyBuild, VerifyAll, VerifyCustom}
}

// IsValidVerifyStrategy returns true if s is a recognized verification strategy.
func IsValidVerifyStrategy(s VerifyStrategy) bool {
	for _, v := range ValidVerifyStrategies() {
		if v == s {
			return true
		}
	}
	return false
}

// VerifyOpts configures post-sweep verification.
type VerifyOpts struct {
	Strategy    VerifyStrategy
	CustomCmd   string // Shell command for VerifyCustom
	ProjectPath string
}

// VerifyResult holds the overall verification outcome.
type VerifyResult struct {
	Strategy VerifyStrategy
	Success  bool
	Output   string // Combined stdout+stderr
	Steps    []VerifyStepResult
}

// VerifyStepResult holds the outcome of a single verification step.
type VerifyStepResult struct {
	Name    string // "build", "tests", "lint", "custom"
	Success bool
	Output  string
}

// Verify runs post-sweep verification on the project.
func Verify(ctx context.Context, opts VerifyOpts) (*VerifyResult, error) {
	result := &VerifyResult{Strategy: opts.Strategy}

	switch opts.Strategy {
	case VerifyNone:
		result.Success = true
		return result, nil

	case VerifyBuild:
		step := runVerifyStep(ctx, opts.ProjectPath, "build", "go", "build", "./...")
		result.Steps = append(result.Steps, step)
		result.Success = step.Success
		result.Output = step.Output

	case VerifyTests:
		step := runVerifyStep(ctx, opts.ProjectPath, "tests", "go", "test", "./...")
		result.Steps = append(result.Steps, step)
		result.Success = step.Success
		result.Output = step.Output

	case VerifyLint:
		step := runLintStep(ctx, opts.ProjectPath)
		result.Steps = append(result.Steps, step)
		result.Success = step.Success
		result.Output = step.Output

	case VerifyAll:
		steps := []struct {
			name string
			args []string
		}{
			{"build", []string{"go", "build", "./..."}},
			{"tests", []string{"go", "test", "./..."}},
		}

		result.Success = true
		var outputs []string

		for _, s := range steps {
			step := runVerifyStep(ctx, opts.ProjectPath, s.name, s.args[0], s.args[1:]...)
			result.Steps = append(result.Steps, step)
			outputs = append(outputs, step.Output)
			if !step.Success {
				result.Success = false
				break // Stop on first failure
			}
		}

		// Add lint if previous steps passed
		if result.Success {
			lintStep := runLintStep(ctx, opts.ProjectPath)
			result.Steps = append(result.Steps, lintStep)
			outputs = append(outputs, lintStep.Output)
			if !lintStep.Success {
				result.Success = false
			}
		}

		result.Output = strings.Join(outputs, "\n---\n")

	case VerifyCustom:
		if opts.CustomCmd == "" {
			return nil, fmt.Errorf("--sweep-verify-cmd is required with --sweep-verify=custom")
		}
		step := runVerifyStep(ctx, opts.ProjectPath, "custom", "sh", "-c", opts.CustomCmd)
		result.Steps = append(result.Steps, step)
		result.Success = step.Success
		result.Output = step.Output

	default:
		return nil, fmt.Errorf("unknown verify strategy: %q", opts.Strategy)
	}

	return result, nil
}

// runVerifyStep executes a single verification command.
func runVerifyStep(ctx context.Context, dir, name, bin string, args ...string) VerifyStepResult {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()
	output := strings.TrimSpace(combined.String())

	return VerifyStepResult{
		Name:    name,
		Success: err == nil,
		Output:  output,
	}
}

// runLintStep runs golangci-lint if available, otherwise returns a skip result.
func runLintStep(ctx context.Context, dir string) VerifyStepResult {
	// Check if golangci-lint is available
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return VerifyStepResult{
			Name:    "lint",
			Success: true,
			Output:  "golangci-lint not found, skipping lint step",
		}
	}
	return runVerifyStep(ctx, dir, "lint", "golangci-lint", "run", "./...")
}
