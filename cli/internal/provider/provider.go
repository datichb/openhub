// Package provider handles LLM provider configuration, detection, and credential management.
package provider

// Name represents a supported LLM provider identifier.
type Name string

const (
	Bedrock       Name = "bedrock"
	Anthropic     Name = "anthropic"
	OpenRouter    Name = "openrouter"
	GithubCopilot Name = "github-copilot"
)

// AllProviders returns all supported provider names.
func AllProviders() []Name {
	return []Name{Bedrock, Anthropic, OpenRouter, GithubCopilot}
}

// Config holds non-secret provider configuration (stored in hub.toml or project DB).
type Config struct {
	AWSProfile string `mapstructure:"aws_profile" json:"aws_profile,omitempty"` // AWS profile name (bedrock)
	AWSRegion  string `mapstructure:"aws_region" json:"aws_region,omitempty"`   // AWS region (bedrock)
	AuthMode   string `mapstructure:"auth_mode" json:"auth_mode,omitempty"`     // "bearer" | "profile" | "env" (bedrock)
}

// EnvVar returns the environment variable name that opencode expects for a given provider.
func EnvVar(name Name) string {
	switch name {
	case Bedrock:
		return "AWS_BEARER_TOKEN_BEDROCK"
	case Anthropic:
		return "ANTHROPIC_API_KEY"
	case OpenRouter:
		return "OPENROUTER_API_KEY"
	case GithubCopilot:
		return "" // no env var needed, uses gh auth
	default:
		return ""
	}
}

// KeychainKey returns the keychain key name for storing provider credentials.
// If projectID is non-empty, returns the project-scoped key.
func KeychainKey(name Name, projectID string) string {
	base := ""
	switch name {
	case Bedrock:
		base = "bedrock-token"
	case Anthropic:
		base = "anthropic-api-key"
	case OpenRouter:
		base = "openrouter-api-key"
	case GithubCopilot:
		return "" // no secret needed
	default:
		return ""
	}

	if projectID != "" {
		return base + "-" + projectID
	}
	return base + "-default"
}

// Description returns a human-readable description of what credentials are needed.
func Description(name Name) string {
	switch name {
	case Bedrock:
		return "AWS profile (~/.aws/credentials) or bearer token"
	case Anthropic:
		return "API key (ANTHROPIC_API_KEY)"
	case OpenRouter:
		return "API key (OPENROUTER_API_KEY)"
	case GithubCopilot:
		return "GitHub Copilot access (gh auth)"
	default:
		return ""
	}
}
