package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectionResult holds the result of detecting provider credentials on the system.
type DetectionResult struct {
	Provider  Name
	Available bool
	Source    string // "env", "keychain", "aws-profile", "gh-auth"
	Details   string // e.g., "profile default, region eu-west-1"
}

// Detect checks if credentials for the given provider are available on the system.
// It checks environment variables, AWS config files, and gh CLI auth status.
// It does NOT check the keychain (caller should do that separately if needed).
func Detect(name Name) DetectionResult {
	switch name {
	case Bedrock:
		return detectBedrock()
	case Anthropic:
		return detectAnthropic()
	case OpenRouter:
		return detectOpenRouter()
	case GithubCopilot:
		return detectGithubCopilot()
	default:
		return DetectionResult{Provider: name, Available: false}
	}
}

// DetectAll checks all providers and returns results.
func DetectAll() []DetectionResult {
	var results []DetectionResult
	for _, p := range AllProviders() {
		results = append(results, Detect(p))
	}
	return results
}

func detectBedrock() DetectionResult {
	r := DetectionResult{Provider: Bedrock}

	// Check env vars first
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		r.Available = true
		r.Source = "env"
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
		}
		if region != "" {
			r.Details = "region " + region
		}
		return r
	}

	// Check bearer token env var
	if os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "" {
		r.Available = true
		r.Source = "env"
		r.Details = "bearer token"
		return r
	}

	// Check AWS credentials file
	home, err := os.UserHomeDir()
	if err == nil {
		credFile := filepath.Join(home, ".aws", "credentials")
		if _, err := os.Stat(credFile); err == nil {
			r.Available = true
			r.Source = "aws-profile"
			profile := os.Getenv("AWS_PROFILE")
			if profile == "" {
				profile = "default"
			}
			region := os.Getenv("AWS_REGION")
			if region == "" {
				region = os.Getenv("AWS_DEFAULT_REGION")
			}
			details := "profile " + profile
			if region != "" {
				details += ", region " + region
			}
			r.Details = details
			return r
		}
	}

	return r
}

func detectAnthropic() DetectionResult {
	r := DetectionResult{Provider: Anthropic}

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		r.Available = true
		r.Source = "env"
		// Show masked key for confirmation
		if len(key) > 8 {
			r.Details = key[:4] + "..." + key[len(key)-4:]
		}
		return r
	}

	return r
}

func detectOpenRouter() DetectionResult {
	r := DetectionResult{Provider: OpenRouter}

	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		r.Available = true
		r.Source = "env"
		if len(key) > 8 {
			r.Details = key[:4] + "..." + key[len(key)-4:]
		}
		return r
	}

	return r
}

func detectGithubCopilot() DetectionResult {
	r := DetectionResult{Provider: GithubCopilot}

	// Check if gh CLI is authenticated
	cmd := exec.Command("gh", "auth", "status")
	output, err := cmd.CombinedOutput()
	if err == nil {
		r.Available = true
		r.Source = "gh-auth"
		// Extract account info from output
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Logged in to") {
				r.Details = strings.TrimSpace(line)
				break
			}
		}
		return r
	}

	// Check GITHUB_TOKEN env var
	if os.Getenv("GITHUB_TOKEN") != "" {
		r.Available = true
		r.Source = "env"
		r.Details = "GITHUB_TOKEN"
		return r
	}

	return r
}
