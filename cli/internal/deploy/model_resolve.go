package deploy

import (
	"strings"
)

// ModelOverrides holds per-agent and per-family model overrides at a single cascade level.
// Used both for hub-level (from hub.toml) and project-level (from DB) overrides.
type ModelOverrides struct {
	Default  string            // global model at this level
	Families map[string]string // family name → model
	Agents   map[string]string // agent-id → model
}

// ResolveAgentModel resolves the effective model for a given agent using the 7-level cascade.
//
// Cascade priority (first match wins):
//  1. Project-level agent override
//  2. Project-level family override
//  3. Project-level global model
//  4. Hub-level agent override
//  5. Hub-level family override
//  6. Hub-level global model
//  7. Agent frontmatter floor
//
// Arguments:
//   - agentID: the agent identifier (e.g., "reviewer")
//   - family: the agent's family derived from directory (e.g., "quality")
//   - projectOverrides: model overrides at the project level (nil if none)
//   - hubOverrides: model overrides at the hub level (nil if none)
//   - frontmatterModel: the model declared in the agent's frontmatter (may be empty)
//   - provider: the resolved provider for this project (e.g., "bedrock", "anthropic")
//
// Returns the normalized model string for opencode.json, or "" if no model is defined at any level.
func ResolveAgentModel(agentID, family string, projectOverrides, hubOverrides *ModelOverrides, frontmatterModel, provider string) string {
	// Walk the cascade: first non-empty match wins
	resolved := ""

	// Level 1: Project-level agent override
	if projectOverrides != nil && projectOverrides.Agents != nil {
		if m, ok := projectOverrides.Agents[agentID]; ok && m != "" {
			resolved = m
			goto normalize
		}
	}

	// Level 2: Project-level family override
	if projectOverrides != nil && projectOverrides.Families != nil && family != "" {
		if m, ok := projectOverrides.Families[family]; ok && m != "" {
			resolved = m
			goto normalize
		}
	}

	// Level 3: Project-level global model
	if projectOverrides != nil && projectOverrides.Default != "" {
		resolved = projectOverrides.Default
		goto normalize
	}

	// Level 4: Hub-level agent override
	if hubOverrides != nil && hubOverrides.Agents != nil {
		if m, ok := hubOverrides.Agents[agentID]; ok && m != "" {
			resolved = m
			goto normalize
		}
	}

	// Level 5: Hub-level family override
	if hubOverrides != nil && hubOverrides.Families != nil && family != "" {
		if m, ok := hubOverrides.Families[family]; ok && m != "" {
			resolved = m
			goto normalize
		}
	}

	// Level 6: Hub-level global model
	if hubOverrides != nil && hubOverrides.Default != "" {
		resolved = hubOverrides.Default
		goto normalize
	}

	// Level 7: Agent frontmatter floor
	if frontmatterModel != "" {
		resolved = frontmatterModel
		goto normalize
	}

	return ""

normalize:
	if provider == "" {
		return resolved
	}
	return NormalizeModelForProvider(resolved, provider)
}

// NormalizeModelForProvider converts a model identifier to the format expected by opencode
// for the given provider. The input model may be in any of these forms:
//   - Short name: "claude-sonnet-4-5"
//   - Provider-prefixed: "anthropic/claude-sonnet-4-5"
//   - Already normalized: "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"
//
// The function extracts the short model name and re-formats it for the target provider.
func NormalizeModelForProvider(model, provider string) string {
	if model == "" || provider == "" {
		return model
	}

	// Extract the short model name (strip any existing provider prefix)
	shortName := extractShortModelName(model)
	if shortName == "" {
		return model // cannot parse, return as-is
	}

	switch provider {
	case "anthropic":
		return "anthropic/" + shortName
	case "bedrock":
		return formatBedrockModel(shortName)
	case "github-copilot":
		return formatGithubCopilotModel(shortName)
	case "openrouter":
		return "anthropic/" + shortName // openrouter uses anthropic/ prefix
	default:
		// Unknown provider: use anthropic/ prefix as safe default
		return "anthropic/" + shortName
	}
}

// extractShortModelName strips provider prefixes and version suffixes to get the canonical short name.
// Examples:
//   - "anthropic/claude-sonnet-4-5" → "claude-sonnet-4-5"
//   - "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0" → "claude-sonnet-4-5"
//   - "claude-sonnet-4-5" → "claude-sonnet-4-5"
//   - "github-copilot/claude-sonnet-4.5" → "claude-sonnet-4-5"
func extractShortModelName(model string) string {
	// Handle bedrock format: "amazon-bedrock/anthropic.claude-xxx-YYYYMMDD-vN:M"
	if strings.HasPrefix(model, "amazon-bedrock/") {
		after := strings.TrimPrefix(model, "amazon-bedrock/")
		// Remove "anthropic." prefix
		after = strings.TrimPrefix(after, "anthropic.")
		// Remove date-version suffix (-YYYYMMDD-vN:M)
		after = stripBedrockVersionSuffix(after)
		return after
	}

	// Handle github-copilot format: "github-copilot/claude-sonnet-4.5"
	if strings.HasPrefix(model, "github-copilot/") {
		after := strings.TrimPrefix(model, "github-copilot/")
		// Normalize dots to dashes in version (4.5 → 4-5)
		after = normalizeModelDots(after)
		return after
	}

	// Handle anthropic/ or openrouter/ prefix
	if idx := strings.Index(model, "/"); idx != -1 {
		return model[idx+1:]
	}

	// Already a short name
	return model
}

// stripBedrockVersionSuffix removes the date-version suffix from a bedrock model name.
// "claude-sonnet-4-5-20250929-v1:0" → "claude-sonnet-4-5"
func stripBedrockVersionSuffix(name string) string {
	// Pattern: model-YYYYMMDD-vN:M or model-YYYYMMDD-vN
	// Strategy: find the first segment that looks like a date (8 digits)
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) == 8 && isAllDigits(part) {
			// This is the date segment — everything before it is the model name
			return strings.Join(parts[:i], "-")
		}
	}
	// No date suffix found — return as-is
	return name
}

// normalizeModelDots converts dots to dashes in model version numbers (4.5 → 4-5).
func normalizeModelDots(name string) string {
	return strings.ReplaceAll(name, ".", "-")
}

// isAllDigits returns true if the string is non-empty and contains only ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// bedrockModelVersions maps short model names to their bedrock version identifiers.
var bedrockModelVersions = map[string]string{
	"claude-opus-4":        "anthropic.claude-opus-4-20250514-v1:0",
	"claude-sonnet-4-5":    "anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-6":    "anthropic.claude-sonnet-4-6-20250715-v1:0",
	"claude-haiku-3-5":     "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-sonnet-3-5":    "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-sonnet-3-5-v2": "anthropic.claude-3-5-sonnet-20241022-v2:0",
}

// formatBedrockModel converts a short model name to the bedrock format.
// Uses a lookup table for known models, falls back to a best-effort pattern.
func formatBedrockModel(shortName string) string {
	if bedrockID, ok := bedrockModelVersions[shortName]; ok {
		return "amazon-bedrock/" + bedrockID
	}
	// Best-effort fallback: "amazon-bedrock/anthropic.<name>"
	return "amazon-bedrock/anthropic." + shortName
}

// githubCopilotModelNames maps short model names to github-copilot formatted names.
var githubCopilotModelNames = map[string]string{
	"claude-opus-4":     "claude-opus-4",
	"claude-sonnet-4-5": "claude-sonnet-4.5",
	"claude-sonnet-4-6": "claude-sonnet-4.6",
}

// formatGithubCopilotModel converts a short model name to the github-copilot format.
func formatGithubCopilotModel(shortName string) string {
	if name, ok := githubCopilotModelNames[shortName]; ok {
		return "github-copilot/" + name
	}
	// Fallback: use as-is
	return "github-copilot/" + shortName
}
