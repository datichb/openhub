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

// BedrockRegion pairs a region code with a human-readable label.
type BedrockRegion struct {
	Code  string // e.g. "us-east-1"
	Label string // e.g. "us-east-1 (N. Virginia)"
}

// BedrockRegions lists AWS regions that support the Bedrock runtime API.
// Source: https://docs.aws.amazon.com/general/latest/gr/bedrock.html
var BedrockRegions = []BedrockRegion{
	{"us-east-1", "us-east-1 (N. Virginia)"},
	{"us-east-2", "us-east-2 (Ohio)"},
	{"us-west-2", "us-west-2 (Oregon)"},
	{"eu-central-1", "eu-central-1 (Frankfurt)"},
	{"eu-west-1", "eu-west-1 (Ireland)"},
	{"eu-west-3", "eu-west-3 (Paris)"},
	{"eu-north-1", "eu-north-1 (Stockholm)"},
	{"ap-southeast-1", "ap-southeast-1 (Singapore)"},
	{"ap-southeast-2", "ap-southeast-2 (Sydney)"},
	{"ap-northeast-1", "ap-northeast-1 (Tokyo)"},
	{"ap-northeast-2", "ap-northeast-2 (Seoul)"},
	{"ap-northeast-3", "ap-northeast-3 (Osaka)"},
	{"ap-south-1", "ap-south-1 (Mumbai)"},
	{"ap-south-2", "ap-south-2 (Hyderabad)"},
	{"sa-east-1", "sa-east-1 (São Paulo)"},
	{"ca-central-1", "ca-central-1 (Canada)"},
}

// BedrockRegionLabels returns display labels suitable for tview dropdown options.
func BedrockRegionLabels() []string {
	labels := make([]string, len(BedrockRegions))
	for i, r := range BedrockRegions {
		labels[i] = r.Label
	}
	return labels
}

// BedrockRegionIndex returns the index of region in BedrockRegions, or -1 if
// the region code is not in the known list.
func BedrockRegionIndex(region string) int {
	for i, r := range BedrockRegions {
		if r.Code == region {
			return i
		}
	}
	return -1
}

// Config holds non-secret provider configuration (stored in hub.toml or project DB).
type Config struct {
	AWSProfile string `mapstructure:"aws_profile" json:"aws_profile,omitempty"` // AWS profile name (bedrock)
	AWSRegion  string `mapstructure:"aws_region" json:"aws_region,omitempty"`   // AWS region (bedrock)
	AuthMode   string `mapstructure:"auth_mode" json:"auth_mode,omitempty"`     // "bearer" | "profile" | "env" (bedrock)
}

// EnvVar returns the environment variable name of the key of a provider.
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
		base = "openhub.provider.bedrock.token"
	case Anthropic:
		base = "openhub.provider.anthropic.token"
	case OpenRouter:
		base = "openhub.provider.openrouter.token"
	case GithubCopilot:
		return "" // no secret needed
	default:
		return ""
	}

	if projectID != "" {
		return base + "." + projectID
	}
	return base
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
