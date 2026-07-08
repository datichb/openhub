package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAgentModel_FullCascade(t *testing.T) {
	projectOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"quality": "claude-opus-4"},
		Agents:   map[string]string{"debugger": "claude-sonnet-4-6"},
	}
	hubOverrides := &ModelOverrides{
		Default:  "claude-haiku-3-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
		Agents:   map[string]string{"reviewer": "claude-opus-4"},
	}

	tests := []struct {
		name             string
		agentID          string
		family           string
		frontmatterModel string
		provider         string
		expected         string
	}{
		{
			name:     "Level 1: project agent override wins",
			agentID:  "debugger",
			family:   "quality",
			provider: "anthropic",
			expected: "anthropic/claude-sonnet-4-6",
		},
		{
			name:     "Level 2: project family override (no agent override)",
			agentID:  "reviewer",
			family:   "quality",
			provider: "anthropic",
			expected: "anthropic/claude-opus-4",
		},
		{
			name:     "Level 3: project global (no agent/family override)",
			agentID:  "designer",
			family:   "design",
			provider: "anthropic",
			expected: "anthropic/claude-sonnet-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAgentModel(tt.agentID, tt.family, projectOverrides, hubOverrides, tt.frontmatterModel, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveAgentModel_HubLevels(t *testing.T) {
	// No project overrides — fall through to hub levels
	hubOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
		Agents:   map[string]string{"reviewer": "claude-opus-4"},
	}

	tests := []struct {
		name             string
		agentID          string
		family           string
		frontmatterModel string
		provider         string
		expected         string
	}{
		{
			name:     "Level 4: hub agent override",
			agentID:  "reviewer",
			family:   "quality",
			provider: "anthropic",
			expected: "anthropic/claude-opus-4",
		},
		{
			name:     "Level 5: hub family override",
			agentID:  "orchestrator",
			family:   "planning",
			provider: "anthropic",
			expected: "anthropic/claude-sonnet-4-6",
		},
		{
			name:     "Level 6: hub global model",
			agentID:  "designer",
			family:   "design",
			provider: "anthropic",
			expected: "anthropic/claude-sonnet-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAgentModel(tt.agentID, tt.family, nil, hubOverrides, tt.frontmatterModel, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveAgentModel_FrontmatterFloor(t *testing.T) {
	// No overrides at all — falls through to frontmatter
	result := ResolveAgentModel("orchestrator", "planning", nil, nil, "anthropic/claude-sonnet-4-6", "anthropic")
	assert.Equal(t, "anthropic/claude-sonnet-4-6", result)
}

func TestResolveAgentModel_NoModelAnywhere(t *testing.T) {
	// No model at any level
	result := ResolveAgentModel("designer", "design", nil, nil, "", "anthropic")
	assert.Equal(t, "", result)
}

func TestResolveAgentModel_BedrockProvider(t *testing.T) {
	hubOverrides := &ModelOverrides{
		Agents: map[string]string{"reviewer": "claude-opus-4"},
	}

	result := ResolveAgentModel("reviewer", "quality", nil, hubOverrides, "", "bedrock")
	assert.Equal(t, "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", result)
}

func TestResolveAgentModel_BedrockProviderWithFrontmatter(t *testing.T) {
	// Frontmatter declares "anthropic/claude-sonnet-4-6", project uses bedrock
	result := ResolveAgentModel("orchestrator", "planning", nil, nil, "anthropic/claude-sonnet-4-6", "bedrock")
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0", result)
}

func TestResolveAgentModel_EmptyProvider(t *testing.T) {
	// If provider is empty, return model as-is without normalization
	result := ResolveAgentModel("orchestrator", "planning", nil, nil, "anthropic/claude-sonnet-4-6", "")
	assert.Equal(t, "anthropic/claude-sonnet-4-6", result)
}

func TestNormalizeModelForProvider(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		provider string
		expected string
	}{
		// Anthropic provider
		{"anthropic short name", "claude-opus-4", "anthropic", "anthropic/claude-opus-4"},
		{"anthropic already prefixed", "anthropic/claude-opus-4", "anthropic", "anthropic/claude-opus-4"},
		{"anthropic from bedrock format", "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", "anthropic", "anthropic/claude-opus-4"},

		// Bedrock provider
		{"bedrock short name", "claude-opus-4", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0"},
		{"bedrock from anthropic prefixed", "anthropic/claude-opus-4", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0"},
		{"bedrock already bedrock format", "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0"},
		{"bedrock sonnet", "claude-sonnet-4-5", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"bedrock sonnet 4-6", "claude-sonnet-4-6", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0"},
		{"bedrock unknown model fallback", "claude-future-5", "bedrock", "amazon-bedrock/anthropic.claude-future-5"},

		// GitHub Copilot
		{"github-copilot sonnet", "claude-sonnet-4-5", "github-copilot", "github-copilot/claude-sonnet-4.5"},
		{"github-copilot opus", "claude-opus-4", "github-copilot", "github-copilot/claude-opus-4"},

		// OpenRouter
		{"openrouter", "claude-opus-4", "openrouter", "anthropic/claude-opus-4"},

		// Empty inputs
		{"empty model", "", "anthropic", ""},
		{"empty provider", "claude-opus-4", "", "claude-opus-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeModelForProvider(tt.model, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractShortModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4", "claude-opus-4"},
		{"anthropic/claude-opus-4", "claude-opus-4"},
		{"amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", "claude-opus-4"},
		{"amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0", "claude-sonnet-4-5"},
		{"github-copilot/claude-sonnet-4.5", "claude-sonnet-4-5"},
		{"openrouter/claude-opus-4", "claude-opus-4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractShortModelName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveAgentModel_ProjectOverrideBeatsHub(t *testing.T) {
	// Both project and hub define the same agent — project wins
	projectOverrides := &ModelOverrides{
		Agents: map[string]string{"reviewer": "claude-sonnet-4-5"},
	}
	hubOverrides := &ModelOverrides{
		Agents: map[string]string{"reviewer": "claude-opus-4"},
	}

	result := ResolveAgentModel("reviewer", "quality", projectOverrides, hubOverrides, "anthropic/claude-opus-4", "anthropic")
	assert.Equal(t, "anthropic/claude-sonnet-4-5", result)
}

func TestResolveAgentModel_FamilyOverrideBeatsGlobal(t *testing.T) {
	// Project has family override AND global — family wins for matching agent
	projectOverrides := &ModelOverrides{
		Default:  "claude-haiku-3-5",
		Families: map[string]string{"quality": "claude-opus-4"},
	}

	result := ResolveAgentModel("reviewer", "quality", projectOverrides, nil, "", "anthropic")
	assert.Equal(t, "anthropic/claude-opus-4", result)
}

func TestResolveAgentModel_NoFamilySkipsLevel(t *testing.T) {
	// Agent with no family (empty string) — family levels are skipped
	projectOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"planning": "claude-opus-4"},
	}

	result := ResolveAgentModel("standalone", "", projectOverrides, nil, "", "anthropic")
	// Falls to level 3 (project global) because no family match
	assert.Equal(t, "anthropic/claude-sonnet-4-5", result)
}
